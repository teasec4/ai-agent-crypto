package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ai-agent/internal/session"
)

func TestWriterHandoffEndpoints(t *testing.T) {
	store := session.NewStore()
	state := store.Create()
	mux := http.NewServeMux()
	NewAgentHandler(nil, store).RegisterRoutes(mux)

	first := postJSON[SessionConnectResponse](t, mux, "/sessions/"+state.ID+"/connect", `{"clientId":"client-a"}`)
	if first.Role != session.RoleWriter {
		t.Fatalf("expected first client writer role, got %q", first.Role)
	}

	second := postJSON[SessionConnectResponse](t, mux, "/sessions/"+state.ID+"/connect", `{"clientId":"client-b"}`)
	if second.Role != session.RoleViewer {
		t.Fatalf("expected second client viewer role, got %q", second.Role)
	}

	requested := postJSON[session.ClientAccess](t, mux, "/sessions/"+state.ID+"/writer/request", `{"clientId":"client-b"}`)
	if requested.PendingWriterRequest == nil {
		t.Fatal("expected pending writer request")
	}

	approved := postJSON[session.ClientAccess](t, mux, "/sessions/"+state.ID+"/writer/approve", `{"clientId":"client-a","requestId":"`+requested.PendingWriterRequest.ID+`"}`)
	if approved.WriterClientID != "client-b" {
		t.Fatalf("expected client-b writer after approval, got %q", approved.WriterClientID)
	}
}

func postJSON[T any](t *testing.T, mux http.Handler, path, body string) T {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("POST %s returned %d: %s", path, rec.Code, rec.Body.String())
	}

	var result T
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return result
}
