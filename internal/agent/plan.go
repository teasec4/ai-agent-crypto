package agent

type Plan struct {
	Action     string
	Parameters map[string]interface{}
	Input      string
}
