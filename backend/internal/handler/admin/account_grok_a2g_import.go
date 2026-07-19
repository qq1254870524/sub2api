package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GrokA2GImportRequest imports Grok2API (G2A) account-pool export into Sub2API.
// Supported payloads:
//   - plain text: one SSO token per line
//   - Grok2API JSON: { "basic": ["token"|{token,tags}], "super": [...], ... }
//   - JSON array of tokens / objects with token|sso|sso_token
//   - { "tokens": [...] } / { "content": "..." }
// Never overwrites existing accounts that already carry the same SSO.
type GrokA2GImportRequest struct {
	Content            string         `json:"content"`
	Contents           []string       `json:"contents"`
	Tokens             []string       `json:"tokens"`
	SSOTokens          []string       `json:"sso_tokens"`
	Name               string         `json:"name"`
	Notes              *string        `json:"notes"`
	ProxyID            *int64         `json:"proxy_id"`
	GroupIDs           []int64        `json:"group_ids"`
	Credentials        map[string]any `json:"credentials"`
	Extra              map[string]any `json:"extra"`
	Concurrency        int            `json:"concurrency"`
	LoadFactor         *int           `json:"load_factor"`
	Priority           int            `json:"priority"`
	RateMultiplier     *float64       `json:"rate_multiplier"`
	ExpiresAt          *int64         `json:"expires_at"`
	AutoPauseOnExpired *bool          `json:"auto_pause_on_expired"`
}

type GrokA2GImportResult struct {
	Total   int                    `json:"total"`
	Created int                    `json:"created"`
	Skipped int                    `json:"skipped"`
	Failed  int                    `json:"failed"`
	Items   []GrokA2GImportItem    `json:"items,omitempty"`
	Errors  []GrokA2GImportMessage `json:"errors,omitempty"`
}

type GrokA2GImportItem struct {
	Index     int          `json:"index"`
	Name      string       `json:"name,omitempty"`
	Email     string       `json:"email,omitempty"`
	SSOMasked string       `json:"sso_masked,omitempty"`
	Action    string       `json:"action"` // created | skipped | failed
	AccountID int64        `json:"account_id,omitempty"`
	Message   string       `json:"message,omitempty"`
	Account   *dto.Account `json:"account,omitempty"`
}

type GrokA2GImportMessage struct {
	Index   int    `json:"index,omitempty"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

// ImportA2G imports Grok2API pool SSO tokens into Sub2API Grok OAuth accounts.
// POST /api/v1/admin/accounts/import/a2g
func (h *GrokOAuthHandler) ImportA2G(c *gin.Context) {
	var req GrokA2GImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	tokens, parseErrs := parseGrokA2GTokens(req)
	if len(tokens) == 0 {
		msg := "请提供 Grok2API 导出内容（txt 每行一个 SSO，或 G2A JSON {pool:[tokens]}）"
		if len(parseErrs) > 0 {
			msg = msg + "；" + strings.Join(parseErrs, "; ")
		}
		response.BadRequest(c, msg)
		return
	}

	ctx := c.Request.Context()
	existingSSO, err := h.listExistingGrokSSOSet(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	result := GrokA2GImportResult{
		Total: len(tokens),
		Items: make([]GrokA2GImportItem, 0, len(tokens)),
	}

	// Pre-filter duplicates (existing pool + batch-local) so we never overwrite.
	toImport := make([]string, 0, len(tokens))
	for i, token := range tokens {
		masked := maskSSOToken(token)
		if _, exists := existingSSO[token]; exists {
			result.Skipped++
			result.Items = append(result.Items, GrokA2GImportItem{
				Index:     i + 1,
				SSOMasked: masked,
				Action:    "skipped",
				Message:   "SSO already exists in Sub2API; not overwritten",
			})
			continue
		}
		// mark as reserved for batch-local dedupe
		existingSSO[token] = struct{}{}
		toImport = append(toImport, token)
	}

	if len(toImport) == 0 {
		response.Success(c, result)
		return
	}

	ssoReq := GrokSSOToOAuthRequest{
		SSOTokens:          toImport,
		Name:               req.Name,
		Notes:              req.Notes,
		ProxyID:            req.ProxyID,
		GroupIDs:           append([]int64(nil), req.GroupIDs...),
		Credentials:        cloneGrokSSOMap(req.Credentials),
		Extra:              cloneGrokSSOMap(req.Extra),
		Concurrency:        req.Concurrency,
		LoadFactor:         req.LoadFactor,
		Priority:           req.Priority,
		RateMultiplier:     req.RateMultiplier,
		ExpiresAt:          req.ExpiresAt,
		AutoPauseOnExpired: req.AutoPauseOnExpired,
	}

	workerCount := grokSSOImportConcurrency
	if len(toImport) < workerCount {
		workerCount = len(toImport)
	}
	jobs := make(chan grokSSOImportJob)
	items := make([]grokSSOImportWorkerResult, len(toImport))
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				items[job.index] = h.safeCreateAccountFromSSOToken(ctx, ssoReq, job.token, job.index+1, len(toImport))
			}
		}()
	}
	for i, token := range toImport {
		jobs <- grokSSOImportJob{index: i, token: token}
	}
	close(jobs)
	wg.Wait()

	// Map worker results; index offset is relative to toImport, remap to original token order.
	importIndexByToken := make(map[string]int, len(toImport))
	for i, token := range toImport {
		importIndexByToken[token] = i
	}
	// rebuild items ordered by original tokens list for stable UX
	result.Items = result.Items[:0]
	for i, token := range tokens {
		masked := maskSSOToken(token)
		// was skipped earlier?
		if wi, ok := importIndexByToken[token]; ok {
			wr := items[wi]
			if wr.created {
				result.Created++
				item := GrokA2GImportItem{
					Index:     i + 1,
					Name:      wr.item.Name,
					Email:     wr.item.Email,
					SSOMasked: masked,
					Action:    "created",
					Account:   wr.item.Account,
				}
				if wr.item.Account != nil {
					item.AccountID = wr.item.Account.ID
				}
				result.Items = append(result.Items, item)
			} else {
				result.Failed++
				msg := wr.item.Error
				if msg == "" {
					msg = "import failed"
				}
				result.Items = append(result.Items, GrokA2GImportItem{
					Index:     i + 1,
					Name:      wr.item.Name,
					Email:     wr.item.Email,
					SSOMasked: masked,
					Action:    "failed",
					Message:   msg,
				})
				result.Errors = append(result.Errors, GrokA2GImportMessage{
					Index:   i + 1,
					Name:    wr.item.Name,
					Message: msg,
				})
				// free reservation so a later retry can attempt again
				delete(existingSSO, token)
			}
			continue
		}
		result.Items = append(result.Items, GrokA2GImportItem{
			Index:     i + 1,
			SSOMasked: masked,
			Action:    "skipped",
			Message:   "SSO already exists in Sub2API; not overwritten",
		})
	}

	slog.Info("grok_a2g_import_done",
		"total", result.Total,
		"created", result.Created,
		"skipped", result.Skipped,
		"failed", result.Failed,
	)
	response.Success(c, result)
}

func (h *GrokOAuthHandler) listExistingGrokSSOSet(ctx context.Context) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	page := 1
	pageSize := dataPageCap
	for {
		items, total, err := h.adminService.ListAccounts(ctx, page, pageSize, service.PlatformGrok, "", "", "", 0, "", "created_at", "desc")
		if err != nil {
			return nil, err
		}
		for i := range items {
			for _, sso := range extractAccountSSOTokens(items[i]) {
				out[sso] = struct{}{}
			}
		}
		if len(items) == 0 || page*pageSize >= int(total) {
			break
		}
		page++
	}
	return out, nil
}

func extractAccountSSOTokens(account service.Account) []string {
	candidates := []string{
		account.GetCredential("sso"),
		account.GetCredential("sso_token"),
		account.GetCredential("ssoToken"),
	}
	result := make([]string, 0, 1)
	seen := make(map[string]struct{})
	for _, raw := range candidates {
		token := xai.NormalizeSSOToken(raw)
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}
	return result
}

func parseGrokA2GTokens(req GrokA2GImportRequest) ([]string, []string) {
	rawChunks := make([]string, 0, 4)
	if strings.TrimSpace(req.Content) != "" {
		rawChunks = append(rawChunks, req.Content)
	}
	for _, c := range req.Contents {
		if strings.TrimSpace(c) != "" {
			rawChunks = append(rawChunks, c)
		}
	}

	tokens := make([]string, 0, 64)
	seen := make(map[string]struct{})
	var errs []string

	appendToken := func(raw string) {
		token := xai.NormalizeSSOToken(raw)
		if token == "" {
			return
		}
		if _, ok := seen[token]; ok {
			return
		}
		seen[token] = struct{}{}
		tokens = append(tokens, token)
	}

	for _, t := range req.Tokens {
		appendToken(t)
	}
	for _, t := range req.SSOTokens {
		appendToken(t)
	}

	for _, chunk := range rawChunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		// Try JSON first
		if strings.HasPrefix(chunk, "{") || strings.HasPrefix(chunk, "[") {
			extracted, err := extractTokensFromA2GJSON(chunk)
			if err != nil {
				errs = append(errs, err.Error())
				// fall through to line parse for resilience
			} else {
				for _, t := range extracted {
					appendToken(t)
				}
				continue
			}
		}
		// TXT / multi-line paste
		for _, line := range strings.Split(strings.NewReplacer("\r", "\n", ",", "\n").Replace(chunk), "\n") {
			appendToken(line)
		}
	}
	return tokens, errs
}

func extractTokensFromA2GJSON(raw string) ([]string, error) {
	var anyVal any
	if err := json.Unmarshal([]byte(raw), &anyVal); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	out := make([]string, 0, 32)
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				out = append(out, s)
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		case map[string]any:
			// Prefer explicit token/sso fields on objects
			found := false
			for _, key := range []string{"token", "sso", "sso_token", "ssoToken", "sso_cookie"} {
				if val, ok := t[key]; ok {
					if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
						out = append(out, s)
						found = true
						break
					}
				}
			}
			if found {
				return
			}
			// Grok2API pool map: {basic:[...], super:[...]} — walk array values only
			// Also accept nested Sub2API-like {accounts:[{credentials:{sso}}]}
			if accounts, ok := t["accounts"].([]any); ok {
				for _, acc := range accounts {
					walk(acc)
				}
				return
			}
			if creds, ok := t["credentials"].(map[string]any); ok {
				walk(creds)
				return
			}
			// Walk pool arrays / tokens arrays
			if tokens, ok := t["tokens"].([]any); ok {
				walk(tokens)
				return
			}
			if ssoTokens, ok := t["sso_tokens"].([]any); ok {
				walk(ssoTokens)
				return
			}
			// Heuristic: known pool names or any array values
			for _, key := range []string{"basic", "super", "heavy", "console", "items"} {
				if arr, ok := t[key].([]any); ok {
					walk(arr)
					found = true
				}
			}
			if found {
				return
			}
			// Fallback: walk all values that are arrays or objects with token-ish keys
			for _, val := range t {
				switch val.(type) {
				case []any, map[string]any:
					walk(val)
				}
			}
		}
	}
	walk(anyVal)
	if len(out) == 0 {
		return nil, fmt.Errorf("JSON contained no SSO tokens")
	}
	return out, nil
}

func maskSSOToken(token string) string {
	if len(token) <= 16 {
		return token
	}
	return token[:8] + "..." + token[len(token)-8:]
}

func (h *GrokOAuthHandler) findGrokAccountBySSO(ctx context.Context, sso string) (*service.Account, error) {
	sso = xai.NormalizeSSOToken(sso)
	if sso == "" {
		return nil, nil
	}
	page := 1
	pageSize := dataPageCap
	for {
		items, total, err := h.adminService.ListAccounts(ctx, page, pageSize, service.PlatformGrok, "", "", "", 0, "", "created_at", "desc")
		if err != nil {
			return nil, err
		}
		for i := range items {
			for _, existing := range extractAccountSSOTokens(items[i]) {
				if existing == sso {
					acc := items[i]
					return &acc, nil
				}
			}
		}
		if len(items) == 0 || page*pageSize >= int(total) {
			break
		}
		page++
	}
	return nil, nil
}
