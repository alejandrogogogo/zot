package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// ProbeAPIKey verifies that key is valid for provider by making a
// lightweight authenticated request. Returns nil on success.
func ProbeAPIKey(ctx context.Context, provider, key string) error {
	if key == "" {
		return fmt.Errorf("empty key")
	}
	c := &http.Client{Timeout: 15 * time.Second}
	var req *http.Request
	var err error

	switch provider {
	case "anthropic":
		req, err = http.NewRequestWithContext(ctx, "GET", "https://api.anthropic.com/v1/models", nil)
		if err != nil {
			return err
		}
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", "2023-06-01")
	case "openai":
		req, err = http.NewRequestWithContext(ctx, "GET", "https://api.openai.com/v1/models", nil)
		if err != nil {
			return err
		}
		req.Header.Set("authorization", "Bearer "+key)
	case "kimi":
		req, err = http.NewRequestWithContext(ctx, "GET", "https://api.kimi.com/coding/v1/models", nil)
		if err != nil {
			return err
		}
		req.Header.Set("authorization", "Bearer "+key)
	case "deepseek":
		req, err = http.NewRequestWithContext(ctx, "GET", "https://api.deepseek.com/v1/models", nil)
		if err != nil {
			return err
		}
		req.Header.Set("authorization", "Bearer "+key)
	case "google":
		// Google Generative Language: list models with the API key.
		// Accepts the key via x-goog-api-key header (preferred over
		// the ?key= query param so it doesn't show up in proxy logs).
		req, err = http.NewRequestWithContext(ctx, "GET", "https://generativelanguage.googleapis.com/v1beta/models", nil)
		if err != nil {
			return err
		}
		req.Header.Set("x-goog-api-key", key)
	// OpenAI-compatible third parties: a GET /v1/models with bearer auth
	// is enough to validate the key. Branches kept explicit (rather than a
	// generic default) so the URL list is searchable and reviewable.
	case "moonshotai":
		req, err = http.NewRequestWithContext(ctx, "GET", "https://api.moonshot.ai/v1/models", nil)
		if err != nil {
			return err
		}
		req.Header.Set("authorization", "Bearer "+key)
	case "moonshotai-cn":
		req, err = http.NewRequestWithContext(ctx, "GET", "https://api.moonshot.cn/v1/models", nil)
		if err != nil {
			return err
		}
		req.Header.Set("authorization", "Bearer "+key)
	case "groq":
		req, err = http.NewRequestWithContext(ctx, "GET", "https://api.groq.com/openai/v1/models", nil)
		if err != nil {
			return err
		}
		req.Header.Set("authorization", "Bearer "+key)
	case "cerebras":
		req, err = http.NewRequestWithContext(ctx, "GET", "https://api.cerebras.ai/v1/models", nil)
		if err != nil {
			return err
		}
		req.Header.Set("authorization", "Bearer "+key)
	case "xai":
		req, err = http.NewRequestWithContext(ctx, "GET", "https://api.x.ai/v1/models", nil)
		if err != nil {
			return err
		}
		req.Header.Set("authorization", "Bearer "+key)
	case "together":
		req, err = http.NewRequestWithContext(ctx, "GET", "https://api.together.ai/v1/models", nil)
		if err != nil {
			return err
		}
		req.Header.Set("authorization", "Bearer "+key)
	case "openrouter":
		req, err = http.NewRequestWithContext(ctx, "GET", "https://openrouter.ai/api/v1/models", nil)
		if err != nil {
			return err
		}
		req.Header.Set("authorization", "Bearer "+key)
	case "huggingface":
		req, err = http.NewRequestWithContext(ctx, "GET", "https://router.huggingface.co/v1/models", nil)
		if err != nil {
			return err
		}
		req.Header.Set("authorization", "Bearer "+key)
	case "zai":
		req, err = http.NewRequestWithContext(ctx, "GET", "https://api.z.ai/api/coding/paas/v4/models", nil)
		if err != nil {
			return err
		}
		req.Header.Set("authorization", "Bearer "+key)
	case "mistral":
		req, err = http.NewRequestWithContext(ctx, "GET", "https://api.mistral.ai/v1/models", nil)
		if err != nil {
			return err
		}
		req.Header.Set("authorization", "Bearer "+key)
	default:
		return fmt.Errorf("unknown provider %q", provider)
	}

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("probe %s: %w", provider, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("%s rejected the key (http %d)", provider, resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s http %d", provider, resp.StatusCode)
	}
	return nil
}
