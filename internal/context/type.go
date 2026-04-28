package context

// Provider defines the interface for retrieving workspace context.
type Provider interface {
	GetContext(cmd string) (RepoContext, error)
}

// WorkspaceContext holds contextual information about the workspace.
type WorkspaceContext struct {
	Cwd           string
	RepoRoot      string
	Branch        string
	DefaultBranch string
	Status        string
	RecentCommits []string
	ProjectDocs   map[string]string
}

// RepoContext holds information about a git repository at a given point.
type RepoContext struct {
	Cmd           string
	RepoRoot      string
	Branch        string
	DefaultBranch string
	IsDirty       bool
	Status        string
	Commits       []Commit
	Docs          map[string]string
}

// Commit represents a single git commit.
type Commit struct {
	Hash    string
	Message string
}
