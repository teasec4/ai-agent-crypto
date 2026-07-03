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
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("error loading config", "error", err)
		os.Exit(1)
	}
	h := harness.New(cfg)
	session := h.NewAgentSession()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("AI Agent ready. Commands: /reset, /approve, /dismiss, Ctrl+C to exit.")
	fmt.Println()

	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "/reset" {
			session.Reset()
			fmt.Println("Agent: контекст сброшен.")
			fmt.Println()
			continue
		}

		if pa := session.PendingAction(); pa != nil {
			switch input {
			case "/approve":
				slog.Info("user approved", "tool", pa.Tool, "id", pa.ID)
				result := session.Approve()
				printResult(result)
				continue
			case "/dismiss":
				slog.Info("user dismissed", "tool", pa.Tool, "id", pa.ID)
				result := session.Reject()
				printResult(result)
				continue
			default:
				fmt.Printf("Ожидается /approve или /dismiss для действия %q.\n", pa.Tool)
				fmt.Printf("  %s\n\n", pa.Summary)
				continue
			}
		}

		slog.Info("user input", "input", input)
		result := session.Run(input)

		for _, iter := range result.LoopResult.Trace {
			if iter.Outcome == "tool_calls" && len(iter.ToolEvents) > 0 {
				for _, ev := range iter.ToolEvents {
					if ev.Error != "" {
						slog.Warn("tool error", "tool", ev.Tool, "error", ev.Error)
					}
				}
			}
		}

		printResult(result)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
}

func printResult(result harness.HarnessExecutionResult) {
	if pa := result.LoopResult.PendingAction; pa != nil {
		printApprovalCard(pa)
		return
	}
	fmt.Printf("Agent: %s\n\n", result.LoopResult.Answer)
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
	fmt.Println("│  Type  /approve  to confirm")
	fmt.Println("│  Type  /dismiss  to reject")
	fmt.Println("└─────────────────────────────────────────────┘")
	fmt.Println()
}
