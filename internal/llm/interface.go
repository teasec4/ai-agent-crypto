package llm

type LlmClient interface{
	Chat(prompt string) (string, error)
}