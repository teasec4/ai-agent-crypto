package tools

type Tool interface {
	Run(params map[string]interface{}) (string, error)
}
