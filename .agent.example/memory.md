# Project Memory

## How to run
- Server: `go run ./cmd/api`
- Tests: `go test ./...`
- Race tests: `go test -race ./...`

## Decisions
- Keep the server and Flutter client concerns separate unless the user explicitly asks to work on the client.
- Approval flow for interactive UI uses SSE `/sessions/{sessionID}/stream` plus `/approve` or `/reject`.
- For cryptocurrency price/rank/market questions, the agent should try `get_crypto_price` before refusing.

## User Preferences
- Answer in Russian by default.
- Be direct, practical, and explain changes in small steps.

## Notes
- This file is a template. Copy it to `.agent/memory.md` for local project memory.
