package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GrokCPAImportRequest accepts one or more CLIProxy/CPA xai OAuth JSON payloads.
// Compatible with Desktop/Grok/cpa xai-*.json:
//
//	type=xai, auth_kind=oauth, email/sub, access_token/refresh_token/id_token, base_url, expired
type GrokCPAImportRequest struct {
	Content                 string           `json:"content"`
	Contents                []string         `json:"contents"`
	Items                   []map[string]any `json:"items"`
	Name                    string           `json:"name"`
	Notes                   *string          `json:"notes"`
	GroupIDs                []int64          `json:"group_ids"`
	ProxyID                 *int64           `json:"proxy_id"`
	Concurrency             *int             `json:"concurrency"`
	Priority                *int             `json:"priority"`
	RateMultiplier          *float64         `json:"rate_multiplier"`
	LoadFactor              *int             `json:"load_factor"`
	ExpiresAt               *int64           `json:"expires_at"`
	AutoPauseOnExpired      *bool            `json:"auto_pause_on_expired"`
	CredentialExtras        map[string]any   `json:"credential_extras"`
	Extra                   map[string]any   `json:"extra"`
	UpdateExisting          *bool            `json:"update_existing"`
	SkipDefaultGroupBind    *bool            `json:"skip_default_group_bind"`
	ConfirmMixedChannelRisk *bool            `json:"confirm_mixed_channel_risk"`
}

type GrokCPAImportResult struct {
	Total   int                    `json:"total"`
	Created int                    `json:"created"`
	Updated int                    `json:"updated"`
	Skipped int                    `json:"skipped"`
	Failed  int                    `json:"failed"`
	Items   []GrokCPAImportItem    `json:"items,omitempty"`
	Errors  []GrokCPAImportMessage `json:"errors,omitempty"`
}

type GrokCPAImportItem struct {
	Index     int    `json:"index"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty"`
	Action    string `json:"action"`
	AccountID int64  `json:"account_id,omitempty"`
	Message   string `json:"message,omitempty"`
}

type GrokCPAImportMessage struct {
	Index   int    `json:"index"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

type grokCPAImportEntry struct {
	Index int
	Value any
}

type grokCPANormalized struct {
	Name         string
	Email        string
	Sub          string
	Credentials  map[string]any
	IdentityKeys []string
}

// ImportGrokCPA imports CLIProxy/CPA xai OAuth JSON into Grok OAuth accounts.
// POST /api/v1/admin/accounts/import/grok-cpa
func (h *AccountHandler) ImportGrokCPA(c *gin.Context) {
	var req GrokCPAImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.Concurrency != nil && *req.Concurrency < 0 {
		response.BadRequest(c, "concurrency must be >= 0")
		return
	}
	if req.Priority != nil && *req.Priority < 0 {
		response.BadRequest(c, "priority must be >= 0")
		return
	}
	if req.RateMultiplier != nil && *req.RateMultiplier < 0 {
		response.BadRequest(c, "rate_multiplier must be >= 0")
		return
	}
	if req.LoadFactor != nil && *req.LoadFactor > 10000 {
		response.BadRequest(c, "load_factor must be <= 10000")
		return
	}

	entries, err := parseGrokCPAImportEntries(req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if len(entries) == 0 {
		response.BadRequest(c, "请提供 CPA/xai OAuth JSON（content/contents/items）")
		return
	}

	executeAdminIdempotentJSON(c, "admin.accounts.import_grok_cpa", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.importGrokCPA(ctx, req, entries)
	})
}

func (h *AccountHandler) importGrokCPA(ctx context.Context, req GrokCPAImportRequest, entries []grokCPAImportEntry) (GrokCPAImportResult, error) {
	result := GrokCPAImportResult{
		Total: len(entries),
		Items: make([]GrokCPAImportItem, 0, len(entries)),
	}

	existingAccounts, err := h.listAccountsFiltered(ctx, service.PlatformGrok, service.AccountTypeOAuth, "", "", 0, "", "created_at", "desc")
	if err != nil {
		return result, err
	}
	index := buildGrokCPAAccountIndex(existingAccounts)

	updateExisting := true
	if req.UpdateExisting != nil {
		updateExisting = *req.UpdateExisting
	}
	concurrency := 1
	if req.Concurrency != nil {
		concurrency = *req.Concurrency
	}
	priority := 1
	if req.Priority != nil {
		priority = *req.Priority
	}
	skipDefaultGroupBind := false
	if req.SkipDefaultGroupBind != nil {
		skipDefaultGroupBind = *req.SkipDefaultGroupBind
	}
	skipMixedChannelCheck := req.ConfirmMixedChannelRisk != nil && *req.ConfirmMixedChannelRisk
	credentialExtras := sanitizeGrokCPACredentialExtras(req.CredentialExtras)

	seen := map[string]struct{}{}
	for _, entry := range entries {
		item, normErr := normalizeGrokCPAImportEntry(entry.Value)
		if normErr != nil {
			result.Failed++
			result.Items = append(result.Items, GrokCPAImportItem{
				Index:   entry.Index,
				Action:  "failed",
				Message: normErr.Error(),
			})
			result.Errors = append(result.Errors, GrokCPAImportMessage{
				Index:   entry.Index,
				Message: normErr.Error(),
			})
			continue
		}

		accountName := buildGrokCPAAccountName(req.Name, item, entry.Index, len(entries))
		credentials := service.MergeCredentials(cloneGrokCPAMap(credentialExtras), item.Credentials)
		if req.Extra != nil {
			// Extra is account-level, not credentials.
		}

		// Batch-local dedupe
		dupKey := ""
		for _, k := range item.IdentityKeys {
			if _, ok := seen[k]; ok {
				dupKey = k
				break
			}
		}
		if dupKey != "" {
			result.Skipped++
			result.Items = append(result.Items, GrokCPAImportItem{
				Index:   entry.Index,
				Name:    accountName,
				Email:   item.Email,
				Action:  "skipped",
				Message: "duplicate identity in batch: " + dupKey,
			})
			continue
		}
		for _, k := range item.IdentityKeys {
			seen[k] = struct{}{}
		}

		existing := index.Find(item.IdentityKeys)
		if existing != nil {
			if !updateExisting {
				result.Skipped++
				result.Items = append(result.Items, GrokCPAImportItem{
					Index:     entry.Index,
					Name:      accountName,
					Email:     item.Email,
					Action:    "skipped",
					AccountID: existing.ID,
					Message:   "existing account not updated",
				})
				continue
			}
			updateInput := &service.UpdateAccountInput{
				Name:                  accountName,
				Notes:                 req.Notes,
				Type:                  service.AccountTypeOAuth,
				Credentials:           credentials,
				Extra:                 cloneGrokCPAMap(req.Extra),
				Concurrency:           &concurrency,
				Priority:              &priority,
				RateMultiplier:        req.RateMultiplier,
				LoadFactor:            req.LoadFactor,
				ExpiresAt:             req.ExpiresAt,
				AutoPauseOnExpired:    req.AutoPauseOnExpired,
				SkipMixedChannelCheck: skipMixedChannelCheck,
			}
			if req.ProxyID != nil {
				updateInput.ProxyID = req.ProxyID
			}
			if len(req.GroupIDs) > 0 {
				groupIDs := append([]int64(nil), req.GroupIDs...)
				updateInput.GroupIDs = &groupIDs
			}
			updated, updateErr := h.adminService.UpdateAccount(ctx, existing.ID, updateInput)
			if updateErr != nil {
				result.Failed++
				result.Items = append(result.Items, GrokCPAImportItem{
					Index:   entry.Index,
					Name:    accountName,
					Email:   item.Email,
					Action:  "failed",
					Message: updateErr.Error(),
				})
				result.Errors = append(result.Errors, GrokCPAImportMessage{
					Index:   entry.Index,
					Name:    accountName,
					Message: updateErr.Error(),
				})
				continue
			}
			if h.tokenCacheInvalidator != nil && updated != nil {
				_ = h.tokenCacheInvalidator.InvalidateToken(ctx, updated)
			}
			accountID := existing.ID
			if updated != nil {
				accountID = updated.ID
				index.Add(*updated)
				h.scheduleGrokImportProbe(updated)
			}
			result.Updated++
			result.Items = append(result.Items, GrokCPAImportItem{
				Index:     entry.Index,
				Name:      accountName,
				Email:     item.Email,
				Action:    "updated",
				AccountID: accountID,
			})
			continue
		}

		account, createErr := h.adminService.CreateAccount(ctx, &service.CreateAccountInput{
			Name:                  accountName,
			Notes:                 req.Notes,
			Platform:              service.PlatformGrok,
			Type:                  service.AccountTypeOAuth,
			Credentials:           credentials,
			Extra:                 cloneGrokCPAMap(req.Extra),
			ProxyID:               req.ProxyID,
			Concurrency:           concurrency,
			Priority:              priority,
			RateMultiplier:        req.RateMultiplier,
			LoadFactor:            req.LoadFactor,
			GroupIDs:              req.GroupIDs,
			ExpiresAt:             req.ExpiresAt,
			AutoPauseOnExpired:    req.AutoPauseOnExpired,
			SkipDefaultGroupBind:  skipDefaultGroupBind,
			SkipMixedChannelCheck: skipMixedChannelCheck,
		})
		if createErr != nil {
			result.Failed++
			result.Items = append(result.Items, GrokCPAImportItem{
				Index:   entry.Index,
				Name:    accountName,
				Email:   item.Email,
				Action:  "failed",
				Message: createErr.Error(),
			})
			result.Errors = append(result.Errors, GrokCPAImportMessage{
				Index:   entry.Index,
				Name:    accountName,
				Message: createErr.Error(),
			})
			continue
		}
		accountID := int64(0)
		if account != nil {
			accountID = account.ID
			index.Add(*account)
			h.scheduleGrokImportProbe(account)
		}
		result.Created++
		result.Items = append(result.Items, GrokCPAImportItem{
			Index:     entry.Index,
			Name:      accountName,
			Email:     item.Email,
			Action:    "created",
			AccountID: accountID,
		})
	}

	return result, nil
}

func parseGrokCPAImportEntries(req GrokCPAImportRequest) ([]grokCPAImportEntry, error) {
	entries := make([]grokCPAImportEntry, 0)
	index := 0

	addRaw := func(raw string) error {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil
		}
		values, err := parseGrokCPAJSONContent(raw)
		if err != nil {
			return err
		}
		for _, value := range values {
			index++
			entries = append(entries, grokCPAImportEntry{Index: index, Value: value})
		}
		return nil
	}

	if err := addRaw(req.Content); err != nil {
		return nil, err
	}
	for _, content := range req.Contents {
		if err := addRaw(content); err != nil {
			return nil, err
		}
	}
	for _, item := range req.Items {
		if item == nil {
			continue
		}
		index++
		entries = append(entries, grokCPAImportEntry{Index: index, Value: item})
	}
	return entries, nil
}

func parseGrokCPAJSONContent(content string) ([]any, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil
	}
	// JSON array
	if strings.HasPrefix(content, "[") {
		var arr []any
		if err := json.Unmarshal([]byte(content), &arr); err != nil {
			return nil, fmt.Errorf("invalid JSON array: %w", err)
		}
		return arr, nil
	}
	// Single object
	if strings.HasPrefix(content, "{") {
		var obj map[string]any
		if err := json.Unmarshal([]byte(content), &obj); err != nil {
			return nil, fmt.Errorf("invalid JSON object: %w", err)
		}
		return []any{obj}, nil
	}
	// NDJSON
	lines := strings.Split(content, "\n")
	out := make([]any, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			return nil, fmt.Errorf("invalid NDJSON line: %w", err)
		}
		out = append(out, obj)
	}
	return out, nil
}

func normalizeGrokCPAImportEntry(value any) (*grokCPANormalized, error) {
	obj, ok := value.(map[string]any)
	if !ok {
		// json.Unmarshal numbers may leave nested maps as map[string]any already;
		// try re-marshal.
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("CPA entry is not an object")
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("CPA entry is not an object")
		}
		obj = m
	}

	// Nested credentials support
	if nested, ok := obj["credentials"].(map[string]any); ok && nested != nil {
		// Prefer nested token fields but keep top-level email/sub/type.
		merged := cloneGrokCPAMap(nested)
		for k, v := range obj {
			if k == "credentials" {
				continue
			}
			if _, exists := merged[k]; !exists {
				merged[k] = v
			}
		}
		obj = merged
	}

	accessToken := firstNonEmptyString(obj, "access_token", "accessToken")
	refreshToken := firstNonEmptyString(obj, "refresh_token", "refreshToken")
	idToken := firstNonEmptyString(obj, "id_token", "idToken")
	sso := normalizeGrokCPASSO(firstNonEmptyString(obj, "sso", "sso_token", "ssoToken"))
	if accessToken == "" && refreshToken == "" && sso == "" {
		return nil, fmt.Errorf("CPA JSON 缺少 access_token/refresh_token/sso")
	}
	if refreshToken == "" && sso == "" {
		return nil, fmt.Errorf("CPA JSON 缺少 refresh_token，无法创建长期 OAuth 账号")
	}

	claims := map[string]any{}
	if accessToken != "" {
		claims = xai.DecodeJWTClaims(accessToken)
	}
	if len(claims) == 0 && idToken != "" {
		claims = xai.DecodeJWTClaims(idToken)
	}

	email := firstNonEmptyString(obj, "email", "name")
	if email == "" {
		email = xai.JWTClaimString(claims, "email")
	}
	sub := firstNonEmptyString(obj, "sub", "subject")
	if sub == "" {
		sub = xai.JWTClaimString(claims, "sub")
	}
	if sub == "" {
		sub = xai.JWTClaimString(claims, "principal_id")
	}
	clientID := firstNonEmptyString(obj, "client_id", "clientId")
	if clientID == "" {
		clientID = xai.JWTClaimString(claims, "client_id")
	}
	if clientID == "" {
		if aud := xai.JWTClaimString(claims, "aud"); aud != "" {
			clientID = aud
		}
	}
	if clientID == "" {
		clientID = xai.EffectiveClientID()
	}
	teamID := firstNonEmptyString(obj, "team_id", "teamId")
	if teamID == "" {
		teamID = xai.JWTClaimString(claims, "team_id")
	}
	scope := firstNonEmptyString(obj, "scope")
	tokenType := firstNonEmptyString(obj, "token_type", "tokenType")
	if tokenType == "" {
		tokenType = "Bearer"
	}
	baseURL := firstNonEmptyString(obj, "base_url", "baseUrl")
	if baseURL == "" {
		baseURL = xai.DefaultCLIBaseURL
	}
	expiresAt := firstNonEmptyString(obj, "expired", "expires_at", "expiresAt")
	if expiresAt == "" {
		if exp := jwtClaimInt64(claims, "exp"); exp > 0 {
			expiresAt = time.Unix(exp, 0).UTC().Format(time.RFC3339)
		}
	}

	credentials := map[string]any{
		"base_url":   baseURL,
		"client_id":  clientID,
		"token_type": tokenType,
	}
	if scope != "" {
		credentials["scope"] = scope
	}
	if email != "" {
		credentials["email"] = email
	}
	if sub != "" {
		credentials["sub"] = sub
	}
	if teamID != "" {
		credentials["team_id"] = teamID
	}
	if expiresAt != "" {
		credentials["expires_at"] = expiresAt
	}
	if accessToken != "" {
		credentials["access_token"] = accessToken
	}
	if refreshToken != "" {
		credentials["refresh_token"] = refreshToken
	}
	if idToken != "" {
		credentials["id_token"] = idToken
	}
	// Keep SSO only as optional operational hint; OAuth path does not require it.
	if sso != "" {
		credentials["sso"] = sso
	}

	name := email
	if name == "" && sub != "" {
		if len(sub) > 10 {
			name = "grok-" + sub[:10]
		} else {
			name = "grok-" + sub
		}
	}
	if name == "" {
		name = "grok-cpa"
	}

	identityKeys := make([]string, 0, 3)
	if email != "" {
		identityKeys = append(identityKeys, "email:"+strings.ToLower(strings.TrimSpace(email)))
	}
	if sub != "" {
		identityKeys = append(identityKeys, "sub:"+strings.ToLower(strings.TrimSpace(sub)))
	}
	if refreshToken != "" {
		identityKeys = append(identityKeys, "rt:"+refreshToken)
	}

	return &grokCPANormalized{
		Name:         name,
		Email:        email,
		Sub:          sub,
		Credentials:  credentials,
		IdentityKeys: identityKeys,
	}, nil
}

func buildGrokCPAAccountName(base string, item *grokCPANormalized, index, total int) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return item.Name
	}
	if total <= 1 {
		return base
	}
	return fmt.Sprintf("%s-%d", base, index)
}

type grokCPAAccountIndex struct {
	byKey map[string][]service.Account
}

func buildGrokCPAAccountIndex(accounts []service.Account) *grokCPAAccountIndex {
	idx := &grokCPAAccountIndex{byKey: map[string][]service.Account{}}
	for i := range accounts {
		idx.Add(accounts[i])
	}
	return idx
}

func (idx *grokCPAAccountIndex) Add(account service.Account) {
	if idx == nil {
		return
	}
	keys := grokCPAIdentityKeysFromAccount(account)
	for _, key := range keys {
		idx.byKey[key] = append(idx.byKey[key], account)
	}
}

func (idx *grokCPAAccountIndex) Find(keys []string) *service.Account {
	if idx == nil {
		return nil
	}
	for _, key := range keys {
		if accounts := idx.byKey[key]; len(accounts) > 0 {
			acc := accounts[0]
			return &acc
		}
	}
	return nil
}

func grokCPAIdentityKeysFromAccount(account service.Account) []string {
	keys := make([]string, 0, 3)
	email := strings.ToLower(strings.TrimSpace(account.GetCredential("email")))
	if email == "" {
		email = strings.ToLower(strings.TrimSpace(account.Name))
	}
	if email != "" {
		keys = append(keys, "email:"+email)
	}
	if sub := strings.ToLower(strings.TrimSpace(account.GetCredential("sub"))); sub != "" {
		keys = append(keys, "sub:"+sub)
	}
	if rt := strings.TrimSpace(account.GetCredential("refresh_token")); rt != "" {
		keys = append(keys, "rt:"+rt)
	}
	return keys
}

func sanitizeGrokCPACredentialExtras(extras map[string]any) map[string]any {
	if extras == nil {
		return nil
	}
	// Never allow extras to override token identity fields.
	blocked := map[string]struct{}{
		"access_token":  {},
		"refresh_token": {},
		"id_token":      {},
		"sso":           {},
		"email":         {},
		"sub":           {},
	}
	out := make(map[string]any, len(extras))
	for k, v := range extras {
		lk := strings.ToLower(strings.TrimSpace(k))
		if _, bad := blocked[lk]; bad {
			continue
		}
		out[k] = v
	}
	return out
}

func cloneGrokCPAMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	out := make(map[string]any, len(source))
	for k, v := range source {
		out[k] = v
	}
	return out
}

func firstNonEmptyString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if obj == nil {
			return ""
		}
		if v, ok := obj[key]; ok {
			switch t := v.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" {
					return s
				}
			case fmt.Stringer:
				if s := strings.TrimSpace(t.String()); s != "" {
					return s
				}
			case float64:
				if t != 0 {
					return strconv.FormatInt(int64(t), 10)
				}
			case json.Number:
				if s := strings.TrimSpace(t.String()); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func normalizeGrokCPASSO(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, "sso=") {
		text = strings.TrimSpace(text[4:])
	} else if strings.HasPrefix(lower, "sso:") {
		text = strings.TrimSpace(text[4:])
	}
	return strings.Trim(text, "\"'")
}

func jwtClaimInt64(claims map[string]any, key string) int64 {
	if claims == nil {
		return 0
	}
	v, ok := claims[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int64(t)
	case json.Number:
		n, _ := t.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n
	case int64:
		return t
	case int:
		return int64(t)
	default:
		return 0
	}
}

