package loop

import (
	"ai-agent/internal/llm"
	"ai-agent/internal/tools/registry"
)

func RunLoop(model string, messages []llm.Message, tools registry.Registry) LoopResult{
	var trace = []LoopIteration{}

	for true{
		iterationIndex := len(trace) + 1

		// model call
		
	}
}