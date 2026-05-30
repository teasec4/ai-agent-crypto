package guardrails

import (
	"fmt"

	"ai-agent/internal/llm"
)

type GuardrailInput struct {
	Iteration int
	Messages  []llm.Message
}

type GuardrailFn func(input GuardrailInput) error

func MaxIterations(limit int) GuardrailFn {
	return func(input GuardrailInput) error {
		if input.Iteration >= limit {
			return fmt.Errorf("reached iteration limit (%d)", limit)
		}

		return nil
	}
}

func MaxMessages(limit int) GuardrailFn {
	return func(input GuardrailInput) error {
		if len(input.Messages) > limit {
			return fmt.Errorf("reached message limit (%d)", limit)
		}

		return nil
	}
}

func CombineGuardrails(fns ...GuardrailFn) GuardrailFn {
	return func(input GuardrailInput) error {
		for _, fn := range fns {
			if err := fn(input); err != nil {
				return err
			}
		}
		return nil
	}
}
