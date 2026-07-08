package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSSEParse(t *testing.T) {
	r := strings.NewReader("event: foo\ndata: {\"a\":1}\n\ndata: hello\ndata: world\n\n")
	ch := make(chan sseEvent, 4)
	go readSSE(r, ch)

	e := <-ch
	if e.Event != "foo" || e.Data != `{"a":1}` {
		t.Fatalf("event 1: %+v", e)
	}
	e = <-ch
	if e.Event != "" || e.Data != "hello\nworld" {
		t.Fatalf("event 2: %+v", e)
	}
	if _, ok := <-ch; ok {
		t.Fatalf("channel not closed")
	}
}

func TestModelCatalog(t *testing.T) {
	if len(Catalog) == 0 {
		t.Fatal("empty catalog")
	}
	if _, err := FindModel("anthropic", "claude-sonnet-4-5"); err != nil {
		t.Fatal(err)
	}
	if _, err := FindModel("openai", "gpt-5"); err != nil {
		t.Fatal(err)
	}
	if _, err := FindModel("", "nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestComputeCost(t *testing.T) {
	m, _ := FindModel("anthropic", "claude-sonnet-4-5")
	cost := ComputeCost(m, Usage{InputTokens: 1_000_000, OutputTokens: 1_000_000})
	want := m.PriceInput + m.PriceOutput
	if cost != want {
		t.Fatalf("cost=%v want=%v", cost, want)
	}
}

func TestAnthropicErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"auth","message":"bad key"}}`))
	}))
	defer srv.Close()

	c := NewAnthropic("x", srv.URL)
	_, err := c.Stream(context.Background(), Request{Model: "claude-sonnet-4-5"})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("want 401 err, got %v", err)
	}
}

func TestOpenAIErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"nope"}}`))
	}))
	defer srv.Close()

	c := NewOpenAI("x", srv.URL)
	_, err := c.Stream(context.Background(), Request{Model: "gpt-5"})
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("want 400 err, got %v", err)
	}
}

func TestAnthropicAdaptiveThinking(t *testing.T) {
	c := NewAnthropic("x", "").(*anthropicClient)
	temp := float32(0.7)

	// Opus 4.8 -> adaptive thinking, effort set, no budget, no temperature.
	wire, err := c.buildRequest(Request{
		Model:       "claude-opus-4-8",
		Reasoning:   "high",
		Temperature: &temp,
		Messages:    []Message{{Role: RoleUser, Content: []Content{TextBlock{Text: "hi"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if wire.Thinking == nil || wire.Thinking.Type != "adaptive" {
		t.Fatalf("want adaptive thinking, got %+v", wire.Thinking)
	}
	if wire.Thinking.BudgetTokens != 0 {
		t.Fatalf("adaptive must not send budget_tokens, got %d", wire.Thinking.BudgetTokens)
	}
	if wire.OutputConfig == nil || wire.OutputConfig.Effort != "high" {
		t.Fatalf("want effort=high, got %+v", wire.OutputConfig)
	}
	if wire.Temperature != nil {
		t.Fatalf("adaptive must drop temperature, got %v", *wire.Temperature)
	}

	// maximum -> xhigh effort.
	wire, err = c.buildRequest(Request{
		Model:     "claude-opus-4-8",
		Reasoning: "maximum",
		Messages:  []Message{{Role: RoleUser, Content: []Content{TextBlock{Text: "hi"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if wire.OutputConfig == nil || wire.OutputConfig.Effort != "xhigh" {
		t.Fatalf("want effort=xhigh, got %+v", wire.OutputConfig)
	}

	// Opus 4.5 -> budget-based thinking, no output_config, temperature kept.
	wire, err = c.buildRequest(Request{
		Model:       "claude-opus-4-5",
		Reasoning:   "high",
		Temperature: &temp,
		Messages:    []Message{{Role: RoleUser, Content: []Content{TextBlock{Text: "hi"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if wire.Thinking == nil || wire.Thinking.Type != "enabled" || wire.Thinking.BudgetTokens <= 0 {
		t.Fatalf("want budget thinking, got %+v", wire.Thinking)
	}
	if wire.OutputConfig != nil {
		t.Fatalf("budget models must not send output_config, got %+v", wire.OutputConfig)
	}
	if wire.Temperature == nil || *wire.Temperature != temp {
		t.Fatalf("budget model should keep temperature, got %v", wire.Temperature)
	}
}

func TestAnthropicBuildRequestStripsAssistantImages(t *testing.T) {
	c := NewAnthropic("x", "").(*anthropicClient)
	wire, err := c.buildRequest(Request{
		Model: "claude-sonnet-4-5",
		Messages: []Message{
			{Role: RoleUser, Content: []Content{TextBlock{Text: "make an image"}}},
			{Role: RoleAssistant, Content: []Content{
				TextBlock{Text: "done"},
				ImageBlock{MimeType: "image/png", Data: []byte("png")},
				TextBlock{Text: "Saved image: `zot-gemini-image-x.png`"},
			}},
			{Role: RoleUser, Content: []Content{TextBlock{Text: "hello"}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(wire.Messages) != 3 {
		t.Fatalf("messages=%d", len(wire.Messages))
	}
	assistant := wire.Messages[1]
	if assistant.Role != "assistant" {
		t.Fatalf("role=%q", assistant.Role)
	}
	if len(assistant.Content) != 2 {
		t.Fatalf("assistant content=%+v", assistant.Content)
	}
	for _, b := range assistant.Content {
		if _, ok := b.(anthImageBlock); ok {
			t.Fatalf("assistant image block was not stripped: %+v", assistant.Content)
		}
	}
}

func TestAnthropicStreamHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		write := func(s string) {
			_, _ = w.Write([]byte(s))
			if fl != nil {
				fl.Flush()
			}
		}
		write("event: message_start\ndata: {\"message\":{\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n")
		write("event: content_block_start\ndata: {\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
		write("event: content_block_delta\ndata: {\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hi\"}}\n\n")
		write("event: content_block_stop\ndata: {\"index\":0}\n\n")
		write("event: message_delta\ndata: {\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":5}}\n\n")
		write("event: message_stop\ndata: {}\n\n")
	}))
	defer srv.Close()

	c := NewAnthropic("x", srv.URL)
	evs, err := c.Stream(context.Background(), Request{Model: "claude-sonnet-4-5"})
	if err != nil {
		t.Fatal(err)
	}
	var gotText string
	var done EventDone
	for ev := range evs {
		switch e := ev.(type) {
		case EventTextDelta:
			gotText += e.Delta
		case EventDone:
			done = e
		}
	}
	if gotText != "hi" {
		t.Fatalf("text=%q", gotText)
	}
	if done.Stop != StopEnd {
		t.Fatalf("stop=%v", done.Stop)
	}
}

func TestOpenAICompatAnthropicReasoningEffort(t *testing.T) {
	c := NewOpenRouter("token", "").(*openaiClient)
	wire, err := c.buildRequest(Request{
		Model:     "anthropic/claude-opus-4.8",
		Reasoning: "maximum",
		Messages:  []Message{{Role: RoleUser, Content: []Content{TextBlock{Text: "hi"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if wire.ReasoningEffort != "xhigh" {
		t.Fatalf("want xhigh for adaptive Anthropic model over OpenAI-compatible wire, got %q", wire.ReasoningEffort)
	}

	wire, err = c.buildRequest(Request{
		Model:     "gpt-5.1",
		Reasoning: "maximum",
		Messages:  []Message{{Role: RoleUser, Content: []Content{TextBlock{Text: "hi"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if wire.ReasoningEffort != "high" {
		t.Fatalf("want generic OpenAI-compatible maximum clamped to high, got %q", wire.ReasoningEffort)
	}
}

func TestOpenAIBuildRequestSkipsReasoningOnlyAssistantMessages(t *testing.T) {
	c := NewKimi("token", "").(*openaiClient)
	wire, err := c.buildRequest(Request{
		Model: "kimi-for-coding",
		Messages: []Message{
			{Role: RoleUser, Content: []Content{TextBlock{Text: "first"}}},
			{Role: RoleAssistant, Content: []Content{ReasoningBlock{Summary: "thinking only"}}},
			{Role: RoleUser, Content: []Content{TextBlock{Text: "second"}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, msg := range wire.Messages {
		if msg.Role == "assistant" && msg.Content == nil && len(msg.ToolCalls) == 0 {
			t.Fatalf("message %d is empty assistant: %+v", i, msg)
		}
	}
	if got := len(wire.Messages); got != 2 {
		t.Fatalf("messages=%d want 2 after skipping reasoning-only assistant", got)
	}
}

func TestOpenAIStreamHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		write := func(s string) {
			_, _ = w.Write([]byte(s))
			if fl != nil {
				fl.Flush()
			}
		}
		write("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hel\"},\"finish_reason\":null}]}\n\n")
		write("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}]}\n\n")
		write("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":2}}\n\n")
		write("data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := NewOpenAI("x", srv.URL)
	evs, err := c.Stream(context.Background(), Request{Model: "gpt-5"})
	if err != nil {
		t.Fatal(err)
	}
	var gotText string
	var done EventDone
	for ev := range evs {
		switch e := ev.(type) {
		case EventTextDelta:
			gotText += e.Delta
		case EventDone:
			done = e
		}
	}
	if gotText != "hello" {
		t.Fatalf("text=%q", gotText)
	}
	if done.Stop != StopEnd {
		t.Fatalf("stop=%v", done.Stop)
	}
}
