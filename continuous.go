package main

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SummarizeFn sends text to the LLM and returns a summary.
// Stateless — does not append to conversation history.
type SummarizeFn func(text string) (string, error)

const (
	silenceThreshold   = 500.0
	minChunkDuration   = 8 * time.Second
	maxChunkDuration   = 15 * time.Second
	pollInterval       = 200 * time.Millisecond
	silenceWindow      = 8000  // 0.5s at 16kHz
	summarizeThreshold = 3000  // chars of raw text before triggering summarization
	maxSummaryChars    = 16000 // ~4K tokens budget for summary history
	maxRawChars        = 16000 // ~4K tokens budget for recent raw
	maxSummarizeRetry  = 3
)

// AudioCapture records audio continuously, transcribes in chunks,
// and accumulates transcript text for later use by the LLM.
type AudioCapture struct {
	mode       CaptureMode
	monSource  string
	whisperURL string
	renderer   Renderer
	summarize  SummarizeFn
	recorder   *Recorder
	active     atomic.Bool
	stopCh     chan struct{}

	mu           sync.Mutex
	rawChunks    []string
	rawCharCount int
	summaries    []string
	summarizing  atomic.Bool
	retryCount   int
}

func NewAudioCapture(mode CaptureMode, monSource string, whisperURL string, renderer Renderer, summarize SummarizeFn) *AudioCapture {
	return &AudioCapture{
		mode:       mode,
		monSource:  monSource,
		whisperURL: whisperURL,
		renderer:   renderer,
		summarize:  summarize,
	}
}

func (ac *AudioCapture) Active() bool {
	return ac.active.Load()
}

func (ac *AudioCapture) Toggle() {
	if ac.active.Load() {
		ac.stop()
		return
	}
	ac.start()
}

func (ac *AudioCapture) start() {
	ac.mu.Lock()
	ac.rawChunks = nil
	ac.rawCharCount = 0
	ac.summaries = nil
	ac.retryCount = 0
	ac.mu.Unlock()

	ac.recorder = NewRecorder(CaptureModeSystem, ac.monSource)
	if err := ac.recorder.Start(); err != nil {
		ac.renderer.SetStatus("audio capture error: " + err.Error())
		return
	}

	ac.stopCh = make(chan struct{})
	ac.active.Store(true)
	ac.renderer.SetStatus("audio capture ON — recording...")
}

func (ac *AudioCapture) stop() {
	ac.active.Store(false)
	close(ac.stopCh)
	ac.recorder.Stop()

	ac.mu.Lock()
	n := ac.rawCharCount
	for _, s := range ac.summaries {
		n += len(s)
	}
	ac.mu.Unlock()

	ac.renderer.SetStatus(fmt.Sprintf("audio capture OFF — %d chars accumulated", n))
}

// BuildContext assembles summaries + recent raw chunks for the LLM.
// Drains both buffers.
func (ac *AudioCapture) BuildContext() string {
	ac.TranscribeNow()

	ac.mu.Lock()
	defer ac.mu.Unlock()

	var b strings.Builder

	// Append summaries, dropping oldest if over budget
	totalSummaryChars := 0
	startIdx := 0
	for _, s := range ac.summaries {
		totalSummaryChars += len(s)
	}
	for totalSummaryChars > maxSummaryChars && startIdx < len(ac.summaries) {
		totalSummaryChars -= len(ac.summaries[startIdx])
		startIdx++
	}
	for i := startIdx; i < len(ac.summaries); i++ {
		b.WriteString(ac.summaries[i])
		b.WriteString("\n\n")
	}

	// Append recent raw chunks
	if len(ac.rawChunks) > 0 {
		if b.Len() > 0 {
			b.WriteString("---\nRecent transcript:\n")
		}
		raw := strings.Join(ac.rawChunks, " ")
		if len(raw) > maxRawChars {
			raw = raw[len(raw)-maxRawChars:]
		}
		b.WriteString(raw)
	}

	// Drain
	ac.summaries = nil
	ac.rawChunks = nil
	ac.rawCharCount = 0
	ac.retryCount = 0

	return strings.TrimSpace(b.String())
}

// DrainTranscript is a backward-compatible wrapper around BuildContext.
func (ac *AudioCapture) DrainTranscript() string {
	return ac.BuildContext()
}

// AppendTranscript adds external transcript text (e.g. from mic recording).
func (ac *AudioCapture) AppendTranscript(text string) {
	ac.mu.Lock()
	ac.rawChunks = append(ac.rawChunks, text)
	ac.rawCharCount += len(text)
	ac.mu.Unlock()

	ac.maybeStartSummarize()
}

// TranscriptLen returns the current total length of all accumulated text.
func (ac *AudioCapture) TranscriptLen() int {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	n := ac.rawCharCount
	for _, s := range ac.summaries {
		n += len(s)
	}
	return n
}

// TranscribeNow drains audio samples, transcribes, and appends to raw chunks.
func (ac *AudioCapture) TranscribeNow() {
	samples := ac.recorder.DrainSamples()
	if len(samples) == 0 {
		return
	}
	if rms(samples) < silenceThreshold {
		return
	}

	wavData := EncodeWAV(samples, asrSampleRate)
	text, err := Transcribe(wavData, ac.whisperURL)
	if err != nil {
		fmt.Printf("[audio-capture] transcribe error: %v\n", err)
		return
	}

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return
	}

	ac.mu.Lock()
	ac.rawChunks = append(ac.rawChunks, trimmed)
	ac.rawCharCount += len(trimmed)
	n := ac.rawCharCount
	for _, s := range ac.summaries {
		n += len(s)
	}
	ac.mu.Unlock()

	ac.renderer.AppendTranscriptChunk("audio", trimmed)
	ac.renderer.SetStatus(fmt.Sprintf("audio capture — %d chars accumulated", n))

	ac.maybeStartSummarize()
}

// maybeStartSummarize triggers a background summarization if threshold is met.
func (ac *AudioCapture) maybeStartSummarize() {
	if ac.summarize == nil {
		return
	}
	if ac.summarizing.Load() {
		return
	}

	ac.mu.Lock()
	needsSummarize := ac.rawCharCount >= summarizeThreshold
	ac.mu.Unlock()

	if !needsSummarize {
		return
	}

	ac.summarizing.Store(true)
	go ac.doSummarize()
}

func (ac *AudioCapture) doSummarize() {
	defer ac.summarizing.Store(false)

	ac.mu.Lock()
	if ac.rawCharCount < summarizeThreshold {
		ac.mu.Unlock()
		return
	}
	text := strings.Join(ac.rawChunks, " ")
	ac.mu.Unlock()

	summary, err := ac.summarize(text)
	if err != nil {
		fmt.Printf("[audio-capture] summarize error: %v\n", err)
		ac.mu.Lock()
		ac.retryCount++
		if ac.retryCount >= maxSummarizeRetry {
			// Drop oldest raw chunks to prevent unbounded growth
			half := len(ac.rawChunks) / 2
			dropped := 0
			for _, c := range ac.rawChunks[:half] {
				dropped += len(c)
			}
			ac.rawChunks = ac.rawChunks[half:]
			ac.rawCharCount -= dropped
			ac.retryCount = 0
		}
		ac.mu.Unlock()
		return
	}

	ac.mu.Lock()
	ac.summaries = append(ac.summaries, strings.TrimSpace(summary))
	ac.rawChunks = nil
	ac.rawCharCount = 0
	ac.retryCount = 0
	ac.mu.Unlock()

	ac.renderer.SetStatus("transcript segment summarized")
}

// RunChunkLoop polls for silence-based chunk boundaries while active.
// Must be called in a goroutine.
func (ac *AudioCapture) RunChunkLoop() {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	chunkStart := time.Now()

	for {
		select {
		case <-ac.stopCh:
			return
		case <-ticker.C:
		}

		elapsed := time.Since(chunkStart)

		// Too early — keep accumulating
		if elapsed < minChunkDuration {
			continue
		}

		// Backstop — force drain regardless of silence
		if elapsed >= maxChunkDuration {
			ac.TranscribeNow()
			chunkStart = time.Now()
			continue
		}

		// Between min and max: drain only on silence
		tailRMS := ac.recorder.PeekTailRMS(silenceWindow)
		if tailRMS >= silenceThreshold {
			continue
		}

		ac.TranscribeNow()
		chunkStart = time.Now()
	}
}

func rms(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		v := float64(s)
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(samples)))
}
