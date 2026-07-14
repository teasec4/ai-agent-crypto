package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"ai-agent/internal/approval"
	"ai-agent/internal/config"
	"ai-agent/internal/harness"
	"ai-agent/internal/loop"
	"ai-agent/internal/memory"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	h := harness.New(cfg)
	mem := memory.NewDefaultWorkMemory()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("AI Agent ready. Commands: /reset, Ctrl+C to exit.")
	fmt.Println()

	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "/reset" {
			mem.Reset()
			fmt.Println("Agent: контекст сброшен.")
			fmt.Println()
			continue
		}

		slog.Info("user input", "input", input)

		result := h.RunWithMemoryStreaming(
			input,
			mem,
			"",
			func(event loop.SSEEvent) {
				switch event.Type {
				case loop.EventThinking:
					fmt.Print(".")
				case loop.EventToolStart:
					fmt.Printf("\n  → %s...", event.Tool)
				case loop.EventToolDone:
					fmt.Print(" ✓")
				case loop.EventToolError:
					fmt.Printf(" ✗ (%s)", event.Error)
				}
			},
			func(action *approval.PendingAction) bool {
				printApprovalCard(action)
				fmt.Print("  > ")
				if !scanner.Scan() {
					return false
				}
				answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
				return answer == "y" || answer == "yes" || answer == "да" || answer == "/approve"
			},
		)

		fmt.Println()
		fmt.Printf("Agent: %s\n\n", result.LoopResult.Answer)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	return nil
}

func printApprovalCard(pa *approval.PendingAction) {
	fmt.Println()
	fmt.Println("┌─────────────────────────────────────────────┐")
	fmt.Printf("│  ⚡  NEEDS APPROVAL\n")
	fmt.Println("├─────────────────────────────────────────────┤")
	fmt.Printf("│  Tool:    %s\n", pa.Tool)
	fmt.Printf("│  Risk:    %s\n", pa.Risk)
	fmt.Printf("│  Summary: %s\n", pa.Summary)
	if pa.Preview != "" {
		previewLines := strings.Split(pa.Preview, "\n")
		for _, line := range previewLines {
			fmt.Printf("│  %s\n", line)
		}
	}
	fmt.Println("├─────────────────────────────────────────────┤")
	fmt.Println("│  Type  y/yes/да  to confirm")
	fmt.Println("│  Type  anything else to reject")
	fmt.Println("└─────────────────────────────────────────────┘")
}
