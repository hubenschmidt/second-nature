# Sandbox Tab in Overlay

## Context
Users want to verify that LLM-generated code compiles/runs correctly without leaving the overlay. A third "Sandbox" tab receives code blocks from Chat via a button, executes them with the appropriate compiler/interpreter, and displays output.

## Files

### 1. `sandbox.go` (new) — Execution engine

**`runners` map** — keyed by fence language strings (`"python"`, `"go"`, `"javascript"`, `"js"`, `"typescript"`, `"ts"`, `"cpp"`, `"c++"`, `"rust"`, `"java"`):
- Each entry: `ext string`, optional `compileCmd func(src,bin) []string`, `runCmd func(src) []string`
- Interpreted langs (python/js/ts/go): `runCmd` only — `python3 src`, `node src`, `npx tsx src`, `go run src`
- Compiled langs: `compileCmd` first (`g++ -o bin src`, `rustc -o bin src`, `javac src`), then run binary (or `java -cp dir Main`)

**`SandboxResult` struct** — `Stdout`, `Stderr`, `ExitCode int`, `Error string`

**`RunSandbox(code, lang string) SandboxResult`**:
- Guard: unknown lang → return error result
- `os.MkdirTemp` + `defer os.RemoveAll` for isolation
- Write code to `main{ext}` in tmpdir
- If `compileCmd` != nil → run compile, return early on failure
- Run with `runWithTimeout` (10s `context.WithTimeout`)

**`runWithTimeout(args []string, dir string) SandboxResult`**:
- `exec.CommandContext` with 10s deadline
- Capture stdout/stderr to `strings.Builder`
- Handle timeout, exit errors, success

### 2. `overlay.go` — UI changes

**New fields on `OverlayRenderer`**:
```go
sandboxMu   sync.Mutex
sandboxCode string
sandboxLang string
```

**CSS additions** in `buildShell`:
```css
#sandbox-controls { display:flex; gap:8px; align-items:center; padding:4px 0; }
#sandbox-run { background:rgba(80,180,80,0.3); border:1px solid rgba(80,180,80,0.5); color:#7ec8e3; font:inherit; font-size:12px; padding:4px 12px; border-radius:3px; cursor:pointer; }
#sandbox-run:hover { background:rgba(80,180,80,0.5); }
#sandbox-lang { color:#888; font-size:11px; }
#sandbox-output { background:rgba(0,0,0,0.5); padding:10px; border-radius:4px; font-size:12px; white-space:pre-wrap; max-height:400px; overflow-y:auto; margin-top:8px; }
.sandbox-btn { position:absolute; bottom:4px; right:4px; background:rgba(80,180,80,0.2); border:1px solid rgba(80,180,80,0.4); color:#7ec8e3; font-size:11px; padding:1px 8px; border-radius:3px; cursor:pointer; }
.sandbox-btn:hover { background:rgba(80,180,80,0.4); color:#fff; }
.sandbox-ok { color:#50b050; } .sandbox-fail { color:#e05050; }
```

**HTML** — third tab button + `#sandbox-content` div:
```html
<button id="tab-sandbox" onclick="switchTab('sandbox')">Sandbox</button>
...
<div id="sandbox-content" class="tab-content">
  <div id="sandbox-code"></div>
  <div id="sandbox-controls">
    <button id="sandbox-run" onclick="_runSandbox()">&#9654; Run</button>
    <span id="sandbox-lang"></span>
  </div>
  <pre id="sandbox-output"></pre>
</div>
```

**JS changes** in `<script>`:
- Replace hardcoded 2-tab `switchTab` with data-driven 3-tab version (iterate `['chat','transcript','sandbox']`)
- Add `_injectSandboxButtons()` — queries `.response-block pre code[class*="language-"]`, appends a `▶ Sandbox` button to each `<pre>`. Button onclick: extracts `code.textContent` + lang from class, calls `_sendToSandbox(text, lang)`

**Go bindings** in `NewOverlayRenderer()`:
- `_sendToSandbox(code, lang string)` — stores code/lang on `OverlayRenderer` fields, populates sandbox tab HTML, switches to sandbox tab, **auto-runs immediately** (shows "running...", launches goroutine)
- `_runSandbox()` — reads stored code/lang, shows "running..." in output, launches goroutine: `RunSandbox(code, lang)` → `o.renderSandboxResult(result)`. Used by both auto-run and the manual `▶ Run` button (for re-runs)

**Append `_injectSandboxButtons();`** to JS in `StreamDone()` and `AppendStreamDone()`

**New method `renderSandboxResult(r SandboxResult)`** — builds HTML with stdout, stderr, error, exit code (color-coded), sets `#sandbox-output` innerHTML via `eval()`

### 3. No changes to `main.go`, `renderer.go`, `hotkey.go`
Language comes from goldmark's `class="language-xxx"` on `<code>` elements — no need to thread `lang` through. Sandbox is overlay-only, no new Renderer interface methods.

## Flow
1. LLM response renders in Chat with code blocks
2. `_injectSandboxButtons()` adds `▶ Sandbox` button to each `<pre>` with a recognized language
3. User clicks button → `_sendToSandbox(code, lang)` stores code, populates sandbox tab, switches to it
4. Code displayed in `#sandbox-code`, `▶ Run` button + lang label shown, execution starts automatically
5. Goroutine executes `RunSandbox` with 10s timeout (user can re-run via `▶ Run` button)
6. Result rendered in `#sandbox-output` with color-coded stdout/stderr/exit code

## Verification
1. `PKG_CONFIG_PATH=./pkgconfig:$PKG_CONFIG_PATH go build`
2. Run with overlay → three tabs visible (Chat, Transcript, Sandbox)
3. Trigger a code response → code block shows `▶ Sandbox` button
4. Click button → Sandbox tab opens with code displayed
5. Click Run → output appears with exit code
6. Test timeout: code with `time.sleep(30)` → "timeout after 10s"
