# Interface Integration Guide

This document describes how to connect a UI to the `ai-agent` HTTP API. The backend supports chat sessions, read-only repository tools, approval-gated write/command tools, tool-call trace output, and JSON-backed session persistence.

---

## Quick Start

### Run the API

```bash
cd ai-agent
OPENAI_API_KEY=sk-... go run ./cmd/api
```

Default address: `http://localhost:8080`

### Required environment

| Variable | Example |
|---|---|
| `OPENAI_API_KEY` | `sk-abc123...` |

### Optional environment

| Variable | Default |
|---|---|
| `LLM_BASE_URL` | `https://api.deepseek.com/v1/chat/completions` |
| `LLM_MODEL` | `deepseek-chat` |
| `SESSION_STORAGE_PATH` | `data/sessions.json` |

### CORS

All origins are allowed by default:

```http
Access-Control-Allow-Origin: *
```

---

## API Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/health` | Health check |
| `POST` | `/sessions` | Create new session |
| `GET` | `/sessions` | List all sessions |
| `GET` | `/sessions/{sessionId}` | Session detail with messages |
| `POST` | `/ask` | Send user message |
| `POST` | `/chat/completion` | Alias for `/ask` |
| `POST` | `/sessions/{sessionId}/approvals/{approvalId}` | Approve or reject pending action |

All request/response bodies are JSON with `Content-Type: application/json`.

---

## Response Types (TypeScript)

```ts
// ── Health ──────────────────────────────────────────

type HealthResponse = {
  status: "ok";
};

// ── Ask / Chat ──────────────────────────────────────

type AskRequest = {
  sessionId?: string;  // omit on first message to auto-create
  message: string;
};

type AskResponse = {
  sessionId: string;
  answer: string;
  iterations: number;
  stoppedBy: "success" | "model" | "guardrail" | "approval_required" | "error";
  trace?: LoopIteration[];
  pendingAction?: PendingAction;
};

type LoopIteration = {
  index: number;
  outcome: "tool_calls" | "answer" | "error";
  toolEvents?: ToolEvent[];
  contextSize: number;
};

type ToolEvent = {
  tool: string;
  args: Record<string, unknown>;
  result?: string;
  error?: string;
};

type PendingAction = {
  id: string;
  tool: string;
  risk: "read" | "write" | "exec";
  summary: string;
  preview: string;
  args: Record<string, unknown>;
  createdAt: string;  // ISO 8601
};

// ── Approval ────────────────────────────────────────

type ApprovalRequest = {
  approved: boolean;
};

// Approval response has same shape as AskResponse.

// ── Sessions ────────────────────────────────────────

type SessionResponse = {
  sessionId: string;
};

type SessionListItem = {
  id: string;
  createdAt: string;
  updatedAt: string;
  messageCount: number;
};

type SessionDetailResponse = {
  id: string;
  sessionId: string;
  createdAt: string;
  updatedAt: string;
  messageCount: number;
  messages: ChatMessageResponse[];
  pendingActions?: PendingAction[];
};

type ChatMessageResponse = {
  role: string;
  content: string;
  text: string;
};

// ── Error ───────────────────────────────────────────

type ErrorResponse = {
  error: string;
};
```

---

## Step-by-Step Frontend Integration

### 1. Health check (optional)

```ts
const res = await fetch("http://localhost:8080/health");
const { status } = await res.json();
// { status: "ok" }
```

### 2. First message (auto-creates session)

```ts
async function firstMessage(text: string): Promise<AskResponse> {
  const res = await fetch("http://localhost:8080/ask", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ message: text }),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error);
  }
  return res.json();
}
```

Response includes `sessionId` — store it for future messages.

### 3. Follow-up message (reuse session)

```ts
async function followUp(sessionId: string, text: string): Promise<AskResponse> {
  const res = await fetch("http://localhost:8080/ask", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ sessionId, message: text }),
  });
  if (!res.ok) throw new Error((await res.json()).error);
  return res.json();
}
```

### 4. Resolve approval

```ts
async function resolveApproval(
  sessionId: string,
  approvalId: string,
  approved: boolean,
): Promise<AskResponse> {
  const res = await fetch(
    `http://localhost:8080/sessions/${sessionId}/approvals/${approvalId}`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ approved }),
    },
  );
  if (!res.ok) throw new Error((await res.json()).error);
  return res.json();
}
```

### 5. Load session detail (restore on reload)

```ts
async function getSessionDetail(
  sessionId: string,
): Promise<SessionDetailResponse> {
  const res = await fetch(`http://localhost:8080/sessions/${sessionId}`);
  if (!res.ok) throw new Error((await res.json()).error);
  return res.json();
}
```

### 6. List all sessions

```ts
const res = await fetch("http://localhost:8080/sessions");
const sessions: SessionListItem[] = await res.json();
```

---

## Frontend State Model (Recommended)

```ts
type ChatState = {
  sessionId: string | null;
  messages: ChatMessage[];
  tracesByMessageId: Record<string, LoopIteration[]>;
  pendingAction: PendingAction | null;
  loading: boolean;
  error: string | null;
};

type ChatMessage = {
  id: string;
  role: "user" | "assistant";
  text: string;
};
```

### Full sendMessage implementation

```ts
async function sendMessage(state: ChatState, text: string) {
  // Add user message
  state.messages.push({ id: nanoid(), role: "user", text });
  state.loading = true;
  state.error = null;

  const body = state.sessionId
    ? { sessionId: state.sessionId, message: text }
    : { message: text };

  const res = await fetch("http://localhost:8080/ask", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

  const data: AskResponse = await res.json();
  if (!res.ok) {
    state.error = data.error || "Unknown error";
    state.loading = false;
    return;
  }

  // Persist sessionId
  state.sessionId = data.sessionId;
  localStorage.setItem("agentSessionId", data.sessionId);

  // Add assistant message
  const msgId = nanoid();
  state.messages.push({ id: msgId, role: "assistant", text: data.answer });
  state.tracesByMessageId[msgId] = data.trace ?? [];

  // Show approval card if needed
  state.pendingAction = data.pendingAction ?? null;
  state.loading = false;
}
```

---

## Restoring Session After Page Reload

Since sessions are persisted to `data/sessions.json`, the UI can recover the conversation:

```ts
async function restoreSession(sessionId: string): Promise<ChatState> {
  const detail = await getSessionDetail(sessionId);

  const messages: ChatMessage[] = detail.messages
    .filter((m) => m.role !== "system")
    .map((m) => ({
      id: nanoid(),
      role: m.role as "user" | "assistant",
      text: m.content,
    }));

  // If there are pending approvals, restore the first one
  const pendingAction = detail.pendingActions?.length
    ? detail.pendingActions[0]
    : null;

  return {
    sessionId: detail.sessionId,
    messages,
    tracesByMessageId: {},
    pendingAction,
    loading: false,
    error: null,
  };
}
```

---

## Approval Flow: Step-by-Step

### What the UI sees

1. `/ask` returns `stoppedBy: "approval_required"` with `pendingAction`.
2. UI shows an approval card:

```tsx
function ApprovalCard({ action, onApprove, onReject }: Props) {
  return (
    <div className="approval-card">
      <div className="risk-badge">{action.risk}</div>
      <h3>{action.summary}</h3>
      <pre>{action.preview}</pre>
      <button onClick={() => onApprove(action.id, true)}>Approve</button>
      <button onClick={() => onApprove(action.id, false)}>Reject</button>
    </div>
  );
}
```

3. User clicks approve/reject.
4. UI calls `POST /sessions/{sessionId}/approvals/{approvalId}`.
5. Response has **same shape as `/ask`**:

   - `stoppedBy: "success"` → tool ran, agent answered.
   - `stoppedBy: "approval_required"` → agent needs another risky step.
   - `stoppedBy: "error"` → tool failed.

6. If another `pendingAction` is returned, show the next approval card.

---

## Rendering Tool Calls (Trace)

Each `LoopIteration` in `trace` with `outcome: "tool_calls"` should be rendered as a collapsible card:

```tsx
function ToolCallCard({ iteration }: { iteration: LoopIteration }) {
  return (
    <details className="tool-call">
      <summary>
        🔧 {iteration.toolEvents?.map((e) => e.tool).join(", ")}
      </summary>
      {iteration.toolEvents?.map((event) => (
        <div key={event.tool}>
          <h4>{event.tool}</h4>
          <pre>{JSON.stringify(event.args, null, 2)}</pre>
          {event.error ? (
            <div className="error">{event.error}</div>
          ) : (
            <pre className="result">{event.result}</pre>
          )}
        </div>
      ))}
    </details>
  );
}
```

---

## All Available Tools (Reference for UI)

### Read-Only Tools (no approval needed)

| Tool | Action | Risk | Key Parameters |
|---|---|---|---|
| Git | `git_context` | read | `mode`: branch, status, changed_files, diff, branch_diff, log |
| List directory | `list_directory` | read | `path`, `max_entries` |
| Read file | `read_file` | read | `path`, `max_bytes` |
| Find files | `find_files` | read | `pattern` (glob) |
| Search text | `search_text` | read | `query`, `path` |

### Write Tools (approval required)

| Tool | Action | Risk | Key Parameters |
|---|---|---|---|
| Create directory | `create_directory` | write | `path` |
| Write file | `write_file` | write | `path`, `content`, `overwrite?`, `create_parents?` |
| Edit file | `edit_file` | write | `path`, `old_text`, `new_text`, `replace_all?` |

### Command Tool (approval required)

| Tool | Action | Risk | Key Parameters |
|---|---|---|---|
| Run command | `run_command` | exec | `command`, `args`, `cwd`, `timeout_seconds`, `max_bytes` |

Allowlisted commands: `go`, `git` (read-only), `ls`, `pwd`.

---

## Error Handling

All error responses follow this shape:

```json
{
  "error": "session not found"
}
```

Common HTTP status codes:

| Status | Meaning |
|---|---|
| 400 | Invalid request body or missing required field |
| 404 | Session or approval not found |
| 200 | Success |

Check `response.ok` and display `data.error` to the user.

---

## Current Limitations

- `run_command` is intentionally allowlisted; arbitrary shell execution is not supported.
- The LLM planner sometimes returns plain text instead of JSON; the backend handles this as a fallback direct answer.
- Session persistence uses JSON files; concurrent writes from multiple API instances are not safe yet.
