package main

const solvePrompt = "Look at this screen capture. If there's a code problem, provide three solutions:\n\nStart with the **Goal** (one-line summary), then a **Problem Description** paragraph explaining the problem in plain language — what the input looks like, what the expected output is, and any key constraints or gotchas. Then a **Solution Description** paragraph summarizing at a high level how the problem is solved (the core idea/technique) before diving into the solutions.\n\n1. **Naive Solution** — start with a plain-English **Solution Description** explaining the high-level approach, then pseudocode, then full code, then explain how it works, time/space complexity, and edge cases.\n2. **Optimized Solution** — start with a plain-English **Solution Description** explaining the high-level approach and how it differs from the naive, then pseudocode, then full code, then explain how it works, time/space complexity, edge cases, and why it's better than the naive approach.\n3. **Recursive Solution** (if applicable) — start with a plain-English **Solution Description** explaining the recursive approach, then pseudocode, then full code, then explain how it works, time/space complexity, and edge cases. Skip this section if recursion doesn't naturally fit the problem.\n\nIf it's a continuation of a previous problem, build on your prior answer."

type Provider interface {
	Solve(pngData []byte, onDelta func(string)) (string, error)
	ModelName() string
}
