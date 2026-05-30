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
    └── main.go              # Interactive CLI. Creates one Harness and reuses it.

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
├── llm/
│   ├── client.go            # OpenAI-compatible HTTP client.
│   ├── model.go             # Model endpoint/key/name config.
│   ├── interface.go         # LlmClient interface.
│   └── type.go              # Chat request/response DTOs.
│
├── config/
│   └── config.go            # Env-based config loader.
│
└── agent/
    └── agent.go             # Thin compatibility facade over Harness.
```

---

## Runtime Flow

`cmd/cli` creates the harness once:

```go
cfg, _ := config.Load()
h := harness.New(cfg)
```

For every user input it calls:

```go
result := h.Run(input)
fmt.Println(result.LoopResult.Answer)
```

Inside one run:

```text
1. Harness creates a fresh WorkMemory for the task.
2. WorkMemory adds:
   - default system prompt
   - user task
3. Loop checks guardrails.
4. Planner asks the LLM for a JSON plan:
   - action="message"  -> return answer
   - action="unknown"  -> return unsupported-action answer
   - action="<tool>"   -> executor runs a registered tool
5. Tool result/error is appended to WorkMemory.
6. Loop repeats until answer, guardrail stop, or error.
```

The trace is returned in `LoopResult.Trace`, so the caller can inspect what happened.

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

Important: create `Harness` once and reuse it. Do not recreate it for every input.

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
loop-owned iteration
guardrails before planning
native Go tools
```

Long-term memory, MCP adapters, sessions, and evals can be added later once this core loop feels boring and obvious.
