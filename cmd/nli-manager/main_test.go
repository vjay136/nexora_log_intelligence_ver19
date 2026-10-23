package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIGetSuccessAndHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		http.Error(w, "controller unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	if body, err := apiGet(server.URL + "/ok"); err != nil || body != `{"status":"ok"}` {
		t.Fatalf("apiGet success body=%q err=%v", body, err)
	}
	if _, err := apiGet(server.URL + "/failed"); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("apiGet failure = %v", err)
	}
}

func TestAPIGetRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", (1<<20)+1)))
	}))
	defer server.Close()
	if _, err := apiGet(server.URL); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized response error = %v", err)
	}
}
