package provider

import (
	"fmt"
	"strings"
)

const CodeRules = `**Code rules (always follow):**
- KISS — keep solutions simple and straightforward.
- YAGNI — only implement what is needed, no speculative features.
- Use guard clauses and early returns instead of if/else chains.
- Use object/map lookups instead of switch-case or nested conditionals.
- Flatten all conditional logic — no nested if, else, or switch. No continue or break.
- Prefer small, focused functions over large functions with many branches.
- Group imports: stdlib, external, internal (with blank lines between groups).
- If JavaScript/TypeScript: use modern ES6+ syntax (arrow functions, const/let, template literals, destructuring, for...of).
- Be concise — avoid filler and unnecessary elaboration.`

func BuildContextReceipt(imageCount int, hasContext bool, hasTranscript bool) string {
	parts := []string{}
	if imageCount > 0 {
		parts = append(parts, fmt.Sprintf("%d screenshot(s)", imageCount))
	}
	if hasContext {
		parts = append(parts, "source files")
	}
	if hasTranscript {
		parts = append(parts, "audio transcript")
	}
	if len(parts) == 0 {
		return ""
	}
	return "**You have been provided the following context: " + strings.Join(parts, ", ") +
		".** Begin your response with a `> Context:` line confirming each piece of context you received (e.g. number of screenshots, source file names, transcript presence). Then proceed with your answer.\n\n"
}

func BuildSolvePrompt(lang, contextText, transcript string, imageCount int) string {
	receipt := BuildContextReceipt(imageCount, contextText != "", transcript != "")
	prefix := receipt
	if contextText != "" {
		prefix += "The following source files are provided as additional context:\n\n" + contextText + "\n\n"
	}
	if transcript != "" {
		prefix += "The following is the user's audio transcript providing additional instructions or context:\n\n" + transcript + "\n\n"
	}
	return prefix + CodeRules + "\n\nLook at this screen capture. If there's a code problem, provide two solutions in **" + lang + "**:\n\nStart with the **Goal** (one-line summary), then a **Problem Description** paragraph explaining the problem in plain language — what the input looks like, what the expected output is, and any key constraints or gotchas. Follow the Problem Description with a **TLDR** — a single sentence that captures the essence of the problem in the simplest possible terms. Then a **Solution Description** paragraph summarizing at a high level how the problem is solved (the core idea/technique). Follow the Solution Description with a **TLDR** — a single sentence that captures the core approach in the simplest possible terms.\n\n1. **Naive Solution** — start with a plain-English **Solution Description** explaining the high-level approach, then pseudocode, then full " + lang + " code, then explain how it works, time/space complexity, and edge cases.\n2. **Optimized Solution** — start with a plain-English **Solution Description** explaining the high-level approach and how it differs from the naive, then pseudocode, then full " + lang + " code, then explain how it works, time/space complexity, edge cases, and why it's better than the naive approach.\n\n**Regex rule:** If any solution uses regex, treat that solution as the naive one. Always provide an additional solution using string manipulation, parsing, or other non-regex techniques. Regex is always considered the naive approach.\n\n**JavaScript style rule:** If the language is JavaScript (ECMAScript 6) or later, you MUST use modern ES6+ syntax throughout: arrow functions (`=>`), `const`/`let` (never `var`), template literals, destructuring, spread/rest operators, `Map`/`Set` where appropriate, and `for...of` instead of `for(let i=0;...)` when iterating. Never use `function` keyword for callbacks or inline functions.\n\n**Control flow rule:** Use guard clauses and early returns instead of if/else chains. Use object/map lookups instead of switch-case or nested conditionals. Flatten all conditional logic — no nested `if`, `else`, or `switch`.\n\nAll code must be written in " + lang + ". Be concise — avoid filler and unnecessary elaboration. If it's a continuation of a previous problem, build on your prior answer."
}
