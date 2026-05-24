package llm

type Model struct{
	BaseURL string
	ApiKey string
	Model string
}

func NewModel(baseURL, apiKey, model string) *Model{
	return &Model{
		BaseURL: baseURL,
		ApiKey: apiKey,
		Model: model,
	}
}