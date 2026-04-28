package tools

// Tool is the interface that all tools must implement.
type Tool interface {
	// Name returns the unique name of the tool (e.g. "get_crypto_price").
	Name() string

	// Description returns a human-readable description of what the tool does.
	// This is used by the planner to describe available actions.
	Description() string

	// Run executes the tool with the given parameters and returns the result.
	Run(params map[string]interface{}) (string, error)
}
