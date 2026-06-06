# AI Agent

A small Go playground for building an LLM agent with explicit layers:

```text
cmd/cli -> harness -> loop -> planner -> executor -> tools
                         |
                         -> work memory + guardrails
```

The current goal is to keep the architecture understandable while experimenting with agent loops, validation, and tool execution.

---

## Current Architecture

```text
cmd/
└── cli/
    └── main.go              # Interactive CLI. Creates one Harness and one AgentSession.

internal/
├── harness/
│   └── harness.go           # Wiring layer: LLM client, tools, planner, executor, guardrails.
│
├── loop/
│   ├── loop.go              # Runtime loop: guardrail -> plan -> act/answer.
│   └── type.go              # LoopRequest, LoopResult, trace types.
│
├── guardrails/
│   └── guardrails.go        # Validation checks such as max iterations/messages.
│
├── planner/
│   ├── llm_planner.go       # LLM-backed planner. Returns JSON PlanResult.
│   └── type.go              # Actions: message, unknown, or tool name.
│
├── executor/
│   └── executor.go          # Resolves plan action to a registered tool.
│
├── tools/
│   ├── interface.go         # Tool interface.
│   ├── crypto.go            # CoinGecko crypto price tool.
│   ├── git.go               # Local git context tool.
│   ├── help.go              # Help tool.
│   └── registry/
│       └── registry.go      # Tool registry used by planner and executor.
│
├── memory/
│   └── work_memory.go       # Short-term message history and compaction.
│
├── session/
│   └── store.go             # In-memory API session store keyed by session ID.
│
├── llm/
│   ├── client.go            # OpenAI-compatible HTTP client.
│   ├── model.go             # Model endpoint/key/name config.
│   ├── interface.go         # LlmClient interface.
│   └── type.go              # Chat request/response DTOs.
│
├── config/
│   └── config.go            # Env-based config loader.
```

---

## Runtime Flow

`cmd/cli` creates the harness once, then creates one chat session:

```go
cfg, _ := config.Load()
h := harness.New(cfg)
session := h.NewAgentSession()
```

For every user input it calls:

```go
result := session.Run(input)
fmt.Println(result.LoopResult.Answer)
```

Inside one session:

```text
1. Harness owns reusable dependencies.
2. AgentSession owns WorkMemory.
3. AgentSession starts with:
   - default system prompt
4. Every user input is appended to the same WorkMemory.
5. Loop checks guardrails.
6. Planner asks the LLM for a JSON plan:
   - action="message"  -> return answer
   - action="unknown"  -> return unsupported-action answer
   - action="<tool>"   -> executor runs a registered tool
7. Tool result/error or final answer is appended to WorkMemory.
8. Loop repeats until answer, guardrail stop, or error.
```

The trace is returned in `LoopResult.Trace`, so the caller can inspect what happened.

Tool results are stored as regular `user` messages with the `Tool observation:` prefix instead of `role="tool"`. This keeps the history compatible with OpenAI-style Chat Completions APIs that require `tool_call_id` for native tool messages.

If the model returns a plain text answer immediately after a tool observation instead of planner JSON, the planner accepts it as the final answer for that turn. This is a defensive fallback for models that occasionally finalize after seeing tool output.

`Harness.Run(task)` still exists as a one-shot helper. It creates a fresh AgentSession internally, runs one task, and discards the memory. Interactive code should prefer `NewAgentSession`.

---

## Why Harness Exists

`Harness` owns infrastructure and wiring:

```text
LLM client
tool registry
planner
executor
guardrails
```

This keeps `loop` simple. The loop does not know about env vars, API keys, or tool construction. It only receives a `LoopRequest` with ready-to-use dependencies.

Important: create `Harness` once and reuse it. For chat-like behavior, create `AgentSession` once and call `session.Run(input)` for every user message.

## Why AgentSession Exists

`AgentSession` owns conversation state:

```text
WorkMemory
```

This keeps the harness reusable and keeps the loop stateless from the caller's point of view.

In the CLI:

```text
Harness = process-level dependencies
AgentSession = current conversation
Run     = one user turn inside that conversation
```

The `/reset` command clears the current session memory and keeps the same harness.

For the HTTP API, `internal/session.Store` keeps API session IDs mapped to `WorkMemory` instances. That separates request/session storage from the harness runtime object.

---

## Guardrails

Guardrails validate the loop before each planning step.

Current checks:

```text
MaxIterations(5)
MaxMessages(50)
```

They are combined in `harness.New`:

```go
guardrails.CombineGuardrails(
    guardrails.MaxIterations(loop.DefaultMaxIterations),
    guardrails.MaxMessages(loop.DefaultMaxMessages),
)
```

---

## Tools

Registered tools:

| Tool | Action | Purpose |
|------|--------|---------|
| CryptoTool | `get_crypto_price` | Fetch crypto price via CoinGecko |
| GitTool | `git_context` | Read local git branch/status/log/diff |
| HelpTool | `help` | Explain available capabilities |

`unknown` is not a tool. It is a planner action used when no available tool or direct answer fits.

To add a tool:

1. Implement `tools.Tool`.
2. Register it in `harness.New`.
3. Its description becomes visible to the planner through the registry.

---

## Configuration

Config is loaded from `.env`, `.env.local`, or environment variables.

| Variable | Default | Description |
|----------|---------|-------------|
| `LLM_BASE_URL` | `https://api.deepseek.com/v1/chat/completions` | OpenAI-compatible chat completions endpoint |
| `LLM_MODEL` | `deepseek-chat` | Model name |
| `LLM_TEMPERATURE` | `0.7` | Reserved config value |
| `LLM_MAX_TOKENS` | `2048` | Reserved config value |
| `API_KEY` | empty | Primary API key fallback |
| `OPENAI_API_KEY` | `API_KEY` | LLM API key |
| `COINGECKO_API_KEY` | empty | Optional CoinGecko API key |
| `TIMEOUT_SECONDS` | `30` | HTTP client timeout |

Note: `LLM_TEMPERATURE` and `LLM_MAX_TOKENS` are still in config, but the current `llm.Request` no longer sends them.

---

## Run

```bash
go run ./cmd/cli
```

Development checks:

```bash
go test ./...
go vet ./...
```

---

## Current Direction

The project is intentionally in a simpler phase now:

```text
short-term memory only
harness-owned dependencies
agent-session-owned conversation state
loop-owned iteration
guardrails before planning
native Go tools
```

Long-term memory, MCP adapters, persistent session storage, and evals can be added later once this core loop feels boring and obvious.
