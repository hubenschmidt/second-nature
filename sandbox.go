package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type SandboxResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Error    string
}

type runner struct {
	ext        string
	compileCmd func(src, bin string) []string
	runCmd     func(src string) []string
}

var runners = map[string]runner{
	"python": {ext: ".py", runCmd: func(src string) []string { return []string{"python3", src} }},
	"go":     {ext: ".go", runCmd: func(src string) []string { return []string{"go", "run", src} }},
	"javascript": {ext: ".js", runCmd: func(src string) []string { return []string{"node", src} }},
	"js":         {ext: ".js", runCmd: func(src string) []string { return []string{"node", src} }},
	"typescript": {ext: ".ts", runCmd: func(src string) []string { return []string{"npx", "tsx", src} }},
	"ts":         {ext: ".ts", runCmd: func(src string) []string { return []string{"npx", "tsx", src} }},
	"cpp": {
		ext:        ".cpp",
		compileCmd: func(src, bin string) []string { return []string{"g++", "-o", bin, src} },
		runCmd:     func(src string) []string { return []string{src} },
	},
	"c++": {
		ext:        ".cpp",
		compileCmd: func(src, bin string) []string { return []string{"g++", "-o", bin, src} },
		runCmd:     func(src string) []string { return []string{src} },
	},
	"rust": {
		ext:        ".rs",
		compileCmd: func(src, bin string) []string { return []string{"rustc", "-o", bin, src} },
		runCmd:     func(src string) []string { return []string{src} },
	},
	"java": {
		ext:        ".java",
		compileCmd: func(src, _ string) []string { return []string{"javac", src} },
		runCmd: func(src string) []string {
			return []string{"java", "-cp", filepath.Dir(src), "Main"}
		},
	},
}

func RunSandbox(code, lang string) (result SandboxResult) {
	defer func() {
		if rv := recover(); rv != nil {
			AppLog.Error("sandbox: panic: %v", rv)
			result = SandboxResult{Error: fmt.Sprintf("panic: %v", rv), ExitCode: 1}
		}
	}()

	AppLog.Info("sandbox: lang=%s code=%d bytes", lang, len(code))

	r, ok := runners[strings.ToLower(lang)]
	if !ok {
		AppLog.Error("sandbox: unsupported language: %s", lang)
		return SandboxResult{Error: fmt.Sprintf("unsupported language: %s", lang), ExitCode: 1}
	}

	tmpdir, err := os.MkdirTemp("", "sandbox-*")
	if err != nil {
		AppLog.Error("sandbox: tmpdir: %v", err)
		return SandboxResult{Error: fmt.Sprintf("tmpdir: %v", err), ExitCode: 1}
	}
	defer os.RemoveAll(tmpdir)

	filename := "main" + r.ext
	if lang == "java" {
		filename = "Main.java"
	}
	src := filepath.Join(tmpdir, filename)
	if err := os.WriteFile(src, []byte(code), 0600); err != nil {
		AppLog.Error("sandbox: write: %v", err)
		return SandboxResult{Error: fmt.Sprintf("write: %v", err), ExitCode: 1}
	}

	bin := filepath.Join(tmpdir, "main")

	if r.compileCmd != nil {
		args := r.compileCmd(src, bin)
		AppLog.Info("sandbox: compile %v", args)
		res := runWithTimeout(args, tmpdir)
		if res.ExitCode != 0 {
			AppLog.Warn("sandbox: compile failed exit=%d stderr=%s", res.ExitCode, res.Stderr)
			return res
		}
	}

	runTarget := src
	if r.compileCmd != nil && lang != "java" {
		runTarget = bin
	}
	args := r.runCmd(runTarget)
	AppLog.Info("sandbox: run %v", args)
	res := runWithTimeout(args, tmpdir)
	AppLog.Info("sandbox: exit=%d stdout=%d stderr=%d", res.ExitCode, len(res.Stdout), len(res.Stderr))
	if res.Error != "" {
		AppLog.Error("sandbox: %s", res.Error)
	}
	return res
}

const sandboxTimeout = 10 * time.Second

func runWithTimeout(args []string, dir string) SandboxResult {
	ctx, cancel := context.WithTimeout(context.Background(), sandboxTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return SandboxResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: 124,
			Error:    fmt.Sprintf("timeout after %s", sandboxTimeout),
		}
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return SandboxResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: exitErr.ExitCode(),
		}
	}

	if err != nil {
		return SandboxResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: 1,
			Error:    err.Error(),
		}
	}

	return SandboxResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}
}
