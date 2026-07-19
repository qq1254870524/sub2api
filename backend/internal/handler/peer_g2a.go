package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	peerFailoverHeader    = "X-Peer-Failover"
	peerFailoverHeaderAlt = "X-G2A-Peer-Failover"
)

type peerG2AConfig struct {
	Enabled bool
	BaseURL string
	APIKey  string
	Timeout time.Duration
	Models  []string
}

func loadPeerG2AConfig() peerG2AConfig {
	cfg := peerG2AConfig{
		Enabled: strings.EqualFold(strings.TrimSpace(os.Getenv("GATEWAY_PEER_G2A_ENABLED")), "true") ||
			strings.TrimSpace(os.Getenv("GATEWAY_PEER_G2A_ENABLED")) == "1",
		BaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("GATEWAY_PEER_G2A_BASE_URL")), "/"),
		APIKey:  strings.TrimSpace(os.Getenv("GATEWAY_PEER_G2A_API_KEY")),
		Timeout: 90 * time.Second,
	}
	if v := strings.TrimSpace(os.Getenv("GATEWAY_PEER_G2A_TIMEOUT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Timeout = time.Duration(n) * time.Second
		}
	}
	if v := strings.TrimSpace(os.Getenv("GATEWAY_PEER_G2A_MODELS")); v != "" {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				cfg.Models = append(cfg.Models, strings.ToLower(p))
			}
		}
	}
	return cfg
}

func (c peerG2AConfig) ready() bool {
	return c.Enabled && c.BaseURL != "" && c.APIKey != ""
}

func (c peerG2AConfig) modelAllowed(model string) bool {
	if len(c.Models) == 0 {
		return true
	}
	m := strings.ToLower(strings.TrimSpace(model))
	for _, allow := range c.Models {
		if m == allow || strings.HasPrefix(m, allow) {
			return true
		}
	}
	return false
}

func inboundPeerHop(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	v := c.GetHeader(peerFailoverHeader)
	if v == "" {
		v = c.GetHeader(peerFailoverHeaderAlt)
	}
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func peerBodyPrefix(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}

// tryPeerG2AChatCompletions forwards the original chat body to peer Grok2API.
// Returns true if a response was written to the client.
// Used by both GatewayHandler and OpenAIGatewayHandler (Grok) exhaust paths.
func tryPeerG2AChatCompletions(c *gin.Context, body []byte, streamStarted bool, reason string) bool {
	if streamStarted {
		return false
	}
	if inboundPeerHop(c) {
		return false
	}
	cfg := loadPeerG2AConfig()
	if !cfg.ready() {
		return false
	}
	model := gjson.GetBytes(body, "model").String()
	if model == "" || !cfg.modelAllowed(model) {
		return false
	}

	reqStream := gjson.GetBytes(body, "stream").Bool()
	peerBody := body
	if reqStream {
		var obj map[string]any
		if err := json.Unmarshal(body, &obj); err == nil {
			obj["stream"] = false
			if b, err2 := json.Marshal(obj); err2 == nil {
				peerBody = b
			}
		}
	}

	url := cfg.BaseURL + "/v1/chat/completions"
	ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(peerBody))
	if err != nil {
		logger.FromContext(c.Request.Context()).Warn("peer.g2a.build_request_failed", zap.Error(err))
		return false
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set(peerFailoverHeader, "1")
	req.Header.Set(peerFailoverHeaderAlt, "1")

	logger.FromContext(c.Request.Context()).Warn("peer.g2a.forward_start",
		zap.String("model", model),
		zap.String("reason", reason),
		zap.String("url", url),
	)

	client := &http.Client{Timeout: cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		logger.FromContext(c.Request.Context()).Warn("peer.g2a.forward_error", zap.Error(err))
		return false
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 400 {
		logger.FromContext(c.Request.Context()).Warn("peer.g2a.forward_status",
			zap.Int("status", resp.StatusCode),
			zap.ByteString("body", peerBodyPrefix(raw, 240)),
		)
		return false
	}

	if !reqStream {
		c.Header("X-Peer-Source", "g2a")
		c.Data(http.StatusOK, "application/json", raw)
		return true
	}

	content := ""
	rid := "chatcmpl-peer-g2a"
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err == nil {
		if id, ok := parsed["id"].(string); ok && id != "" {
			rid = id
		}
		if choices, ok := parsed["choices"].([]any); ok && len(choices) > 0 {
			if ch, ok := choices[0].(map[string]any); ok {
				if msg, ok := ch["message"].(map[string]any); ok {
					if s, ok := msg["content"].(string); ok {
						content = s
					}
				}
			}
		}
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Peer-Source", "g2a")
	c.Status(http.StatusOK)
	chunk1, _ := json.Marshal(map[string]any{
		"id": rid, "object": "chat.completion.chunk", "created": time.Now().Unix(),
		"model": model,
		"choices": []map[string]any{{
			"index": 0, "delta": map[string]any{"role": "assistant", "content": content}, "finish_reason": nil,
		}},
	})
	_, _ = c.Writer.Write([]byte("data: " + string(chunk1) + "\n\n"))
	chunk2, _ := json.Marshal(map[string]any{
		"id": rid, "object": "chat.completion.chunk", "created": time.Now().Unix(),
		"model": model,
		"choices": []map[string]any{{
			"index": 0, "delta": map[string]any{}, "finish_reason": "stop",
		}},
	})
	_, _ = c.Writer.Write([]byte("data: " + string(chunk2) + "\n\n"))
	_, _ = c.Writer.Write([]byte("data: [DONE]\n\n"))
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}
	return true
}

// tryPeerG2AOnExhausted attempts peer G2A when local account pool is exhausted.
func tryPeerG2AOnExhausted(c *gin.Context, body []byte, streamStarted bool, lastErr *service.UpstreamFailoverError) bool {
	reason := "accounts_exhausted"
	if lastErr != nil {
		reason = "failover_exhausted"
	}
	return tryPeerG2AChatCompletions(c, body, streamStarted, reason)
}
