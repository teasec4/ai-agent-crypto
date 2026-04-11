package tools

import "fmt"

type BTCTool struct{}

func NewBTCTool() Tool {
	return &BTCTool{}
}

func (t *BTCTool) Run() (string, error) {
	// Для теста просто возвращаем фиктивную цену
	price := 45000.50
	return fmt.Sprintf("BTC price: $%.2f", price), nil
}
