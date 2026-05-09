package chat

import (
	"ai-agent/internal/llm"
	"ai-agent/internal/memory"
)

type Chat struct{
	LlmClient llm.Client
	WorkingMemory memory.WorkMemory
}

func NewChat(apiKey, baseUrl, model string)*Chat{
	return  &Chat{
		LlmClient: *llm.NewClient(
			apiKey, baseUrl, model,
		),
		WorkingMemory: *memory.NewWorkMemory(),
	}
}