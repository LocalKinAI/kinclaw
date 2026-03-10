package brain

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

func retryDo(client *http.Client, newReq func() (*http.Request, error)) (*http.Response, error) {
	const maxAttempts = 3
	var resp *http.Response
	for attempt := 0; attempt < maxAttempts; attempt++ {
		req, err := newReq()
		if err != nil {
			return nil, err
		}
		resp, err = client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 429 && resp.StatusCode < 500 {
			return resp, nil
		}
		resp.Body.Close()
		if attempt < maxAttempts-1 {
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}
	}
	return resp, nil
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (tc ToolCall) ParseArguments() (map[string]string, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &raw); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out, nil
}

type ChatResult struct {
	Content   string
	ToolCalls []ToolCall
}

type StreamFunc func(chunk string, thinking bool) error
type Brain interface {
	Chat(ctx context.Context, messages []Message, tools []json.RawMessage, onChunk StreamFunc) (*ChatResult, error)
}

type claudeBrain struct {
	baseURL, model, apiKey string
	temperature            float64
	maxTokens              int
	client                 *http.Client
}

func NewClaudeBrain(endpoint, model, apiKey string, temperature float64) Brain {
	if endpoint == "" {
		endpoint = "https://api.anthropic.com"
	}
	return &claudeBrain{
		baseURL: strings.TrimRight(endpoint, "/"), model: model, apiKey: apiKey,
		temperature: temperature, maxTokens: 4096,
		client: &http.Client{Timeout: 5 * time.Minute},
	}
}

type cReq struct {
	Model       string            `json:"model"`
	MaxTokens   int               `json:"max_tokens"`
	Temperature float64           `json:"temperature,omitempty"`
	System      string            `json:"system,omitempty"`
	Messages    []cMsg            `json:"messages"`
	Stream      bool              `json:"stream"`
	Tools       []json.RawMessage `json:"tools,omitempty"`
}
type cMsg struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}
type cBlock struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Input     interface{} `json:"input,omitempty"`
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   string      `json:"content,omitempty"`
}
type cResp struct {
	Content []cBlock `json:"content"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error"`
}
type cStreamEvent struct {
	Type         string  `json:"type"`
	Index        int     `json:"index,omitempty"`
	Delta        *cDelta `json:"delta,omitempty"`
	ContentBlock *cBlock `json:"content_block,omitempty"`
	Error        *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}
type cDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

func (b *claudeBrain) Chat(ctx context.Context, messages []Message, tools []json.RawMessage, onChunk StreamFunc) (*ChatResult, error) {
	var systemPrompt string
	var convMsgs []cMsg
	for _, m := range messages {
		switch {
		case m.Role == RoleSystem:
			systemPrompt = m.Content
		case m.Role == RoleAssistant && len(m.ToolCalls) > 0:
			var blocks []cBlock
			if m.Content != "" {
				blocks = append(blocks, cBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				var input interface{}
				json.Unmarshal([]byte(tc.Function.Arguments), &input)
				blocks = append(blocks, cBlock{
					Type: "tool_use", ID: tc.ID, Name: tc.Function.Name, Input: input,
				})
			}
			convMsgs = append(convMsgs, cMsg{Role: "assistant", Content: blocks})
		case m.Role == RoleTool:
			convMsgs = append(convMsgs, cMsg{
				Role: "user",
				Content: []cBlock{{
					Type: "tool_result", ToolUseID: m.ToolCallID, Content: m.Content,
				}},
			})
		default:
			convMsgs = append(convMsgs, cMsg{Role: m.Role, Content: m.Content})
		}
	}
	claudeTools := convertToolsToClaude(tools)
	reqBody := cReq{
		Model: b.model, MaxTokens: b.maxTokens, Temperature: b.temperature,
		System: systemPrompt, Messages: convMsgs, Stream: onChunk != nil, Tools: claudeTools,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}
	resp, err := retryDo(b.client, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL+"/v1/messages", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("anthropic-version", "2023-06-01")
		if strings.HasPrefix(b.apiKey, "sk-ant-oat") {
			req.Header.Set("Authorization", "Bearer "+b.apiKey)
			req.Header.Set("anthropic-beta", "oauth-2025-04-20")
		} else {
			req.Header.Set("x-api-key", b.apiKey)
		}
		return req, nil
	})
	if err != nil {
		return nil, fmt.Errorf("Claude API request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var errResp cResp
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != nil {
			return nil, fmt.Errorf("Claude API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("Claude API returned status %d", resp.StatusCode)
	}
	if onChunk == nil {
		var chatResp cResp
		if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}
		result := &ChatResult{}
		var textParts []string
		for _, block := range chatResp.Content {
			switch block.Type {
			case "text":
				textParts = append(textParts, block.Text)
			case "tool_use":
				args, _ := json.Marshal(block.Input)
				result.ToolCalls = append(result.ToolCalls, ToolCall{
					ID: block.ID, Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: block.Name, Arguments: string(args)},
				})
			}
		}
		result.Content = strings.Join(textParts, "")
		return result, nil
	}
	var full strings.Builder
	var streamToolCalls []ToolCall
	type toolBlock struct {
		id, name  string
		inputJSON strings.Builder
	}
	activeTools := make(map[int]*toolBlock)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var event cStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if event.Error != nil {
			return &ChatResult{Content: full.String()}, fmt.Errorf("Claude stream error: %s", event.Error.Message)
		}
		switch event.Type {
		case "content_block_start":
			if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
				activeTools[event.Index] = &toolBlock{
					id: event.ContentBlock.ID, name: event.ContentBlock.Name,
				}
			}
		case "content_block_delta":
			if event.Delta == nil {
				continue
			}
			switch event.Delta.Type {
			case "text_delta":
				if event.Delta.Text != "" {
					full.WriteString(event.Delta.Text)
					if err := onChunk(event.Delta.Text, false); err != nil {
						return &ChatResult{Content: full.String()}, err
					}
				}
			case "input_json_delta":
				if tb, ok := activeTools[event.Index]; ok {
					tb.inputJSON.WriteString(event.Delta.PartialJSON)
				}
			}
		case "content_block_stop":
			if tb, ok := activeTools[event.Index]; ok {
				args := tb.inputJSON.String()
				if args == "" {
					args = "{}"
				}
				streamToolCalls = append(streamToolCalls, ToolCall{
					ID: tb.id, Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: tb.name, Arguments: args},
				})
				delete(activeTools, event.Index)
			}
		}
	}
	return &ChatResult{Content: full.String(), ToolCalls: streamToolCalls}, nil
}

func convertToolsToClaude(tools []json.RawMessage) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(tools))
	for _, raw := range tools {
		var openai struct {
			Type     string `json:"type"`
			Function struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				Parameters  json.RawMessage `json:"parameters"`
			} `json:"function"`
		}
		if err := json.Unmarshal(raw, &openai); err != nil || openai.Type != "function" {
			out = append(out, raw)
			continue
		}
		claude := struct {
			Type        string          `json:"type"`
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"input_schema"`
		}{
			Type: "custom", Name: openai.Function.Name,
			Description: openai.Function.Description, InputSchema: openai.Function.Parameters,
		}
		b, _ := json.Marshal(claude)
		out = append(out, b)
	}
	return out
}

type openAIBrain struct {
	baseURL, model, apiKey string
	temperature            float64
	maxTokens              int
	client                 *http.Client
}

func NewOpenAIBrain(endpoint, model, apiKey string, temperature float64) Brain {
	if endpoint == "" {
		endpoint = "https://api.openai.com"
	}
	return &openAIBrain{
		baseURL: strings.TrimRight(endpoint, "/"), model: model, apiKey: apiKey,
		temperature: temperature, maxTokens: 4096,
		client: &http.Client{Timeout: 5 * time.Minute},
	}
}

type oaiReq struct {
	Model       string            `json:"model"`
	Messages    []oaiMsg          `json:"messages"`
	Temperature float64           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Stream      bool              `json:"stream"`
	Tools       []json.RawMessage `json:"tools,omitempty"`
}
type oaiMsg struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}
type oaiResp struct {
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message"`
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (b *openAIBrain) Chat(ctx context.Context, messages []Message, tools []json.RawMessage, onChunk StreamFunc) (*ChatResult, error) {
	var oaiMsgs []oaiMsg
	for _, m := range messages {
		msg := oaiMsg{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
		if len(m.ToolCalls) > 0 {
			msg.ToolCalls = m.ToolCalls
		}
		oaiMsgs = append(oaiMsgs, msg)
	}
	reqBody := oaiReq{
		Model: b.model, Messages: oaiMsgs, Temperature: b.temperature,
		MaxTokens: b.maxTokens, Stream: onChunk != nil, Tools: tools,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	resp, err := retryDo(b.client, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL+"/v1/chat/completions", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+b.apiKey)
		return req, nil
	})
	if err != nil {
		return nil, fmt.Errorf("OpenAI API request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var errResp oaiResp
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != nil {
			return nil, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("OpenAI API returned status %d", resp.StatusCode)
	}
	if onChunk == nil {
		var chatResp oaiResp
		if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}
		if len(chatResp.Choices) == 0 {
			return &ChatResult{}, nil
		}
		choice := chatResp.Choices[0].Message
		return &ChatResult{Content: choice.Content, ToolCalls: choice.ToolCalls}, nil
	}
	var full strings.Builder
	toolCallMap := make(map[int]*ToolCall)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var event oaiResp
		if err := json.Unmarshal([]byte(data), &event); err != nil || len(event.Choices) == 0 {
			continue
		}
		delta := event.Choices[0].Delta
		if delta.Content != "" {
			full.WriteString(delta.Content)
			if err := onChunk(delta.Content, false); err != nil {
				return &ChatResult{Content: full.String()}, err
			}
		}
		for _, tc := range delta.ToolCalls {
			if tc.ID != "" {
				toolCallMap[tc.Index] = &ToolCall{ID: tc.ID, Type: "function"}
				toolCallMap[tc.Index].Function.Name = tc.Function.Name
			}
			if existing, ok := toolCallMap[tc.Index]; ok {
				existing.Function.Arguments += tc.Function.Arguments
			}
		}
	}
	var streamToolCalls []ToolCall
	for i := 0; i < len(toolCallMap); i++ {
		if tc, ok := toolCallMap[i]; ok {
			streamToolCalls = append(streamToolCalls, *tc)
		}
	}
	return &ChatResult{Content: full.String(), ToolCalls: streamToolCalls}, nil
}

func NewBrain(provider, endpoint, model, apiKey string, temperature float64) Brain {
	switch provider {
	case "openai", "ollama":
		return NewOpenAIBrain(endpoint, model, apiKey, temperature)
	default:
		return NewClaudeBrain(endpoint, model, apiKey, temperature)
	}
}
