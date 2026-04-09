package tools

type Tool interface {
    Run() (string, error)
}