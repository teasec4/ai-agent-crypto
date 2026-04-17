package agent

import (
	"ai-agent/internal/config"
	"ai-agent/internal/llm"
	"ai-agent/internal/tools"
)

type Executor struct {
	tools map[string]tools.Tool
	llm   *llm.Client
	cfg   *config.Config
}

func NewExecutor(cfg *config.Config, llmClient *llm.Client) Executor {
	// Create crypto tool with config
	cryptoTool := tools.NewCryptoTool(cfg)

	return Executor{
		tools: map[string]tools.Tool{
			"get_crypto_price":     cryptoTool,
			"unknown":              &UnknownTool{},
		},
		llm: llmClient,
		cfg: cfg,
	}
}

func (e Executor) Execute(plan Plan, state *State) string {
	tool, ok := e.tools[plan.Action]
	if !ok {
		tool = e.tools["unknown"]
	}

	// Prepare parameters for the tool
	params := make(map[string]interface{})
	if plan.Parameters != nil {
		params = plan.Parameters
	}

	// Add any additional context from state
	if state.LastQuery != "" {
		params["context"] = state.LastQuery
	}

	result, err := tool.Run(params)
	if err != nil {
		return "Ошибка при выполнении действия: " + err.Error()
	}

	// Update state
	state.LastAction = plan.Action
	state.LastResult = result
	if plan.Input != "" {
		state.LastQuery = plan.Input
	}

	return result
}

// UnknownTool handles unknown actions
type UnknownTool struct{}

func (t *UnknownTool) Run(params map[string]interface{}) (string, error) {
	return "Извините, я не могу выполнить это действие. Пожалуйста, уточните ваш запрос.", nil
}
