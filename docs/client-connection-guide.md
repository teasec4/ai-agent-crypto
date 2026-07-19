# Client Connection Guide

This is the practical client-side contract for the current server.

Base URL in local development:

```text
http://localhost:8080
```

For Android emulator use:

```text
http://10.0.2.2:8080
```

For iOS simulator and macOS client use:

```text
http://127.0.0.1:8080
```

For a physical iOS or Android phone on the same Wi-Fi as the backend computer, use the computer's LAN IP in the app's Server field:

```text
http://192.168.1.20:8080
```

`localhost` on a phone means the phone itself, not the computer running `go run ./cmd/api`.

## Recommended Client Flow

Use this project-first flow for the app UI:

1. Check server health with `GET /health`.
2. Show New Project plus the existing projects list on the same screen.
3. For New Project, browse folders on the backend computer with `/workspace/roots` and `/workspace/browse`.
4. Create the hidden backend session with `POST /sessions`.
5. Connect the device with `POST /sessions/{sessionId}/connect` and keep the returned `clientId`.
6. Open `GET /sessions/{sessionId}/events?clientId=...` for shared live updates.
7. Bind the project folder with `POST /sessions/{sessionId}/workspace` using `clientId`.
8. Send messages through `POST /sessions/{sessionId}/stream` using `clientId`.
9. Render direct stream events for the writer and shared `/events` updates for other devices.
10. If `approval_required` arrives, only the writer calls `/approve` or `/reject`.

`/ask` is still available, but `/stream` is better for UI because it shows thinking/tool progress and supports interactive approvals.

The UI should call these records "projects". `sessionId` is an internal transport detail.

## Health

Request:

```http
GET /health
```

Response:

```json
{
  "status": "ok"
}
```

## Create Session

Request:

```http
POST /sessions
```

Response:

```json
{
  "sessionId": "abc123"
}
```

Store `sessionId` in client state. Reuse it for follow-up messages.

## Connect Client

Each opened app window/device connects to a session and receives a role. The first connected client becomes `writer`; later clients become `viewer` until control is handed off.

Request:

```http
POST /sessions/abc123/connect
Content-Type: application/json
```

```json
{
  "clientId": "optional-existing-client-id"
}
```

Response:

```json
{
  "clientId": "client-phone",
  "role": "writer",
  "writerClientId": "client-phone",
  "session": {
    "id": "abc123",
    "sessionId": "abc123",
    "messageCount": 0,
    "messages": []
  }
}
```

## Live Session Events

Open this stream after connect. It is shared session state: viewers see messages, tool progress, final answers, and writer handoff events.

```http
GET /sessions/abc123/events?clientId=client-phone
Accept: text/event-stream
```

Common events:

| Event | Meaning |
|---|---|
| `message` | A client added a user message |
| `tool_start`, `tool_done`, `tool_error` | Agent tool progress |
| `approval_required` | Writer must approve a tool action |
| `done` | Final assistant answer |
| `writer_request_created` | Viewer requested control |
| `writer_request_resolved` | Writer approved/rejected the request |
| `writer_changed` | Writer role moved to another client |

## Set Workspace

The workspace is the project folder where file/git/command tools operate.

Request:

```http
POST /sessions/abc123/workspace
Content-Type: application/json
```

```json
{
  "path": "/Users/me/project",
  "clientId": "client-phone"
}
```

Response:

```json
{
  "id": "abc123",
  "sessionId": "abc123",
  "createdAt": "2026-07-18T10:00:00+08:00",
  "updatedAt": "2026-07-18T10:01:00+08:00",
  "messageCount": 1,
  "workspace": "/Users/me/project"
}
```

If workspace is not set, tools use the server process working directory.

## Browse Backend Folders

Mobile clients cannot use the phone's native file picker to select folders on the computer running the Go API. Use the backend folder browser instead.

Roots:

```http
GET /workspace/roots
```

Response:

```json
{
  "roots": [
    {
      "path": "/Users/me/go/ai-agent",
      "name": "ai-agent",
      "kind": "cwd"
    },
    {
      "path": "/Users/me",
      "name": "me",
      "kind": "home"
    }
  ]
}
```

Browse:

```http
GET /workspace/browse?path=/Users/me
```

Response:

```json
{
  "path": "/Users/me",
  "roots": [
    {
      "path": "/Users/me",
      "name": "me",
      "kind": "home"
    }
  ],
  "entries": [
    {
      "name": "project-a",
      "path": "/Users/me/project-a",
      "isDir": true
    }
  ]
}
```

The client should let the user navigate directories and then pass the selected `path` to `/sessions/{sessionId}/workspace`. Browsing is limited to the roots returned by the backend, so clients should hide the Up action when `parentPath` is absent.

## Simple Non-Streaming Message

Request:

```http
POST /ask
Content-Type: application/json
```

```json
{
  "sessionId": "abc123",
  "message": "прочитай README и скажи как запустить проект"
}
```

Response:

```json
{
  "sessionId": "abc123",
  "answer": "Проект запускается командой `go run ./cmd/api`...",
  "iterations": 2,
  "stoppedBy": "success",
  "trace": [
    {
      "index": 1,
      "outcome": "tool_calls",
      "toolEvents": [
        {
          "tool": "read_file",
          "args": {
            "path": "README.md"
          },
          "result": "File: README.md\n---\n..."
        }
      ],
      "contextSize": 4
    },
    {
      "index": 2,
      "outcome": "answer",
      "contextSize": 5
    }
  ]
}
```

Possible `stoppedBy` values:

| Value | Meaning |
|---|---|
| `success` | Agent completed normally |
| `approval_required` | Tool needs user approval |
| `model` | Model refused or returned unknown action |
| `guardrail` | Iteration/deadline guardrail stopped execution |
| `error` | Internal, LLM, or tool error |

If `stoppedBy` is `approval_required`, response also includes:

```json
{
  "pendingAction": {
    "id": "action-id",
    "tool": "write_file",
    "risk": "write",
    "summary": "Create file notes.md",
    "preview": "Will write file:\nnotes.md\n(120 bytes)",
    "args": {
      "path": "notes.md"
    },
    "createdAt": "2026-07-18T10:02:00+08:00"
  }
}
```

For approval UX, prefer streaming mode below.

## Streaming Message

Request:

```http
POST /sessions/abc123/stream
Content-Type: application/json
Accept: text/event-stream
```

```json
{
  "message": "найди цену ETH и потом объясни что сделал",
  "clientId": "client-phone"
}
```

Response content type:

```text
text/event-stream
```

The server sends frames like:

```text
event: thinking
data: {"type":"thinking"}

event: tool_start
data: {"type":"tool_start","tool":"get_crypto_price","args":{"query":"ETH"}}

event: tool_done
data: {"type":"tool_done","tool":"get_crypto_price","args":{"query":"ETH"},"result":"Ethereum price: ..."}

event: done
data: {"type":"done","answer":"ETH сейчас стоит ..."}

event: close
data: {}
```

## SSE Event Types

| Event | Data fields | What client should do |
|---|---|---|
| `thinking` | `type` | Show loading/thinking state |
| `tool_start` | `type`, `tool`, `args` | Show tool started |
| `tool_done` | `type`, `tool`, `args`, `result` | Show tool completed |
| `tool_error` | `type`, `tool`, `args`, `error`, `result` | Show tool error |
| `approval_required` | `type`, `tool`, `args`, `action` | Show approve/reject UI |
| `done` | `type`, `answer` | Add assistant message |
| `close` | empty object | Close stream reader |

## Approval Flow

When a write/exec tool is needed, the stream sends:

```text
event: approval_required
data: {"type":"approval_required","tool":"write_file","args":{"path":"notes.md"},"action":{"id":"...","tool":"write_file","risk":"write","summary":"Create file notes.md","preview":"...","args":{"path":"notes.md"},"createdAt":"..."}}
```

Client should show a confirmation card only when it is the current writer.

Approve:

```http
POST /sessions/abc123/approve
Content-Type: application/json
```

```json
{
  "clientId": "client-phone"
}
```

Response:

```json
{
  "status": "approved"
}
```

Reject:

```http
POST /sessions/abc123/reject
Content-Type: application/json
```

```json
{
  "clientId": "client-phone"
}
```

Response:

```json
{
  "status": "rejected"
}
```

Important: approval/rejection is sent as a separate HTTP request while the original SSE stream remains open.

## Writer Handoff

A viewer can request control:

```http
POST /sessions/abc123/writer/request
Content-Type: application/json
```

```json
{
  "clientId": "client-laptop"
}
```

The current writer receives `writer_request_created` on `/events`. To approve:

```http
POST /sessions/abc123/writer/approve
Content-Type: application/json
```

```json
{
  "clientId": "client-phone",
  "requestId": "request-id"
}
```

To reject, use `POST /sessions/abc123/writer/reject` with the same body. When approved, every connected client receives `writer_changed`.

## Load Existing Session

Request:

```http
GET /sessions/abc123
```

Response:

```json
{
  "id": "abc123",
  "sessionId": "abc123",
  "createdAt": "2026-07-18T10:00:00+08:00",
  "updatedAt": "2026-07-18T10:05:00+08:00",
  "messageCount": 4,
  "workspace": "/Users/me/project",
  "messages": [
    {
      "role": "user",
      "content": "прочитай README"
    },
    {
      "role": "assistant",
      "content": "README описывает..."
    }
  ]
}
```

The server hides system messages and tool observations from this response.

## List Sessions

Request:

```http
GET /sessions
```

Response:

```json
[
  {
    "id": "abc123",
    "createdAt": "2026-07-18T10:00:00+08:00",
    "updatedAt": "2026-07-18T10:05:00+08:00",
    "messageCount": 4,
    "workspace": "/Users/me/project"
  }
]
```

## Delete Session

Request:

```http
DELETE /sessions/abc123
```

Response:

```json
{
  "status": "deleted"
}
```

## Common Errors

All JSON errors use this shape:

```json
{
  "error": "message is required"
}
```

Common statuses:

| Status | Meaning |
|---|---|
| `400` | Bad JSON, empty message, invalid workspace |
| `404` | Session not found |
| `409` | Session already has an active agent run or active SSE stream |
| `500` | Server error |

The client should block sending another message to the same session while a stream is active.

## Minimal JavaScript Streaming Example

```js
async function streamMessage(sessionId, message, onEvent) {
  const response = await fetch(`http://localhost:8080/sessions/${sessionId}/stream`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Accept": "text/event-stream",
    },
    body: JSON.stringify({ message }),
  });

  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.error);
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { value, done } = await reader.read();
    if (done) break;

    buffer += decoder.decode(value, { stream: true });
    const frames = buffer.split("\n\n");
    buffer = frames.pop() ?? "";

    for (const frame of frames) {
      const dataLine = frame.split("\n").find((line) => line.startsWith("data: "));
      if (!dataLine) continue;

      const event = JSON.parse(dataLine.slice("data: ".length));
      onEvent(event);
    }
  }
}
```

## Client State Shape

Recommended minimal client state:

```ts
type ClientState = {
  serverUrl: string;
  sessionId: string | null;
  workspace: string | null;
  messages: Array<{ role: "user" | "assistant"; content: string }>;
  activeTool: string | null;
  pendingAction: PendingAction | null;
  loading: boolean;
  error: string | null;
};
```

When `approval_required` arrives, set `pendingAction`.

When `/approve` or `/reject` succeeds, keep reading the same stream.

When `done` arrives, append assistant message and set `loading = false`.

When `close` arrives, close/cleanup stream resources.
