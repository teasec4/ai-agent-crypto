# AI Agent

Go-based AI coding assistant with native OpenAI tool calling, SSE streaming, and workspace sandboxing.

## Quick Start

```bash
cp .env.example .env
# Edit .env with your API key
go run ./cmd/api
```

```bash
# REST API
curl -X POST http://localhost:8080/ask \
  -H "Content-Type: application/json" \
  -d '{"message":"hello"}'

# SSE streaming
curl -N -X POST http://localhost:8080/sessions/{id}/stream \
  -H "Content-Type: application/json" \
  -d '{"message":"найди все .go файлы"}'

# CLI mode
go run ./cmd/cli
```

## Architecture

```
Entry Points:  CLI (cmd/cli)  │  API Server (cmd/api)
                               │
                    ┌──────────┴──────────┐
                    │      Harness        │
                    ├─────────────────────┤
                    │ Planner (LLM)       │ ← native tool_calls
                    │ Executor (10 tools) │ ← stateless, sandboxed
                    │ Loop (limits,       │
                    │   approval, SSE)    │
                    └──────────┬──────────┘
                               │
                    ┌──────────┴──────────┐
                    │                     │
             ┌──────┴──────┐    ┌─────────┴─────────┐
             │   Session   │    │   Tool Backends    │
             │  (JSON)     │    │ File, Shell, Git,  │
             │             │    │ HTTP (crypto)      │
             └─────────────┘    └───────────────────┘
```

## Key Features

| Feature | Details |
|---------|---------|
| **Native tool calling** | Tools passed as `tools[]` in API, LLM returns `tool_calls` |
| **SSE streaming** | Real-time events: `thinking → tool_start → tool_done → done` |
| **Approval flow** | Write/exec tools require user confirmation (CLI y/n or SSE approve/reject) |
| **Workspace sandbox** | `EvalSymlinks`, path traversal prevention, blocked paths |
| **Stateless tools** | No mutable singleton state, context propagated from loop |
| **Allowlist** | Commands limited to `go`, `git`, `ls`, `pwd` with strict arg validation |
| **Project memory** | Durable notes in `.agent/memory.md`, read automatically and editable by humans |

## Configuration

See `.env.example` for all options. Key settings:

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENAI_API_KEY` | — | API key (required) |
| `ALLOW_AUTO_APPROVE` | `false` | Auto-approve write/exec tools in `/ask` |
| `LLM_BASE_URL` | DeepSeek | OpenAI-compatible endpoint |
| `SESSION_TTL_SECONDS` | `0` | Session cleanup TTL; `0` keeps persisted sessions |

## Project Memory

The agent automatically reads `.agent/memory.md` from the workspace and includes it as durable project context. Keep it short and human-editable: run commands, decisions, user preferences, and stable project facts.

`.agent/` is intentionally git-ignored for local notes. Use `.agent.example/memory.md` as the starter template.

The `read_project_memory` tool can show the current memory, and `propose_memory_update` can suggest an entry without modifying files.

## Available Tools

| Tool | Action | Risk | Approval |
|------|--------|------|----------|
| `get_crypto_price` | Read crypto price | read | — |
| `git_context` | Git status/log/diff | read | — |
| `read_project_memory` | Read durable project notes | read | — |
| `propose_memory_update` | Suggest memory entry | read | — |
| `list_directory` | List workspace dirs | read | — |
| `read_file` | Read workspace files | read | — |
| `find_files` | Glob search | read | — |
| `search_text` | Text search | read | — |
| `create_directory` | Create dirs | write | required |
| `write_file` | Write files | write | required |
| `edit_file` | Edit files | write | required |
| `run_command` | Run allowlisted commands | exec | required |

## API

Full API reference in [CLIENT.md](CLIENT.md).
Client integration walkthrough in [docs/client-connection-guide.md](docs/client-connection-guide.md).
