# AI Agent 🤖

![Go Version](https://img.shields.io/badge/Go-1.26-blue?logo=go)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
![Status](https://img.shields.io/badge/Status-Active-brightgreen)
![Architecture](https://img.shields.io/badge/Architecture-Plan--Execute--Finalize-orange)

A modular, extensible AI agent written in Go. The agent uses an LLM (any OpenAI-compatible API) to plan actions, executes registered tools, and returns a natural-language response — all in a single loop iteration without wasted LLM calls.

---

## How It Works

```
┌─────────────────────────────────────────────────────────────────┐
│                         AGENT LOOP                              │
│                                                                 │
│  ┌────────┐    ┌──────────┐    ┌─────────┐    ┌──────────────┐ │
│  │  User  │    │  LLM     │    │ Tool    │    │  format       │ │
│  │ Input  │───▶│ Planner  │───▶│Executor │───▶│  Response     │ │
│  └────────┘    └────┬─────┘    └────┬────┘    └──────┬───────┘ │
│                     │               │                 │         │
│                     │         ┌─────▼──────┐          │         │
│                     │         │ Real tool  │          │         │
│                     │         │ succeeded? │          │         │
│                     │         │            │          │         │
│                     │         │  ┌───┐     │          │         │
│                     │         │  │YES│─────┼──────────┘         │
│                     │         │  └───┘     │                    │
│                     │         │            │                    │
│                     │         │  ┌───┐     │                    │
│                     │         │  │NO │──┐  │                    │
│                     │         │  └───┘  │  │                    │
│                     │         └─────────┘  │                    │
│                     │               ▲      │                    │
│                     │               │      │                    │
│                     │         ┌─────┴──────▼──┐                 │
│                     │         │  Retry with   │                 │
│                     │◄────────┤  next plan    │                 │
│                     │         └───────────────┘                 │
└─────────────────────────────────────────────────────────────────┘
```

**Key insight:** Once a real tool executes successfully, the agent immediately formats the response — no redundant LLM call needed. Only "unknown" fallbacks or tool errors trigger a re-plan.

---

## Architecture

```
cmd/
└── cli/
    └── main.go          # Entry point: wires everything together

internal/
├── agent/
│   ├── agent.go         # Agent loop: Plan → Act → Finalize
│   └── memory.go        # Writes structured long-term memory events
│
├── planner/
│   ├── llm_planner.go   # LLM-backed planner implementation
│   └── type.go          # PlanResult: action, params, reply
│
├── executor/
│   └── executor.go      # ToolExecutor: resolves plan → registered tool
│
├── llm/
│   ├── interface.go      # LlmClient interface
│   ├── client.go         # HTTP client for OpenAI-compatible APIs
│   └── type.go           # Message, Request, Response types
│
├── tools/
│   ├── interface.go      # Tool interface (Name, Description, Run)
│   ├── crypto.go         # Cryptocurrency price (CoinGecko)
│   ├── git.go            # Git repository context
│   ├── help.go           # Help tool
│   └── registry/
│       └── registry.go   # Tool registry: register, lookup, list
│
├── memory/
│   ├── work_memory.go    # Short-term conversation history + compaction
│   ├── store.go          # Structured memory event types + Store interface
│   ├── long_term_memory.go # Read/write facade for long-term memory
│   ├── json_store.go     # JSONL append-only long-term memory
│   └── context_builder.go # Selects long-term memory for planner context
│
└── config/
    └── config.go         # Env-based configuration (.env / .env.local)
```

---

## Agent Loop in Detail

```
┌──────────────────────────────────────────────────────────────────────┐
│                        ITERATION FLOW                                │
│                                                                      │
│  Step 1: COMPACT ?                                                   │
│    • If history exceeds compactAt (10), summarise old messages via   │
│      LLM into a single context message. Fallback: trim oldest.       │
│                                                                      │
│  Step 2: PLAN                                                        │
│    • LLMPlanner builds a message chain:                              │
│        [system: tool descriptions] + [history messages]              │
│    • LLM returns JSON: { action, parameters, reply }                 │
│    • If no available action fits → returns action="unknown"          │
│                                                                      │
│  Step 3: ACT                                                         │
│    • Executor resolves action → registered Tool                      │
│    • Tool.Run(params) returns (string, error)                        │
│                                                                      │
│  Step 4: DECIDE (this is the critical part)                          │
│                                                                      │
│    ┌──────────── Result ────────────┐                                │
│    │                                │                                │
│    ▼                                ▼                                │
│  Success + real tool            Error / "unknown"                    │
│  (not "unknown")                                                     │
│    │                                │                                │
│    │                                ▼                                │
│    │                          Save result to history                 │
│    │                          (planner retries next iter)            │
│    │                                │                                │
│    ▼                                │                                │
│  formatResponse():                  │                                │
│    [system] + [user: original       │                                │
│     input + tool result]            │                                │
│    → LLM formats natural reply   ◄──┘ (loops back to Step 2)        │
│                                                                      │
│  Step 5: RETURN final answer to user                                 │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### Why no re-plan on success?

Older agent architectures call the LLM again after every tool result to check "are we done?" — wasting tokens and creating a failure point. This agent uses a **Plan-Execute-Finalize** pattern:

| Step | Who | What |
|------|-----|------|
| Plan | LLM | Decide which tool to call, with what params |
| Execute | Tool | Run the actual tool logic |
| Finalize | LLM | One-shot formatting of result into natural language |

No double LLM call for the same tool result.

---

## Quick Start

```bash
# Clone and enter
git clone <repo> && cd ai-agent

# Configure
cp .env.example .env   # or edit .env
# Set LLM_BASE_URL, LLM_MODEL, API_KEY

# Run
go run ./cmd/cli
```

### Configuration

All config via `.env` or environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `LLM_BASE_URL` | `https://api.deepseek.com/v1/chat/completions` | OpenAI-compatible API endpoint |
| `LLM_MODEL` | `deepseek-chat` | Model name |
| `LLM_TEMPERATURE` | `0.7` | LLM temperature |
| `LLM_MAX_TOKENS` | `2048` | Max tokens per response |
| `API_KEY` | — | Primary API key |
| `OPENAI_API_KEY` | falls back to `API_KEY` | OpenAI-compat key |
| `TIMEOUT_SECONDS` | `30` | HTTP client timeout |
| `MEMORY_PATH` | `.agent/memory/events.jsonl` | JSONL long-term memory path |
| `MEMORY_SESSION_ID` | `default` | Memory session namespace |
| `MEMORY_CONTEXT_LIMIT` | `8` | Recent memory events to inject into planner context |

---

## Tools

| Tool | Name | What it does |
|------|------|-------------|
| CryptoTool | `get_crypto_price` | Fetches crypto prices via CoinGecko |
| GitTool | `git_context` | Git status, branch, log, diff |
| HelpTool | `help` | Lists available tools |

`unknown` is a planner action, not a registered tool. The agent uses it to re-plan once, then returns a friendly unsupported-request message if no available action fits.

### Adding a new tool

```go
// 1. Implement the Tool interface
type MyTool struct{}
func (t *MyTool) Name() string        { return "my_tool" }
func (t *MyTool) Description() string { return "What my tool does" }
func (t *MyTool) Run(params map[string]interface{}) (string, error) {
    // your logic here
}

// 2. Register in main.go
reg := registry.New(cryptoTool, gitTool, helpTool, myTool)
```

The planner automatically discovers new tools via the registry — no prompt changes needed.

---

## History Compaction

Long conversations are automatically compressed to save tokens:

```
┌────────────────────────────────────────────────────────┐
│  Before compaction (12 messages)                       │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌───  │
│  │ msg1 │ │ msg2 │ │ msg3 │ │ msg4 │ │ msg5 │ │ ... │
│  └──────┘ └──────┘ └──────┘ └──────┘ └──────┘ └───  │
│         │                                   │         │
│         ▼  LLM summarises into               ▼ kept   │
│         │                                   │         │
│  ┌──────────────────────┐ ┌──────┐ ┌──────┐ ┌───     │
│  │ [summary context]    │ │ msg9 │ │ msg10│ │ ...     │
│  └──────────────────────┘ └──────┘ └──────┘ └───     │
│                        After compaction                 │
└────────────────────────────────────────────────────────┘
```

Triggers at `compactAt` (10 messages). If LLM summarisation fails, falls back to keeping the last N/2 messages.

---

## Structured Memory

The agent keeps short-term working memory in-process and writes long-term structured events to JSONL:

```text
.agent/memory/events.jsonl
```

Each event records a type, session, timestamp, optional action/params/result/error, and tags. `LongTermMemory` hides the JSONL store and context builder behind one read/write facade. It reads recent events from previous runs and injects a compact `system` message before planning. The current run is filtered out by timestamp, so the active user request is not duplicated in long-term context.

The `.agent/` directory is ignored by git because it can contain local user data.

---

## Dependency Graph

```
main.go
  ├── config.Load()           → Config struct
  ├── registry.New(tools...)  → Registry
  ├── llm.NewClientWithTimeout() → LlmClient
  └── agent.NewAgent()        → Agent
        ├── planner.NewLLMPlanner(llmClient, registry)
        └── executor.New(registry)
              └── registry.Get(name) → Tool.Run(params)
```

---

## Development

```bash
go build ./...     # Build all packages
go vet ./...       # Static analysis
go run ./cmd/cli   # Run interactive CLI
go run ./cmd/test_llm  # LLM connectivity test
```

---

## License

MIT
