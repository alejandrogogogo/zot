package provider

import (
	"strings"
	"testing"
)

// An image-only tool result must not serialize to an empty
// function_call_output (the Responses API may reject it) and a
// following user-message image must serialize as input_image so the
// model actually receives the bytes.
func TestCodexImageToolResultMirror(t *testing.T) {
	c := NewOpenAICodex("token", "acct", "").(*codexClient)

	wire, err := c.buildRequest(Request{
		Model: "gpt-5.5",
		Messages: []Message{
			{Role: RoleUser, Content: []Content{TextBlock{Text: "look at this"}}},
			{Role: RoleAssistant, Content: []Content{
				ToolCallBlock{ID: "call_1", Name: "read", Arguments: []byte(`{"path":"x.png"}`)},
			}},
			{Role: RoleTool, Content: []Content{
				ToolResultBlock{CallID: "call_1", Content: []Content{
					ImageBlock{MimeType: "image/png", Data: []byte("png-bytes")},
				}},
			}},
			// The agent loop appends this mirror after an image tool result.
			{Role: RoleUser, Content: []Content{
				TextBlock{Text: "Tool output included the following image content:"},
				ImageBlock{MimeType: "image/png", Data: []byte("png-bytes")},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var sawFnOutput, sawInputImage bool
	for _, item := range wire.Input {
		switch v := item.(type) {
		case codexFunctionCallOutput:
			sawFnOutput = true
			if strings.TrimSpace(v.Output) == "" {
				t.Fatalf("image-only tool result produced empty function_call_output")
			}
			if !strings.Contains(strings.ToLower(v.Output), "image") {
				t.Fatalf("placeholder should mention image, got %q", v.Output)
			}
		case codexInputMessage:
			for _, ct := range v.Content {
				if img, ok := ct.(codexInputImage); ok && img.Type == "input_image" {
					sawInputImage = true
				}
			}
		}
	}
	if !sawFnOutput {
		t.Fatalf("no function_call_output emitted")
	}
	if !sawInputImage {
		t.Fatalf("mirrored user image was not serialized as input_image")
	}
}
