package memory

import "ai-agent/internal/llm"

type LongTermMemory struct {
	store          Store
	contextBuilder *ContextBuilder
}

func NewLongTermMemory(store Store) *LongTermMemory {
	if store == nil {
		return nil
	}
	return &LongTermMemory{
		store:          store,
		contextBuilder: NewContextBuilder(store),
	}
}

func (m *LongTermMemory) Append(event MemoryEvent) error {
	if m == nil || m.store == nil {
		return nil
	}
	return m.store.Append(event)
}

func (m *LongTermMemory) BuildContext(req ContextRequest) ([]llm.Message, error) {
	if m == nil || m.contextBuilder == nil {
		return nil, nil
	}
	return m.contextBuilder.Build(req)
}
