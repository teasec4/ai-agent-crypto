# Client API Reference

## Overview

AI Agent exposes an HTTP API for sending tasks, receiving responses, and streaming agent progress via SSE. The agent uses native OpenAI-compatible tool calling — tools are passed as `tools[]` in the LLM request, not embedded in system prompts.

### Base URL

```
http://localhost:8080
```

### Content Type

All request/response bodies are `application/json`. SSE streams use `text/event-stream`.

---

## Endpoints

### `GET /health`

Health check.

```
> GET /health

< 200
{"status":"ok"}
```

---

### `POST /ask`

Send a message to the agent. The agent runs its loop and returns the final answer. Write/exec tools are auto-approved (no user confirmation required).

**Request:**

```json
{
  "sessionId": "abc123",     // optional — omit to auto-create
  "message": "прочитай go.mod"
}
```

**Response:**

```json
{
  "sessionId": "abc123",
  "answer": "Файл go.mod содержит...",
  "iterations": 2,
  "stoppedBy": "success",
  "trace": [
    {
      "index": 1,
      "outcome": "tool_calls",
      "toolEvents": [{"tool": "read_file", "args": {"path": "go.mod"}, "result": "..."}],
      "contextSize": 4
    },
    {
      "index": 2,
      "outcome": "answer",
      "toolEvents": null,
      "contextSize": 5
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `sessionId` | string | Session ID (auto-created or provided) |
| `answer` | string | Final agent answer |
| `iterations` | int | How many loop iterations were executed |
| `stoppedBy` | string | `"success"`, `"guardrail"`, `"error"` |
| `trace` | array | Detailed trace of each iteration |

**Trace Iteration:**

| Field | Type | Description |
|-------|------|-------------|
| `index` | int | Iteration number |
| `outcome` | string | `"tool_calls"` or `"answer"` |
| `toolEvents` | array | Tools executed in this iteration |
| `contextSize` | int | Messages in context after this iteration |

**Tool Event:**

| Field | Type | Description |
|-------|------|-------------|
| `tool` | string | Tool name (e.g. `"read_file"`) |
| `args` | object | Parameters passed to the tool |
| `result` | string | Tool output |
| `error` | string | Error message, if any |

---

### `POST /sessions`

Create a new empty session.

```
> POST /sessions

< 201
{"sessionId": "abc123"}
```

---

### `GET /sessions`

List all sessions.

```
> GET /sessions

< 200
[
  {
    "id": "abc123",
    "createdAt": "2026-07-14T09:00:00Z",
    "updatedAt": "2026-07-14T09:05:00Z",
    "messageCount": 5,
    "workspace": "/Users/me/project"
  }
]
```

---

### `GET /sessions/{sessionID}`

Get session details with messages.

```
> GET /sessions/abc123

< 200
{
  "id": "abc123",
  "sessionId": "abc123",
  "createdAt": "2026-07-14T09:00:00Z",
  "updatedAt": "2026-07-14T09:05:00Z",
  "messageCount": 5,
  "messages": [
    {
      "role": "user",
      "content": "прочитай go.mod"
    },
    {
      "role": "assistant",
      "content": "",
      "tool_calls": [
        {
          "id": "call_...",
          "type": "function",
          "function": {
            "name": "read_file",
            "arguments": "{\"path\":\"go.mod\"}"
          }
        }
      ]
    },
    {
      "role": "tool",
      "content": "Tool read_file result:\nFile: go.mod\n...",
      "tool_call_id": "call_..."
    },
    {
      "role": "assistant",
      "content": "Вот содержимое go.mod..."
    }
  ],
  "workspace": "/Users/me/project"
}
```

**Message roles:**

| Role | Description |
|------|-------------|
| `user` | User message |
| `assistant` | Agent response — may contain `tool_calls` |
| `tool` | Tool result — paired via `tool_call_id` |

---

### `DELETE /sessions/{sessionID}`

Delete a session.

```
> DELETE /sessions/abc123

< 200
{"status": "deleted"}
```

---

### `POST /sessions/{sessionID}/workspace`

Set the workspace directory for a session. All file tools (`read_file`, `write_file`, etc.) operate relative to this directory.

```
> POST /sessions/abc123/workspace
{"path": "/Users/me/project"}

< 200
{...session details...}
```

---

### `POST /sessions/{sessionID}/stream` (SSE)

Send a message and receive **streaming events** via Server-Sent Events (SSE). The connection stays open, sending each agent action as a separate event.

```
> POST /sessions/abc123/stream
{"message": "прочитай go.mod"}
```

**Response** (`Content-Type: text/event-stream`):

```
event: thinking
data: {"type":"thinking"}

event: tool_start
data: {"type":"tool_start","tool":"read_file","args":{"path":"go.mod"}}

event: tool_done
data: {"type":"tool_done","tool":"read_file","args":{"path":"go.mod"},"result":"File: go.mod\n---\n    1\t..."}

event: thinking
data: {"type":"thinking"}

event: done
data: {"type":"done","answer":"Вот содержимое go.mod..."}

event: close
data: {}
```

**Event Types:**

| Event | `data.type` | Fields | Description |
|-------|------------|--------|-------------|
| `thinking` | `"thinking"` | — | Agent is processing |
| `tool_start` | `"tool_start"` | `tool`, `args` | Agent started executing a tool |
| `tool_done` | `"tool_done"` | `tool`, `args`, `result` | Tool completed successfully |
| `tool_error` | `"tool_error"` | `tool`, `args`, `error`, `result` | Tool failed |
| `approval_required` | `"approval_required"` | `tool`, `args`, `action` | Tool needs user confirmation |
| `done` | `"done"` | `answer` | Final answer |
| `close` | — | — | Server closed the stream |

**Approval flow with SSE:**

```
← event: approval_required
   data: {"type":"approval_required","tool":"write_file","args":{...},"action":{"id":"...","summary":"Create file test.txt","preview":"..."}}

→ POST /sessions/abc123/approve   (or POST /sessions/abc123/reject)

← event: tool_done / tool_error
← event: done
```

The SSE connection **stays open** while waiting for approval. Send a separate HTTP request to `/approve` or `/reject` to unblock the agent.

#### curl example

```bash
curl -N -X POST http://localhost:8080/sessions/abc123/stream \
  -H "Content-Type: application/json" \
  -d '{"message":"прочитай go.mod"}'
```

#### JavaScript (browser) example

```javascript
// 1. Create session
const { sessionId } = await fetch('http://localhost:8080/sessions', { method: 'POST' }).then(r => r.json());

// 2. Listen to SSE
const response = await fetch(`http://localhost:8080/sessions/${sessionId}/stream`, {
  method: 'POST',
  headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ message: 'прочитай go.mod' })
});

const reader = response.body.getReader();
const decoder = new TextDecoder();

while (true) {
  const { done, value } = await reader.read();
  if (done) break;

  const text = decoder.decode(value);
  for (const line of text.split('\n')) {
    if (line.startsWith('data: ')) {
      const event = JSON.parse(line.slice(6));
      switch (event.type) {
        case 'thinking':
          showSpinner(); break;
        case 'tool_start':
          console.log(`→ ${event.tool}`, event.args); break;
        case 'tool_done':
          console.log(`✓ ${event.tool}`); break;
        case 'approval_required':
          showApproveButton(event.action); break;
        case 'done':
          showAnswer(event.answer); break;
      }
    }
  }
}

// 3. Approve (separate request)
if (needsApproval) {
  await fetch(`http://localhost:8080/sessions/${sessionId}/approve`, { method: 'POST' });
}
```

#### Flutter (dart:io) example

```dart
import 'dart:convert';
import 'dart:io';

Future<void> streamTask(String sessionId, String message) async {
  final client = HttpClient();
  final request = await client.postUrl(Uri.parse('http://localhost:8080/sessions/$sessionId/stream'));
  request.headers.contentType = ContentType.json;
  request.write(jsonEncode({'message': message}));

  final response = await request.close();
  final stream = response.transform(utf8.decoder).transform(const LineSplitter());

  await for (final line in stream) {
    if (line.startsWith('data: ')) {
      final event = jsonDecode(line.substring(6));
      switch (event['type']) {
        case 'tool_start':
          print('→ ${event['tool']}');
        case 'tool_done':
          print('✓ ${event['tool']}');
        case 'done':
          print('Answer: ${event['answer']}');
        case 'close':
          client.close();
      }
    }
  }
}
```

---

### `POST /sessions/{sessionID}/approve`

Approve a pending tool action during an SSE stream.

```
> POST /sessions/abc123/approve

< 200
{"status": "approved"}
```

---

### `POST /sessions/{sessionID}/reject`

Reject a pending tool action during an SSE stream.

```
> POST /sessions/abc123/reject

< 200
{"status": "rejected"}
```

---

## CLI Mode

The CLI runs the same agent loop with inline approval:

```bash
go run ./cmd/cli
```

```
AI Agent ready. Commands: /reset, Ctrl+C to exit.

> прочитай go.mod
.
  → read_file... ✓
Agent: Вот содержимое go.mod...

> напиши привет мир в test.txt
.
  → write_file...
┌─────────────────────────────────────────────┐
│  ⚡  NEEDS APPROVAL                         │
├─────────────────────────────────────────────┤
│  Tool:    write_file                        │
│  Risk:    write                             │
│  Summary: Create file test.txt              │
├─────────────────────────────────────────────┤
│  Type  y/yes/да  to confirm                │
│  Type  anything else to reject              │
└─────────────────────────────────────────────┘
  > y ✓
Agent: Готово! Создал файл test.txt.
```

| Command | Action |
|---------|--------|
| `/reset` | Clear conversation context |
| `y` / `yes` / `да` | Approve pending action |
| Any other input | Reject pending action |

---

## Available Tools

| Tool | Action | Risk | Approval |
|------|--------|------|----------|
| Crypto | `get_crypto_price` | read | — |
| Git | `git_context` | read | — |
| List Dir | `list_directory` | read | — |
| Read File | `read_file` | read | — |
| Find Files | `find_files` | read | — |
| Search Text | `search_text` | read | — |
| Create Dir | `create_directory` | write | required |
| Write File | `write_file` | write | required |
| Edit File | `edit_file` | write | required |
| Run Command | `run_command` | exec | required |

Read-only tools execute immediately. Write/exec tools require user approval (via CLI y/n or SSE approve/reject).

---

## Configuration

Environment variables (`.env`):

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENAI_API_KEY` | `API_KEY` | LLM API key |
| `LLM_BASE_URL` | `https://api.deepseek.com/v1/chat/completions` | Chat completions endpoint |
| `LLM_MODEL` | `deepseek-chat` | Model name |
| `LLM_TEMPERATURE` | `0.7` | LLM temperature |
| `LLM_MAX_TOKENS` | `2048` | Max response tokens |
| `COINGECKO_API_KEY` | — | Optional CoinGecko key |
| `TIMEOUT_SECONDS` | `30` | HTTP client timeout |
| `SESSION_STORAGE_PATH` | `data/sessions.json` | Session persistence path |
| `LOG_LEVEL` | `info` | debug, info, warn, error |

---

## Example: Full Flow (curl)

```bash
# 1. Start server
go run ./cmd/api &

# 2. Create session
SESSION=$(curl -s -X POST http://localhost:8080/sessions | python3 -c "import sys,json;print(json.load(sys.stdin)['sessionId'])")

# 3. Set workspace
curl -s -X POST "http://localhost:8080/sessions/$SESSION/workspace" \
  -H "Content-Type: application/json" \
  -d '{"path":"/Users/me/project"}'

# 4. Ask a question
curl -s -X POST http://localhost:8080/ask \
  -H "Content-Type: application/json" \
  -d "{\"sessionId\":\"$SESSION\",\"message\":\"найди все go файлы\"}"

# 5. Or stream the result
curl -N -X POST "http://localhost:8080/sessions/$SESSION/stream" \
  -H "Content-Type: application/json" \
  -d '{"message":"прочитай go.mod"}'
```
