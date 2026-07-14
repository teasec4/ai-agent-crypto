package llm

// ---- message types ----

// Message is a single turn in a conversation.
type Message struct {
	Role       string     `json:"role"`                 // "system", "user", "assistant", "tool"
	Content    string     `json:"content"`              // text content
	ToolCallID string     `json:"tool_call_id,omitempty"` // for "tool" role
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"` // for "assistant" role
}

// ---- tool calling (request side) ----

// ToolDefinition describes a tool to the LLM.
type ToolDefinition struct {
	Type     string             `json:"type"`     // "function"
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition describes a function tool.
type FunctionDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  *JSONSchema `json:"parameters"`
}

// JSONSchema describes the parameters of a function.
type JSONSchema struct {
	Type       string                `json:"type"`
	Properties map[string]Property   `json:"properties"`
	Required   []string              `json:"required,omitempty"`
}

// Property describes a single parameter.
type Property struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Items       *Property `json:"items,omitempty"` // for array type
}

// ---- tool calling (response side) ----

// ToolCall represents a tool call from the LLM.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`     // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall represents a function call inside a ToolCall.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ---- request / response ----

// Request is the chat completion request body.
type Request struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
}

// Response is the top-level API response.
type Response struct {
	Choices []Choice  `json:"choices"`
	Error   *APIError `json:"error,omitempty"`
}

// Choice represents a single completion choice.
type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"` // "stop", "tool_calls", "length"
}

// ChatResponse is the parsed result the rest of the agent uses.
type ChatResponse struct {
	Content     string     // text content (empty if tool_calls)
	ToolCalls   []ToolCall // tool calls (nil if text response)
	FinishReason string   // "stop", "tool_calls", "length"
}

// APIError represents an API error response.
type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param"`
	Code    string `json:"code"`
}
