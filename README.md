# AI Agent

[![Go Version](https://img.shields.io/badge/Go-1.26-blue?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Dependencies](https://img.shields.io/badge/deps-godotenv%20v1.5.1-lightgrey)](https://github.com/joho/godotenv)

A modular Go agent loop with read/write workspace tools, approval gates, session persistence, and an HTTP API. Designed as a playground for experimenting with LLM agents — keep the core loop simple and the architecture explicit.

```text
cmd → harness → loop → planner → executor → tools
                      |
                      → memory + guardrails + logging
```

---

## Quick Start

```bash
# CLI mode (interactive chat)
go run ./cmd/cli

# HTTP API (for frontend integration)
OPENAI_API_KEY=sk-... go run ./cmd/api       # starts on :8080
```

Development checks:

```bash
go test ./...
go vet ./...
```

---

## Architecture

```text
cmd/
├── cli/main.go          # Interactive CLI with /approve /dismiss /reset
└── api/main.go          # HTTP server on :8080

internal/
├── harness/harness.go   # Wiring: wires planner, executor, tools, guardrails
├── loop/
│   ├── loop.go          # Main loop: guardrail → plan → act/approve → observe
│   └── type.go          # LoopRequest, LoopResult, trace, PendingAction
├── guardrails/
│   └── guardrails.go    # Max iterations (5), max messages (50)
├── planner/
│   ├── llm_planner.go   # Calls LLM, expects JSON plan, falls back to plain text
│   └── type.go          # PlanResult (action + params + reply)
├── executor/
│   └── executor.go      # Resolves action to tool, handles approval gating
├── approval/
│   └── approval.go      # PendingAction, RiskLevel (read/write/exec)
├── tools/
│   ├── interface.go     # Tool + ApprovalAwareTool + WorkspaceTool interfaces
│   ├── crypto.go        # CoinGecko price fetcher
│   ├── git.go           # Read-only git: branch, status, diff, log
│   ├── workspace.go     # Read-only: list_directory, read_file, find_files, search_text
│   ├── workspace_write.go  # Write with approval: create_directory, write_file, edit_file
│   ├── shell.go         # Command with approval: go, git, ls, pwd (allowlisted)
│   └── registry/
│       └── registry.go  # Tool discovery for planner + executor
├── memory/
│   └── work_memory.go   # Message history + LLM compaction
├── session/
│   ├── store.go         # Session store with persistence callbacks
│   ├── storage.go       # Storage interface (swappable)
│   └── json_storage.go  # JSON file persistence with atomic writes
├── llm/
│   ├── client.go        # OpenAI-compatible HTTP client
│   └── type.go          # Message, Request, Response DTOs
├── handler/
│   ├── handler.go       # HTTP routes: /ask, /sessions, /approvals, /workspace
│   └── type.go          # Request/response types
└── config/
    └── config.go        # Env config loader
```

---

## Loop Flow

```
1. Guardrail check
2. LLM planner returns JSON plan:
   ├── action=message    → answer and finish
   ├── action=unknown    → unsupported reply
   └── action=<tool>     →
         ├── approval needed?  → stop, return pendingAction
         └── no approval → execute tool, observe result, repeat
```

Every iteration is logged via `slog` (timings, decisions, errors).

Tool results are stored as `user`-role messages with a `Tool observation:` prefix to stay compatible with OpenAI-compatible APIs.

A plain text model response after a tool observation is accepted as a direct answer — a defensive fallback.

`Harness` is created once. `AgentSession` is created once per conversation. Each call to `session.Run(input)` processes one user turn.

---

## Tools (10 registered)

| Tool | Action | Risk |
|------|--------|------|
| Crypto | `get_crypto_price` | **network** |
| Git | `git_context` | read |
| List Dir | `list_directory` | read |
| Read File | `read_file` | read |
| Find Files | `find_files` | read |
| Search Text | `search_text` | read |
| Create Dir | `create_directory` | write |
| Write File | `write_file` | write |
| Edit File | `edit_file` | write |
| Run Command | `run_command` | exec |

Read-only tools run automatically. Write and exec tools **require user approval** — the loop stops and returns a `pendingAction`. The frontend (or CLI via `/approve`) confirms execution.

`unknown` is a planner action, not a tool. It signals that no available tool or direct answer fits the request.

---

## Approval Flow

```
User: "Create docs/README.md"
Agent: returns stoppedBy=approval_required with pendingAction
UI:   shows card: tool=write_file, preview=<content>, risk=write
User: clicks Approve → POST /sessions/{id}/approvals/{approvalId}
Agent: executes tool → continues loop → returns final answer
```

In the CLI:

```
┌─────────────────────────────────────────────┐
│  ⚡  NEEDS APPROVAL                         │
├─────────────────────────────────────────────┤
│  Tool:    write_file                        │
│  Risk:    write                             │
│  Summary: Create file docs/README.md        │
│  --- preview ---                            │
├─────────────────────────────────────────────┤
│  Type  /approve  to confirm                 │
│  Type  /dismiss  to reject                  │
└─────────────────────────────────────────────┘
```

---

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |
| `POST` | `/sessions` | Create new session |
| `GET` | `/sessions` | List all sessions |
| `GET` | `/sessions/{id}` | Session detail + messages + workspace |
| `DELETE` | `/sessions/{id}` | Delete session |
| `POST` | `/ask` | Send user message (auto-creates session) |
| `POST` | `/sessions/{id}/approvals/{aid}` | Approve or reject pending action |
| `POST` | `/sessions/{id}/workspace` | Set workspace directory |

CORS allows all origins (`*`) for local development.

Full frontend integration guide: [`docs/interface-integration.md`](docs/interface-integration.md)

---

## Session Persistence

Sessions auto-persist to `data/sessions.json` via the `Storage` interface. Writes are atomic (temp file + rename). Restarting the server reloads all sessions, including pending approvals and workspace paths.

To swap the storage backend, implement `session.Storage` and pass it to `NewStoreWithStorage`.

Disable persistence: `SESSION_STORAGE_PATH=` (empty).

---

## Workspace

Each session can have its own workspace directory. The frontend sends the selected folder:

```http
POST /sessions/{sessionId}/workspace
{"path": "/Users/me/my-project"}
```

All tools (`read_file`, `git_context`, `run_command`, etc.) then operate inside that directory. Workspace is persisted across restarts.

---

## Logging

Structured logging via `log/slog` (stderr, text handler). Level controlled by `LOG_LEVEL`:

| Level | What you see |
|-------|-------------|
| `debug` | Every LLM call, tool execution with timings, message sizes |
| `info` | Loop iterations, planner decisions, tool errors (default) |
| `warn` | Guardrail stops, tool failures |
| `error` | Panics with stack traces, critical errors |

```bash
LOG_LEVEL=debug go run ./cmd/api
```

---

## Configuration

Loaded from `.env`, `.env.local`, or environment variables.

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENAI_API_KEY` | `API_KEY` | LLM API key |
| `LLM_BASE_URL` | `https://api.deepseek.com/v1/chat/completions` | Chat completions endpoint |
| `LLM_MODEL` | `deepseek-chat` | Model name |
| `LLM_TEMPERATURE` | `0.7` | LLM temperature |
| `LLM_MAX_TOKENS` | `2048` | Max response tokens |
| `API_KEY` | — | Fallback API key |
| `COINGECKO_API_KEY` | — | Optional CoinGecko key |
| `TIMEOUT_SECONDS` | `30` | HTTP client timeout |
| `SESSION_STORAGE_PATH` | `data/sessions.json` | Session persistence path |
| `LOG_LEVEL` | `info` | debug, info, warn, error |

---

## Adding a New Tool

1. Implement `tools.Tool` (and optionally `ApprovalAwareTool`, `WorkspaceTool`).
2. Register it in `harness.New`.
3. The planner automatically discovers it via the registry.

---

## Direction

The project stays intentionally simple:

- Short-term memory with optional JSON persistence
- Harness owns shared dependencies; sessions own state
- Loop owns iteration logic + approval gate
- Guardrails run before every planning step
- Native Go tools with explicit risk levels
- Swappable session storage

Long-term memory, MCP adapters, evals, and alternative storage backends are natural next steps — once this core loop feels boring and obvious.
