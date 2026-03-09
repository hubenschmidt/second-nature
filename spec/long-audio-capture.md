# Long-Running Audio Capture — Feature Specification

## 1. Current State

`AudioCapture` in `continuous.go` runs a fixed 10-second ticker. Each tick drains all accumulated PCM samples from `Recorder`, encodes to WAV, sends to whisper-server, and appends the resulting text to a single `strings.Builder`. When "process" is pressed, `handleAudioSend` in `main.go` calls `TranscribeNow()` one final time, drains the full transcript via `DrainTranscript()`, and sends it verbatim to `provider.FollowUp()`.

### Pain Points

- **Unbounded transcript growth.** A 60-minute meeting at ~150 WPM produces ~9,000 words (~12K tokens). The full raw transcript is sent in one shot — no summarization, no compression.
- **Hard 10-second chunk boundaries.** `time.NewTicker(chunkInterval)` fires at exactly 10s regardless of speech state. Words and sentences get split mid-utterance, producing Whisper hallucinations and garbled fragments at boundaries.
- **No intermediate analysis.** User cannot get progressive insight without pressing "process" and draining everything.
- **No Provider access on AudioCapture.** The struct holds `whisperURL` and `renderer` but has no way to call the LLM for summarization.

### Current Constants

| Constant | Value | Location |
|----------|-------|----------|
| `chunkInterval` | 10s | `continuous.go` |
| `silenceThreshold` | 500.0 RMS | `continuous.go` |
| `asrSampleRate` | 16000 Hz | `asr.go` |
| `asrChannels` | 1 (mono) | `asr.go` |
| `asrFramesPerBuf` | 1024 | `asr.go` |

### Current Buffer Sizes (unbounded)

| Buffer | Type | Drain Trigger |
|--------|------|---------------|
| `Recorder.samples` | `[]int16` | `DrainSamples()` every 10s |
| `AudioCapture.transcript` | `strings.Builder` | `DrainTranscript()` on "process" |

## 2. Target Architecture

### 2.1 Rolling Summary

Replace the single `strings.Builder` transcript with a two-tier buffer:

- **`rawChunks []string`** — recent transcription chunks not yet summarized.
- **`summaries []string`** — ordered list of compressed segment summaries produced by the LLM.

Every time `rawChunks` accumulates ≥ `summarizeThreshold` characters, a background goroutine sends the concatenated raw text to the LLM with a summarization prompt. On success, the result is appended to `summaries` and `rawChunks` is cleared.

When "process" is pressed, `BuildContext()` assembles: `[all summaries] + [recent raw chunks]` — bounded context regardless of meeting length.

#### New Type

```go
// SummarizeFn sends text to the LLM and returns a summary.
// Stateless — does not append to conversation history.
type SummarizeFn func(text string) (string, error)
```

#### New Fields on `AudioCapture`

```go
summarize    SummarizeFn
summaries    []string
rawChunks    []string
rawCharCount int
```

The existing `transcript strings.Builder` field is removed.

#### Constants

```go
const (
    summarizeThreshold = 3000  // chars of raw text before triggering summarization
    maxSummaryChars    = 16000 // ~4K tokens budget for summary history
    maxRawChars        = 16000 // ~4K tokens budget for recent raw transcript
    charsPerToken      = 4     // rough English estimate
)
```

#### Constructor Change

```go
func NewAudioCapture(mode CaptureMode, monSource string, whisperURL string,
    renderer Renderer, summarize SummarizeFn) *AudioCapture
```

Wired in `main.go`:

```go
summarize := func(text string) (string, error) {
    return provider.Summarize(summarizePrompt + text)
}
ac := NewAudioCapture(captureMode, monSource, whisperURL, renderer, summarize)
```

#### Summarization Prompt

```
Summarize this meeting transcript segment into concise bullet points.
Preserve: key decisions, action items, names, technical terms, questions raised.
Omit: filler words, repetition, small talk.
Keep the summary under 500 words.

Transcript segment:
```

#### Provider Interface Addition

```go
type Provider interface {
    // ... existing methods ...
    Summarize(text string) (string, error) // stateless, no history append
}
```

### 2.2 Silence-Based Smart Chunk Boundaries

Replace the fixed 10s ticker with a 200ms polling loop that uses a flexible drain window.

#### Strategy

- **Minimum chunk:** 8 seconds — never drain before this.
- **Maximum chunk (backstop):** 15 seconds — always drain by this point.
- **Between 8-15s:** drain when the tail 0.5s of audio has RMS below the silence threshold.

This naturally breaks on sentence boundaries, pauses, and speaker turns. Whisper handles 8-15s segments well (trained on 30s segments).

#### New Constants

```go
const (
    minChunkDuration = 8 * time.Second
    maxChunkDuration = 15 * time.Second
    pollInterval     = 200 * time.Millisecond
    silenceWindow    = 8000 // 0.5s at 16kHz
)
```

#### New Recorder Methods (`asr.go`)

```go
// PeekTailRMS returns the RMS of the last n samples without draining.
func (r *Recorder) PeekTailRMS(n int) float64

// SampleCount returns current buffer length.
func (r *Recorder) SampleCount() int
```

#### Revised RunChunkLoop (`continuous.go`)

```
poll every 200ms:
  elapsed < 8s  → skip
  elapsed >= 15s → force drain (TranscribeNow), reset timer
  8-15s + tail RMS < threshold → drain on silence, reset timer
```

### 2.3 Context Assembly on "Process"

New method replaces `DrainTranscript()`:

```go
func (ac *AudioCapture) BuildContext() string
```

1. Call `TranscribeNow()` to flush remaining audio.
2. Lock mutex.
3. Build: `[summaries joined by "\n\n"] + "\n\n---\nRecent transcript:\n" + [rawChunks joined by " "]`.
4. If total exceeds budget, drop oldest summaries first.
5. Reset summaries and rawChunks.
6. Unlock, return.

## 3. Token Budget Math

For a 60-minute meeting at 150 WPM:

| Metric | Value |
|--------|-------|
| Total words | ~9,000 |
| Total chars | ~36,000 |
| Total tokens (raw) | ~9,000 |
| Summarization compression | ~5:1 |
| Summary tokens (60 min) | ~1,800 |
| Recent raw tokens (last ~5 min unsummarized) | ~1,000 |
| `audioSendPrefix` prompt | ~100 tokens |
| **Total input on "process"** | **~3,000 tokens** |

Leaves ample room for response (4,096 tokens) within any model's context window.

## 4. Data Flow

```
parec (PulseAudio)
  │
  ▼
Recorder.samples []int16  (continuous append)
  │
  │  every 200ms poll:
  │  ├── elapsed < 8s → skip
  │  ├── elapsed ≥ 15s → force drain
  │  └── 8-15s + tail silent → drain
  │
  ▼
DrainSamples() → EncodeWAV() → Transcribe(whisper-server)
  │
  ▼
rawChunks []string  (append chunk)
  │
  │  rawCharCount ≥ 3000?
  │  ├── no  → continue
  │  └── yes → goroutine: Provider.Summarize(concat(rawChunks))
  │              │
  │              ▼
  │         summaries []string  (append, clear rawChunks)
  │
  ▼
User presses "process" (→↓)
  │
  ▼
BuildContext():
  ┌────────────────────────────────────────┐
  │  [summary 1]                           │
  │  [summary 2]                           │  ≤ 4K tokens
  │  ...                                   │
  │  ---                                   │
  │  Recent transcript:                    │
  │  [rawChunk 1] [rawChunk 2] ...         │  ≤ 4K tokens
  └────────────────────────────────────────┘
  │
  ▼
Provider.FollowUp(audioSendPrefix + context)
  │
  ▼
Renderer.AppendStream{Start,Delta,Done}
```

## 5. Files Affected

| File | Change |
|------|--------|
| `continuous.go` | Replace `transcript strings.Builder` with `rawChunks`/`summaries`. Add `SummarizeFn` field. New constructor signature. Replace ticker-based `RunChunkLoop` with polling loop. Add `BuildContext()`. Add `triggerSummarize()`. |
| `asr.go` | Add `PeekTailRMS(n int) float64` and `SampleCount() int` to `Recorder`. |
| `provider.go` | Add `Summarize(text string) (string, error)` to `Provider` interface. |
| `solve.go` | Implement `Summarize` on `AnthropicProvider` — stateless one-shot call, no history append. |
| `solve_openai.go` | Implement `Summarize` on `OpenAIProvider` — same pattern. |
| `main.go` | Wire `SummarizeFn` into `NewAudioCapture`. Update `handleAudioSend` to use `BuildContext()`. Add `summarizePrompt` const. |

## 6. Edge Cases

| Scenario | Behavior |
|----------|----------|
| **No speech for extended period** | `TranscribeNow` checks `rms < silenceThreshold` and returns early. No empty chunks appended. No wasted Whisper or LLM calls. |
| **Very fast speech (no silence gaps)** | 15s backstop forces drain. Whisper handles 15s segments well. Worst case: one mid-sentence split per 15s. |
| **LLM summarization error** | Log error, keep `rawChunks` intact, retry on next threshold crossing. After 3 failures for the same segment, drop oldest raw chunks to prevent unbounded growth. |
| **"Process" pressed during summarization** | `BuildContext()` acquires mutex. Uses whatever summaries exist plus all current `rawChunks`. In-flight summary result is discarded (its source text was already drained). |
| **Very short recording (<10s)** | `BuildContext()` returns whatever raw chunks exist. No summarization triggered. |
| **Meeting exceeds summary budget** | `BuildContext()` drops oldest summaries until total fits within `maxSummaryChars`. Beginning of very long meetings may be lost; recent context is always preserved. |
| **Whisper hallucinations on silence** | Existing `rms < silenceThreshold` guard drops silent chunks pre-Whisper. Silence-based drain reduces near-silent tails. |

## 7. Implementation Sequence

1. Add `PeekTailRMS` and `SampleCount` to `Recorder` (`asr.go`)
2. Add `Summarize` to `Provider` interface and implement (`provider.go`, `solve.go`, `solve_openai.go`)
3. Refactor `AudioCapture` struct (`continuous.go`) — new fields, new constructor
4. Implement silence-based `RunChunkLoop` (`continuous.go`)
5. Implement `triggerSummarize` and `BuildContext` (`continuous.go`)
6. Wire up in `main.go` — new constructor, `handleAudioSend` uses `BuildContext()`
7. Test with a 5-minute recording with verbose logging to verify chunk boundaries, summarization triggers, and context assembly
