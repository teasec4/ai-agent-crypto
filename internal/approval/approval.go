package approval

import "time"

type RiskLevel string

const (
	RiskRead  RiskLevel = "read"
	RiskWrite RiskLevel = "write"
	RiskExec  RiskLevel = "exec"
)

type PendingAction struct {
	ID        string         `json:"id"`
	Tool      string         `json:"tool"`
	Risk      RiskLevel      `json:"risk"`
	Summary   string         `json:"summary"`
	Preview   string         `json:"preview"`
	Args      map[string]any `json:"args"`
	CreatedAt time.Time      `json:"createdAt"`
}
