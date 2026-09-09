package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlePermissionMode(t *testing.T) {
	srv := New("127.0.0.1:0", nil, func(context.Context, string) {})
	var got []string
	current := "ask"
	srv.SetPermissionModeHandler(func(mode string) string {
		got = append(got, mode)
		if mode == "auto" || mode == "ask" {
			current = mode
		}
		return current
	})

	post := func(body string) (int, string) {
		rr := httptest.NewRecorder()
		srv.handlePermissionMode(rr, httptest.NewRequest(
			http.MethodPost, "/api/permission_mode", strings.NewReader(body)))
		var reply struct {
			Mode string `json:"mode"`
		}
		_ = json.Unmarshal(rr.Body.Bytes(), &reply)
		return rr.Code, reply.Mode
	}

	if code, mode := post(`{"mode":"auto"}`); code != http.StatusOK || mode != "auto" {
		t.Errorf("auto: got %d %q", code, mode)
	}
	// An unknown value must not silently open the gate: the handler
	// reports whatever is actually in force, which is the old value.
	if code, mode := post(`{"mode":"yolo"}`); code != http.StatusOK || mode != "auto" {
		t.Errorf("unknown value: got %d %q, want the mode still in force", code, mode)
	}
	if code, mode := post(`{"mode":"ask"}`); code != http.StatusOK || mode != "ask" {
		t.Errorf("ask: got %d %q", code, mode)
	}
	if len(got) != 3 {
		t.Errorf("handler called %d times, want 3", len(got))
	}

	rr := httptest.NewRecorder()
	srv.handlePermissionMode(rr, httptest.NewRequest(http.MethodGet, "/api/permission_mode", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should be refused, got %d", rr.Code)
	}
}

func TestPermissionModeNotWired(t *testing.T) {
	srv := New("127.0.0.1:0", nil, func(context.Context, string) {})
	rr := httptest.NewRecorder()
	srv.handlePermissionMode(rr, httptest.NewRequest(
		http.MethodPost, "/api/permission_mode", strings.NewReader(`{"mode":"auto"}`)))
	if rr.Code != http.StatusNotImplemented {
		t.Errorf("unwired handler should 501, got %d", rr.Code)
	}
}
