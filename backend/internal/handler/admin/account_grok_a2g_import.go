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
// Never overwrites existing accounts that already carry the same SSO or email.
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

// GrokG2ASSOExportItem is one Grok account row exposed for G2A reverse import.
type GrokG2ASSOExportItem struct {
	AccountID int64  `json:"account_id"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty"`
	SSO       string `json:"sso,omitempty"`
	SSOMasked string `json:"sso_masked,omitempty"`
	HasSSO    bool   `json:"has_sso"`
	Status    string `json:"status,omitempty"`
}

// GrokG2ASSOExportResult is a lightweight SSO export for Grok2API bridge import.
// Only admin auth is required (no step-up). Sensitive values are returned because
// the caller is already an authenticated admin performing pool bridging.
type GrokG2ASSOExportResult struct {
	Platform   string                 `json:"platform"`
	Total      int                    `json:"total"`
	WithSSO    int                    `json:"with_sso"`
	WithoutSSO int                    `json:"without_sso"`
	Tokens     []string               `json:"tokens"`
	Items      []GrokG2ASSOExportItem `json:"items,omitempty"`
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
		msg := "请提供 Grok2API SSO（浏览器直连 tokens、txt 每行一个 SSO，或 G2A JSON {pool:[tokens]}）"
		if len(parseErrs) > 0 {
			msg = msg + "；" + strings.Join(parseErrs, "; ")
		}
		response.BadRequest(c, msg)
		return
	}

	ctx := c.Request.Context()
	existingSSO, _, err := h.listExistingGrokIdentitySets(ctx)
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
			if wr.skipped {
				result.Skipped++
				result.Items = append(result.Items, GrokA2GImportItem{
					Index:     i + 1,
					Name:      wr.item.Name,
					Email:     wr.item.Email,
					SSOMasked: masked,
					Action:    "skipped",
					Message:   firstNonEmptyA2G(wr.item.Error, "email already exists in Sub2API; not overwritten"),
					AccountID: accountIDFromDTO(wr.item.Account),
					Account:   wr.item.Account,
				})
			} else if wr.created {
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

// ExportG2ASSO exports Grok account SSO tokens for Grok2API reverse import.
// GET /api/v1/admin/accounts/export/g2a-sso
func (h *GrokOAuthHandler) ExportG2ASSO(c *gin.Context) {
	ctx := c.Request.Context()
	page := 1
	pageSize := dataPageCap
	result := GrokG2ASSOExportResult{
		Platform: service.PlatformGrok,
		Tokens:   make([]string, 0, 64),
		Items:    make([]GrokG2ASSOExportItem, 0, 64),
	}
	seenSSO := make(map[string]struct{})

	for {
		items, total, err := h.adminService.ListAccounts(ctx, page, pageSize, service.PlatformGrok, "", "", "", 0, "", "created_at", "desc")
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		for i := range items {
			acc := items[i]
			email := firstNonEmptyA2G(
				normalizeGrokEmail(acc.GetCredential("email")),
				normalizeGrokEmail(acc.GetCredential("user_email")),
			)
			ssos := extractAccountSSOTokens(acc)
			if len(ssos) == 0 {
				result.WithoutSSO++
				result.Items = append(result.Items, GrokG2ASSOExportItem{
					AccountID: acc.ID,
					Name:      acc.Name,
					Email:     email,
					HasSSO:    false,
					Status:    acc.Status,
				})
				result.Total++
				continue
			}
			for _, sso := range ssos {
				if _, ok := seenSSO[sso]; ok {
					continue
				}
				seenSSO[sso] = struct{}{}
				result.WithSSO++
				result.Total++
				result.Tokens = append(result.Tokens, sso)
				result.Items = append(result.Items, GrokG2ASSOExportItem{
					AccountID: acc.ID,
					Name:      acc.Name,
					Email:     email,
					SSO:       sso,
					SSOMasked: maskSSOToken(sso),
					HasSSO:    true,
					Status:    acc.Status,
				})
			}
		}
		if len(items) == 0 || page*pageSize >= int(total) {
			break
		}
		page++
	}

	slog.Info("grok_g2a_sso_export",
		"total", result.Total,
		"with_sso", result.WithSSO,
		"without_sso", result.WithoutSSO,
	)
	response.Success(c, result)
}

func (h *GrokOAuthHandler) listExistingGrokIdentitySets(ctx context.Context) (map[string]struct{}, map[string]struct{}, error) {
	ssoSet := make(map[string]struct{})
	emailSet := make(map[string]struct{})
	page := 1
	pageSize := dataPageCap
	for {
		items, total, err := h.adminService.ListAccounts(ctx, page, pageSize, service.PlatformGrok, "", "", "", 0, "", "created_at", "desc")
		if err != nil {
			return nil, nil, err
		}
		for i := range items {
			for _, sso := range extractAccountSSOTokens(items[i]) {
				ssoSet[sso] = struct{}{}
			}
			for _, email := range extractAccountEmails(items[i]) {
				emailSet[email] = struct{}{}
			}
		}
		if len(items) == 0 || page*pageSize >= int(total) {
			break
		}
		page++
	}
	return ssoSet, emailSet, nil
}

func (h *GrokOAuthHandler) listExistingGrokSSOSet(ctx context.Context) (map[string]struct{}, error) {
	sso, _, err := h.listExistingGrokIdentitySets(ctx)
	return sso, err
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

func extractAccountEmails(account service.Account) []string {
	candidates := []string{
		account.GetCredential("email"),
		account.GetCredential("user_email"),
		account.GetCredential("userEmail"),
	}
	// Some historical rows only put email in the account name like "user@x.com".
	if strings.Contains(account.Name, "@") {
		candidates = append(candidates, account.Name)
	}
	result := make([]string, 0, 1)
	seen := make(map[string]struct{})
	for _, raw := range candidates {
		email := normalizeGrokEmail(raw)
		if email == "" {
			continue
		}
		if _, ok := seen[email]; ok {
			continue
		}
		seen[email] = struct{}{}
		result = append(result, email)
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

func normalizeGrokEmail(raw string) string {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || !strings.Contains(email, "@") {
		return ""
	}
	return email
}

func firstNonEmptyA2G(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func accountIDFromDTO(account *dto.Account) int64 {
	if account == nil {
		return 0
	}
	return account.ID
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

func (h *GrokOAuthHandler) findGrokAccountByEmail(ctx context.Context, email string) (*service.Account, error) {
	email = normalizeGrokEmail(email)
	if email == "" {
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
			for _, existing := range extractAccountEmails(items[i]) {
				if existing == email {
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
