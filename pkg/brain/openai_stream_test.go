package brain

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// kinfer answers a stream request that carries tools with one plain JSON
// body — it buffers the reply so a tool call never arrives in fragments.
// Read as SSE that body is a line without "data: ", and the turn used to
// end with no text and no tool calls.
func TestOpenAIBrain_StreamAnsweredWithPlainJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","index":0,"message":{"content":"checking","role":"assistant","tool_calls":[{"function":{"arguments":"{\"location\":\"Tokyo\"}","name":"weather"},"id":"call_1","type":"function"}]}}],"object":"chat.completion","usage":{"prompt_tokens":12,"completion_tokens":5}}`))
	}))
	defer srv.Close()

	var streamed string
	b := NewOpenAIBrain(srv.URL, "m", "", 0)
	res, err := b.Chat(context.Background(), []Message{{Role: RoleUser, Content: "weather in Tokyo?"}}, nil,
		func(chunk string, thinking bool) error { streamed += chunk; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ToolCalls) != 1 {
		t.Fatalf("tool calls = %+v, want one", res.ToolCalls)
	}
	if tc := res.ToolCalls[0]; tc.ID != "call_1" || tc.Function.Name != "weather" || tc.Function.Arguments != `{"location":"Tokyo"}` {
		t.Errorf("tool call = %+v", tc)
	}
	if res.Content != "checking" || streamed != "checking" {
		t.Errorf("content = %q, streamed = %q, want both %q", res.Content, streamed, "checking")
	}
	if res.Usage.InputTokens != 12 || res.Usage.OutputTokens != 5 {
		t.Errorf("usage = %+v", res.Usage)
	}
}

func TestOpenAIBrain_StreamStillReadsSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n" +
			"data: [DONE]\n\n"))
	}))
	defer srv.Close()

	var streamed string
	b := NewOpenAIBrain(srv.URL, "m", "", 0)
	res, err := b.Chat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil,
		func(chunk string, thinking bool) error { streamed += chunk; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if res.Content != "hello" || streamed != "hello" {
		t.Errorf("content = %q, streamed = %q", res.Content, streamed)
	}
}

// kinfer refuses tools for a model whose chat template has no tool-call
// convention, and says so as {"error":"…"}. That sentence is the fix;
// "returned status 400" is a riddle.
func TestOpenAIBrain_FlatErrorBodyIsReported(t *testing.T) {
	const why = "this model's chat template does not use the <tool_call> convention, so it cannot answer with tool calls"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"` + why + `"}`))
	}))
	defer srv.Close()

	b := NewOpenAIBrain(srv.URL, "m", "", 0)
	_, err := b.Chat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), why) {
		t.Fatalf("err = %v, want it to carry %q", err, why)
	}
}
