package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientChatUsesContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("cancelled context should prevent the request from reaching the server")
	}))
	defer server.Close()

	client := NewClientWithTimeout("test-key", server.URL, "test-model", 0, 16, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Chat(ctx, []Message{{Role: "user", Content: "hello"}}, nil)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context cancellation error, got %v", err)
	}
}
