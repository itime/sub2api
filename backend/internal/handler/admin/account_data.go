package admin

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"log/slog"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

const (
	dataType       = "sub2api-data"
	legacyDataType = "sub2api-bundle"
	dataVersion    = 1
	dataPageCap              = 1000
	fastImportMaxConcurrency = 30
)

type DataPayload struct {
	Type       string        `json:"type,omitempty"`
	Version    int           `json:"version,omitempty"`
	ExportedAt string        `json:"exported_at"`
	Proxies    []DataProxy   `json:"proxies"`
	Accounts   []DataAccount `json:"accounts"`
}

type DataProxy struct {
	ProxyKey string `json:"proxy_key"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Status   string `json:"status"`
}

// DataAccount 是管理员显式备份导出使用的账号结构，故意不走 dto.Account 的脱敏路径，
// Credentials 原文返回。这是"管理员备份"这一显式行为的一部分；如未来需要导出脱敏版本，
// 应新增独立结构而非修改这里。
type DataAccount struct {
	Name               string         `json:"name"`
	Notes              *string        `json:"notes,omitempty"`
	Platform           string         `json:"platform"`
	Type               string         `json:"type"`
	Credentials        map[string]any `json:"credentials"`
	Extra              map[string]any `json:"extra,omitempty"`
	ProxyKey           *string        `json:"proxy_key,omitempty"`
	GroupIDs           []int64        `json:"group_ids,omitempty"`
	Concurrency        int            `json:"concurrency"`
	Priority           int            `json:"priority"`
	RateMultiplier     *float64       `json:"rate_multiplier,omitempty"`
	ExpiresAt          *int64         `json:"expires_at,omitempty"`
	AutoPauseOnExpired *bool          `json:"auto_pause_on_expired,omitempty"`
	// CreatedAt 用于在导入时显式指定账号的创建时间。
	// - 当来源端（例如 openai-free）已有真实创建时间时，应通过该字段透传，
	//   避免导入后的 created_at 被覆盖为导入瞬间的时间。
	// - 同时支持秒级 Unix 时间戳（int64）与 ISO 8601 字符串两种形式。
	// - 字段缺失或解析失败时，保持原行为（由 ent 默认填充当前时间）。
	CreatedAt any `json:"created_at,omitempty"`
}

type DataImportRequest struct {
	Data                 DataPayload `json:"data"`
	SkipDefaultGroupBind *bool       `json:"skip_default_group_bind"`
	FastImport           *bool       `json:"fast_import"`
	DefaultProxyID       *int64      `json:"default_proxy_id,omitempty"`
	DefaultGroupIDs      []int64     `json:"default_group_ids,omitempty"`
}

type DataImportResult struct {
	ProxiesReceived int               `json:"proxies_received"`
	AccountsReceived int               `json:"accounts_received"`
	ProxyCreated    int               `json:"proxy_created"`
	ProxyReused     int               `json:"proxy_reused"`
	ProxySkipped    int               `json:"proxy_skipped"`
	ProxyFailed     int               `json:"proxy_failed"`
	AccountCreated  int               `json:"account_created"`
	AccountUpdated  int               `json:"account_updated"`
	AccountSkipped  int               `json:"account_skipped"`
	AccountFailed      int               `json:"account_failed"`
	CreatedAccountIDs  []int64           `json:"created_account_ids,omitempty"`
	Items              []DataImportItem  `json:"items,omitempty"`
	Errors             []DataImportError `json:"errors,omitempty"`
}

type DataImportItem struct {
	Kind     string `json:"kind"`
	Name     string `json:"name,omitempty"`
	Action   string `json:"action"`
	Message  string `json:"message,omitempty"`
	ProxyKey string `json:"proxy_key,omitempty"`
}

type DataImportError struct {
	Kind     string `json:"kind"`
	Name     string `json:"name,omitempty"`
	ProxyKey string `json:"proxy_key,omitempty"`
	Message  string `json:"message"`
}

func buildProxyKey(protocol, host string, port int, username, password string) string {
	return fmt.Sprintf("%s|%s|%d|%s|%s", strings.TrimSpace(protocol), strings.TrimSpace(host), port, strings.TrimSpace(username), strings.TrimSpace(password))
}

func (h *AccountHandler) ExportData(c *gin.Context) {
	ctx := c.Request.Context()

	selectedIDs, err := parseAccountIDs(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	accounts, err := h.resolveExportAccounts(ctx, selectedIDs, c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	includeProxies, err := parseIncludeProxies(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var proxies []service.Proxy
	if includeProxies {
		proxies, err = h.resolveExportProxies(ctx, accounts)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	} else {
		proxies = []service.Proxy{}
	}

	proxyKeyByID := make(map[int64]string, len(proxies))
	dataProxies := make([]DataProxy, 0, len(proxies))
	for i := range proxies {
		p := proxies[i]
		key := buildProxyKey(p.Protocol, p.Host, p.Port, p.Username, p.Password)
		proxyKeyByID[p.ID] = key
		dataProxies = append(dataProxies, DataProxy{
			ProxyKey: key,
			Name:     p.Name,
			Protocol: p.Protocol,
			Host:     p.Host,
			Port:     p.Port,
			Username: p.Username,
			Password: p.Password,
			Status:   p.Status,
		})
	}

	dataAccounts := make([]DataAccount, 0, len(accounts))
	for i := range accounts {
		acc := accounts[i]
		var proxyKey *string
		if acc.ProxyID != nil {
			if key, ok := proxyKeyByID[*acc.ProxyID]; ok {
				proxyKey = &key
			}
		}
		var expiresAt *int64
		if acc.ExpiresAt != nil {
			v := acc.ExpiresAt.Unix()
			expiresAt = &v
		}
		dataAccounts = append(dataAccounts, DataAccount{
			Name:               acc.Name,
			Notes:              acc.Notes,
			Platform:           acc.Platform,
			Type:               acc.Type,
			Credentials:        acc.Credentials,
			Extra:              acc.Extra,
			ProxyKey:           proxyKey,
			Concurrency:        acc.Concurrency,
			Priority:           acc.Priority,
			RateMultiplier:     acc.RateMultiplier,
			ExpiresAt:          expiresAt,
			AutoPauseOnExpired: &acc.AutoPauseOnExpired,
			CreatedAt:          acc.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	payload := DataPayload{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Proxies:    dataProxies,
		Accounts:   dataAccounts,
	}

	response.Success(c, payload)
}

func (h *AccountHandler) ImportData(c *gin.Context) {
	var req DataImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if err := validateDataHeader(req.Data); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	executeAdminIdempotentJSON(c, "admin.accounts.import_data", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.importData(ctx, req)
	})
}

func (h *AccountHandler) importData(ctx context.Context, req DataImportRequest) (DataImportResult, error) {
	skipDefaultGroupBind := true
	if req.SkipDefaultGroupBind != nil {
		skipDefaultGroupBind = *req.SkipDefaultGroupBind
	}
	fastImport := req.FastImport != nil && *req.FastImport

	dataPayload := req.Data
	result := DataImportResult{
		ProxiesReceived:  len(dataPayload.Proxies),
		AccountsReceived: len(dataPayload.Accounts),
	}

	existingProxies, err := h.listAllProxies(ctx)
	if err != nil {
		return result, err
	}

	proxyKeyToID := make(map[string]int64, len(existingProxies))
	proxyNameToID := make(map[string]int64, len(existingProxies))
	for i := range existingProxies {
		p := existingProxies[i]
		key := buildProxyKey(p.Protocol, p.Host, p.Port, p.Username, p.Password)
		proxyKeyToID[key] = p.ID
		nameKey := normalizeProxyNameKey(p.Name)
		if nameKey != "" {
			proxyNameToID[nameKey] = p.ID
			proxyKeyToID[buildProxyNameKey(p.Name)] = p.ID
		}
	}

	for i := range dataPayload.Proxies {
		item := dataPayload.Proxies[i]
		key := item.ProxyKey
		if key == "" {
			key = buildProxyKey(item.Protocol, item.Host, item.Port, item.Username, item.Password)
		}
		if err := validateDataProxy(item); err != nil {
			result.ProxyFailed++
			recordImportError(&result, DataImportError{
				Kind:     "proxy",
				Name:     item.Name,
				ProxyKey: key,
				Message:  err.Error(),
			})
			continue
		}
		normalizedStatus := normalizeProxyStatus(item.Status)
		if existingID, ok := proxyKeyToID[key]; ok {
			registerImportedProxyKeys(proxyKeyToID, proxyNameToID, item, key, existingID)
			result.ProxyReused++
			recordImportItem(&result, DataImportItem{
				Kind:     "proxy",
				Name:     item.Name,
				Action:   "reused",
				ProxyKey: key,
				Message:  "matched existing proxy",
			})
			if normalizedStatus != "" {
				if proxy, getErr := h.adminService.GetProxy(ctx, existingID); getErr == nil && proxy != nil && proxy.Status != normalizedStatus {
					_, _ = h.adminService.UpdateProxy(ctx, existingID, &service.UpdateProxyInput{
						Status: normalizedStatus,
					})
				}
			}
			continue
		}
		if existingID, ok := proxyNameToID[normalizeProxyNameKey(item.Name)]; ok {
			registerImportedProxyKeys(proxyKeyToID, proxyNameToID, item, key, existingID)
			result.ProxyReused++
			recordImportItem(&result, DataImportItem{
				Kind:     "proxy",
				Name:     item.Name,
				Action:   "reused",
				ProxyKey: key,
				Message:  "matched existing proxy by name",
			})
			if normalizedStatus != "" {
				if proxy, getErr := h.adminService.GetProxy(ctx, existingID); getErr == nil && proxy != nil && proxy.Status != normalizedStatus {
					_, _ = h.adminService.UpdateProxy(ctx, existingID, &service.UpdateProxyInput{
						Status: normalizedStatus,
					})
				}
			}
			continue
		}

		created, createErr := h.adminService.CreateProxy(ctx, &service.CreateProxyInput{
			Name:     defaultProxyName(item.Name),
			Protocol: item.Protocol,
			Host:     item.Host,
			Port:     item.Port,
			Username: item.Username,
			Password: item.Password,
		})
		if createErr != nil {
			result.ProxyFailed++
			recordImportError(&result, DataImportError{
				Kind:     "proxy",
				Name:     item.Name,
				ProxyKey: key,
				Message:  createErr.Error(),
			})
			continue
		}
		registerImportedProxyKeys(proxyKeyToID, proxyNameToID, item, key, created.ID)
		result.ProxyCreated++
		recordImportItem(&result, DataImportItem{
			Kind:     "proxy",
			Name:     item.Name,
			Action:   "created",
			ProxyKey: key,
		})

		if normalizedStatus != "" && normalizedStatus != created.Status {
			_, _ = h.adminService.UpdateProxy(ctx, created.ID, &service.UpdateProxyInput{
				Status: normalizedStatus,
			})
		}
	}

	// 收集需要异步设置隐私的 Antigravity OAuth 账号
	var privacyAccounts []*service.Account

	var emailIndex map[importAccountKey]map[string]service.Account
	if !fastImport {
		var indexErr error
		emailIndex, indexErr = h.buildImportAccountEmailIndex(ctx, dataPayload.Accounts)
		if indexErr != nil {
			return result, indexErr
		}
	}

	if fastImport {
		h.importAccountsFastConcurrent(ctx, req, dataPayload.Accounts, proxyKeyToID, skipDefaultGroupBind, &result, &privacyAccounts)
	} else {
		h.importAccountsSequential(ctx, req, dataPayload.Accounts, proxyKeyToID, skipDefaultGroupBind, emailIndex, &result, &privacyAccounts)
	}

	// 异步设置 Antigravity 隐私，避免大量导入时阻塞请求
	if len(privacyAccounts) > 0 {
		adminSvc := h.adminService
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("import_antigravity_privacy_panic", "recover", r)
				}
			}()
			bgCtx := context.Background()
			for _, acc := range privacyAccounts {
				adminSvc.ForceAntigravityPrivacy(bgCtx, acc)
			}
			slog.Info("import_antigravity_privacy_done", "count", len(privacyAccounts))
		}()
	}

	return result, nil
}

func (h *AccountHandler) importAccountsSequential(
	ctx context.Context,
	req DataImportRequest,
	accounts []DataAccount,
	proxyKeyToID map[string]int64,
	skipDefaultGroupBind bool,
	emailIndex map[importAccountKey]map[string]service.Account,
	result *DataImportResult,
	privacyAccounts *[]*service.Account,
) {
	for i := range accounts {
		item := accounts[i]
		normalizeImportAccount(&item)
		if err := validateDataAccount(item); err != nil {
			result.AccountFailed++
			recordImportError(result, DataImportError{
				Kind:    "account",
				Name:    item.Name,
				Message: err.Error(),
			})
			continue
		}

		proxyID, proxyErr := resolveImportAccountProxyID(item, proxyKeyToID, req.DefaultProxyID)
		if proxyErr != nil {
			result.AccountFailed++
			recordImportError(result, DataImportError{
				Kind:     "account",
				Name:     item.Name,
				ProxyKey: proxyErr.proxyKey,
				Message:  proxyErr.message,
			})
			continue
		}

		groupIDs := resolveImportAccountGroupIDs(item.GroupIDs, req.DefaultGroupIDs)
		enrichCredentialsFromIDToken(&item)

		existingAccount := lookupImportAccountForUpdate(emailIndex, item)
		if existingAccount != nil {
			concurrency := item.Concurrency
			priority := item.Priority
			updateInput := &service.UpdateAccountInput{
				Name:                  item.Name,
				Notes:                 item.Notes,
				Type:                  item.Type,
				Credentials:           item.Credentials,
				Extra:                 item.Extra,
				ProxyID:               normalizeImportedProxyID(proxyID, item.ProxyKey != nil),
				Concurrency:           &concurrency,
				Priority:              &priority,
				RateMultiplier:        item.RateMultiplier,
				GroupIDs:              &groupIDs,
				ExpiresAt:             item.ExpiresAt,
				AutoPauseOnExpired:    item.AutoPauseOnExpired,
				Status:                service.StatusActive,
				SkipMixedChannelCheck: true,
			}
			updated, updateErr := h.adminService.UpdateAccount(ctx, existingAccount.ID, updateInput)
			if updateErr != nil {
				result.AccountFailed++
				recordImportError(result, DataImportError{
					Kind:    "account",
					Name:    item.Name,
					Message: updateErr.Error(),
				})
				continue
			}
			if _, clearErr := h.adminService.ClearAccountError(ctx, existingAccount.ID); clearErr != nil {
				slog.Warn("import_account_clear_error_failed", "account_id", existingAccount.ID, "error", clearErr)
			}
			if _, schedErr := h.adminService.SetAccountSchedulable(ctx, existingAccount.ID, true); schedErr != nil {
				result.AccountFailed++
				recordImportError(result, DataImportError{
					Kind:    "account",
					Name:    item.Name,
					Message: schedErr.Error(),
				})
				continue
			}
			if updated.Platform == service.PlatformAntigravity && updated.Type == service.AccountTypeOAuth {
				*privacyAccounts = append(*privacyAccounts, updated)
			}
			result.AccountUpdated++
			recordImportItem(result, DataImportItem{
				Kind:    "account",
				Name:    item.Name,
				Action:  "updated",
				Message: "matched existing account by email",
			})
			continue
		}

		created, err := h.createImportedAccount(ctx, item, proxyID, groupIDs, skipDefaultGroupBind)
		if err != nil {
			result.AccountFailed++
			recordImportError(result, DataImportError{
				Kind:    "account",
				Name:    item.Name,
				Message: err.Error(),
			})
			continue
		}
		if created.Platform == service.PlatformAntigravity && created.Type == service.AccountTypeOAuth {
			*privacyAccounts = append(*privacyAccounts, created)
		}
		result.AccountCreated++
		result.CreatedAccountIDs = append(result.CreatedAccountIDs, created.ID)
		recordImportItem(result, DataImportItem{
			Kind:   "account",
			Name:   item.Name,
			Action: "created",
		})
	}
}

func (h *AccountHandler) createImportedAccount(
	ctx context.Context,
	item DataAccount,
	proxyID *int64,
	groupIDs []int64,
	skipDefaultGroupBind bool,
) (*service.Account, error) {
	accountInput := &service.CreateAccountInput{
		Name:                 item.Name,
		Notes:                item.Notes,
		Platform:             item.Platform,
		Type:                 item.Type,
		Credentials:          item.Credentials,
		Extra:                item.Extra,
		ProxyID:              proxyID,
		Concurrency:          item.Concurrency,
		Priority:             item.Priority,
		RateMultiplier:       item.RateMultiplier,
		GroupIDs:             groupIDs,
		ExpiresAt:            item.ExpiresAt,
		AutoPauseOnExpired:   item.AutoPauseOnExpired,
		SkipDefaultGroupBind: skipDefaultGroupBind,
		CreatedAt:            parseDataAccountCreatedAt(item.CreatedAt),
	}
	return h.adminService.CreateAccount(ctx, accountInput)
}

func (h *AccountHandler) importAccountsFastConcurrent(
	ctx context.Context,
	req DataImportRequest,
	accounts []DataAccount,
	proxyKeyToID map[string]int64,
	skipDefaultGroupBind bool,
	result *DataImportResult,
	privacyAccounts *[]*service.Account,
) {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(fastImportMaxConcurrency)

	var mu sync.Mutex
	for i := range accounts {
		idx := i
		g.Go(func() error {
			item := accounts[idx]
			normalizeImportAccount(&item)
			if err := validateDataAccount(item); err != nil {
				mu.Lock()
				result.AccountFailed++
				recordImportError(result, DataImportError{
					Kind:    "account",
					Name:    item.Name,
					Message: err.Error(),
				})
				mu.Unlock()
				return nil
			}

			proxyID, proxyErr := resolveImportAccountProxyID(item, proxyKeyToID, req.DefaultProxyID)
			if proxyErr != nil {
				mu.Lock()
				result.AccountFailed++
				recordImportError(result, DataImportError{
					Kind:     "account",
					Name:     item.Name,
					ProxyKey: proxyErr.proxyKey,
					Message:  proxyErr.message,
				})
				mu.Unlock()
				return nil
			}

			groupIDs := resolveImportAccountGroupIDs(item.GroupIDs, req.DefaultGroupIDs)
			enrichCredentialsFromIDToken(&item)

			created, err := h.createImportedAccount(gctx, item, proxyID, groupIDs, skipDefaultGroupBind)
			if err != nil {
				mu.Lock()
				result.AccountFailed++
				recordImportError(result, DataImportError{
					Kind:    "account",
					Name:    item.Name,
					Message: err.Error(),
				})
				mu.Unlock()
				return nil
			}

			mu.Lock()
			if created.Platform == service.PlatformAntigravity && created.Type == service.AccountTypeOAuth {
				*privacyAccounts = append(*privacyAccounts, created)
			}
			result.AccountCreated++
			result.CreatedAccountIDs = append(result.CreatedAccountIDs, created.ID)
			recordImportItem(result, DataImportItem{
				Kind:   "account",
				Name:   item.Name,
				Action: "created",
			})
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()
}

func normalizeImportedProxyID(proxyID *int64, explicit bool) *int64 {
	if proxyID != nil {
		return proxyID
	}
	if !explicit {
		return nil
	}
	zero := int64(0)
	return &zero
}

type importAccountKey struct {
	platform    string
	accountType string
}

func (h *AccountHandler) buildImportAccountEmailIndex(ctx context.Context, items []DataAccount) (map[importAccountKey]map[string]service.Account, error) {
	needed := make(map[importAccountKey]struct{})
	for i := range items {
		key := importAccountKey{
			platform:    strings.TrimSpace(items[i].Platform),
			accountType: strings.TrimSpace(items[i].Type),
		}
		if key.platform == "" || key.accountType == "" {
			continue
		}
		needed[key] = struct{}{}
	}

	index := make(map[importAccountKey]map[string]service.Account, len(needed))
	for key := range needed {
		accounts, err := h.listAccountsFiltered(ctx, key.platform, key.accountType, "", "", 0, "", "created_at", "desc")
		if err != nil {
			return nil, err
		}
		byEmail := make(map[string]service.Account, len(accounts))
		for i := range accounts {
			email := importExistingAccountEmail(accounts[i])
			if email == "" {
				continue
			}
			if _, exists := byEmail[email]; !exists {
				byEmail[email] = accounts[i]
			}
		}
		index[key] = byEmail
	}
	return index, nil
}

func lookupImportAccountForUpdate(index map[importAccountKey]map[string]service.Account, item DataAccount) *service.Account {
	email := importAccountEmail(item)
	if email == "" {
		return nil
	}
	key := importAccountKey{
		platform:    strings.TrimSpace(item.Platform),
		accountType: strings.TrimSpace(item.Type),
	}
	byEmail, ok := index[key]
	if !ok {
		return nil
	}
	account, ok := byEmail[email]
	if !ok {
		return nil
	}
	return &account
}

func (h *AccountHandler) findImportAccountForUpdate(ctx context.Context, item DataAccount) (*service.Account, error) {
	index, err := h.buildImportAccountEmailIndex(ctx, []DataAccount{item})
	if err != nil {
		return nil, err
	}
	return lookupImportAccountForUpdate(index, item), nil
}

func importAccountEmail(item DataAccount) string {
	for _, value := range []any{
		item.Credentials["email"],
		item.Extra["email"],
		item.Name,
	} {
		if email := normalizeImportEmail(value); email != "" {
			return email
		}
	}
	return ""
}

func importExistingAccountEmail(account service.Account) string {
	for _, value := range []any{
		account.Credentials["email"],
		account.Extra["email"],
		account.Name,
	} {
		if email := normalizeImportEmail(value); email != "" {
			return email
		}
	}
	return ""
}

func normalizeImportEmail(value any) string {
	email, ok := value.(string)
	if !ok {
		return ""
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") {
		return ""
	}
	return email
}

func (h *AccountHandler) listAllProxies(ctx context.Context) ([]service.Proxy, error) {
	page := 1
	pageSize := dataPageCap
	var out []service.Proxy
	for {
		items, total, err := h.adminService.ListProxies(ctx, page, pageSize, "", "", "", "created_at", "desc")
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if len(out) >= int(total) || len(items) == 0 {
			break
		}
		page++
	}
	return out, nil
}

func (h *AccountHandler) listAccountsFiltered(ctx context.Context, platform, accountType, status, search string, groupID int64, privacyMode, sortBy, sortOrder string) ([]service.Account, error) {
	page := 1
	pageSize := dataPageCap
	var out []service.Account
	for {
		items, total, err := h.adminService.ListAccounts(ctx, page, pageSize, platform, accountType, status, search, groupID, privacyMode, sortBy, sortOrder)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if len(out) >= int(total) || len(items) == 0 {
			break
		}
		page++
	}
	return out, nil
}

func (h *AccountHandler) resolveExportAccounts(ctx context.Context, ids []int64, c *gin.Context) ([]service.Account, error) {
	if len(ids) > 0 {
		accounts, err := h.adminService.GetAccountsByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		out := make([]service.Account, 0, len(accounts))
		for _, acc := range accounts {
			if acc == nil {
				continue
			}
			out = append(out, *acc)
		}
		return out, nil
	}

	platform := c.Query("platform")
	accountType := c.Query("type")
	status := c.Query("status")
	privacyMode := strings.TrimSpace(c.Query("privacy_mode"))
	search := strings.TrimSpace(c.Query("search"))
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "asc")
	if len(search) > 100 {
		search = search[:100]
	}

	groupID := int64(0)
	if groupIDStr := c.Query("group"); groupIDStr != "" {
		if groupIDStr == accountListGroupUngroupedQueryValue {
			groupID = service.AccountListGroupUngrouped
		} else {
			parsedGroupID, parseErr := strconv.ParseInt(groupIDStr, 10, 64)
			if parseErr != nil || parsedGroupID <= 0 {
				return nil, infraerrors.BadRequest("INVALID_GROUP_FILTER", "invalid group filter")
			}
			groupID = parsedGroupID
		}
	}

	return h.listAccountsFiltered(ctx, platform, accountType, status, search, groupID, privacyMode, sortBy, sortOrder)
}

func (h *AccountHandler) resolveExportProxies(ctx context.Context, accounts []service.Account) ([]service.Proxy, error) {
	if len(accounts) == 0 {
		return []service.Proxy{}, nil
	}

	seen := make(map[int64]struct{})
	ids := make([]int64, 0)
	for i := range accounts {
		if accounts[i].ProxyID == nil {
			continue
		}
		id := *accounts[i].ProxyID
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return []service.Proxy{}, nil
	}

	return h.adminService.GetProxiesByIDs(ctx, ids)
}

func parseAccountIDs(c *gin.Context) ([]int64, error) {
	values := c.QueryArray("ids")
	if len(values) == 0 {
		raw := strings.TrimSpace(c.Query("ids"))
		if raw != "" {
			values = []string{raw}
		}
	}
	if len(values) == 0 {
		return nil, nil
	}

	ids := make([]int64, 0, len(values))
	for _, item := range values {
		for _, part := range strings.Split(item, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil || id <= 0 {
				return nil, fmt.Errorf("invalid account id: %s", part)
			}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func parseIncludeProxies(c *gin.Context) (bool, error) {
	raw := strings.TrimSpace(strings.ToLower(c.Query("include_proxies")))
	if raw == "" {
		return true, nil
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return true, fmt.Errorf("invalid include_proxies value: %s", raw)
	}
}

type importProxyResolveError struct {
	proxyKey string
	message  string
}

func recordImportItem(result *DataImportResult, item DataImportItem) {
	result.Items = append(result.Items, item)
}

func recordImportError(result *DataImportResult, err DataImportError) {
	result.Errors = append(result.Errors, err)
	recordImportItem(result, DataImportItem{
		Kind:     err.Kind,
		Name:     err.Name,
		Action:   "failed",
		ProxyKey: err.ProxyKey,
		Message:  err.Message,
	})
}

func resolveImportAccountGroupIDs(itemGroupIDs, defaultGroupIDs []int64) []int64 {
	if len(itemGroupIDs) > 0 {
		return append([]int64(nil), itemGroupIDs...)
	}
	if len(defaultGroupIDs) == 0 {
		return nil
	}
	return append([]int64(nil), defaultGroupIDs...)
}

func resolveImportAccountProxyID(
	item DataAccount,
	proxyKeyToID map[string]int64,
	defaultProxyID *int64,
) (*int64, *importProxyResolveError) {
	if item.ProxyKey != nil && strings.TrimSpace(*item.ProxyKey) != "" {
		key := strings.TrimSpace(*item.ProxyKey)
		if id, ok := proxyKeyToID[key]; ok {
			return &id, nil
		}
		return nil, &importProxyResolveError{
			proxyKey: key,
			message:  "proxy_key not found in import payload or existing proxies",
		}
	}
	if defaultProxyID != nil && *defaultProxyID > 0 {
		id := *defaultProxyID
		return &id, nil
	}
	return nil, nil
}

func validateDataHeader(payload DataPayload) error {
	if payload.Type != "" && payload.Type != dataType && payload.Type != legacyDataType {
		return fmt.Errorf("unsupported data type: %s", payload.Type)
	}
	if payload.Version != 0 && payload.Version != dataVersion {
		return fmt.Errorf("unsupported data version: %d", payload.Version)
	}
	if payload.Proxies == nil {
		return errors.New("proxies is required")
	}
	if payload.Accounts == nil {
		return errors.New("accounts is required")
	}
	return nil
}

func validateDataProxy(item DataProxy) error {
	if strings.TrimSpace(item.Protocol) == "" {
		return errors.New("proxy protocol is required")
	}
	if strings.TrimSpace(item.Host) == "" {
		return errors.New("proxy host is required")
	}
	if item.Port <= 0 || item.Port > 65535 {
		return errors.New("proxy port is invalid")
	}
	switch item.Protocol {
	case "http", "https", "socks5", "socks5h":
	default:
		return fmt.Errorf("proxy protocol is invalid: %s", item.Protocol)
	}
	if item.Status != "" {
		normalizedStatus := normalizeProxyStatus(item.Status)
		if normalizedStatus != service.StatusActive && normalizedStatus != "inactive" {
			return fmt.Errorf("proxy status is invalid: %s", item.Status)
		}
	}
	return nil
}

// normalizeImportAccount maps external export formats (CPA / Codex CLI session JSON)
// into sub2api's canonical OpenAI OAuth account shape.
func normalizeImportAccount(item *DataAccount) {
	if item == nil {
		return
	}
	applyCPAImportNormalizations(item)
	if strings.EqualFold(strings.TrimSpace(item.Type), "codex") {
		item.Type = service.AccountTypeOAuth
		if strings.TrimSpace(item.Platform) == "" {
			item.Platform = service.PlatformOpenAI
		}
		if item.Extra == nil {
			item.Extra = map[string]any{}
		}
		if _, ok := item.Extra["import_source"]; !ok {
			item.Extra["import_source"] = "codex_session"
		}
	}
}

func validateDataAccount(item DataAccount) error {
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("account name is required")
	}
	if strings.TrimSpace(item.Platform) == "" {
		return errors.New("account platform is required")
	}
	if strings.TrimSpace(item.Type) == "" {
		return errors.New("account type is required")
	}
	if len(item.Credentials) == 0 {
		return errors.New("account credentials is required")
	}
	switch item.Type {
	case service.AccountTypeOAuth, service.AccountTypeSetupToken, service.AccountTypeAPIKey, service.AccountTypeUpstream:
	default:
		return fmt.Errorf("account type is invalid: %s", item.Type)
	}
	if item.RateMultiplier != nil && *item.RateMultiplier < 0 {
		return errors.New("rate_multiplier must be >= 0")
	}
	if item.Concurrency < 0 {
		return errors.New("concurrency must be >= 0")
	}
	if item.Priority < 0 {
		return errors.New("priority must be >= 0")
	}
	return nil
}

func defaultProxyName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "imported-proxy"
	}
	return name
}

func normalizeProxyNameKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func buildProxyNameKey(name string) string {
	nameKey := normalizeProxyNameKey(name)
	if nameKey == "" {
		return ""
	}
	return "name|" + nameKey
}

func registerImportedProxyKeys(proxyKeyToID map[string]int64, proxyNameToID map[string]int64, item DataProxy, importKey string, id int64) {
	if strings.TrimSpace(importKey) != "" {
		proxyKeyToID[importKey] = id
	}
	canonicalKey := buildProxyKey(item.Protocol, item.Host, item.Port, item.Username, item.Password)
	if strings.TrimSpace(canonicalKey) != "" {
		proxyKeyToID[canonicalKey] = id
	}
	nameKey := normalizeProxyNameKey(item.Name)
	if nameKey == "" {
		return
	}
	proxyNameToID[nameKey] = id
	proxyKeyToID[buildProxyNameKey(item.Name)] = id
}

// enrichCredentialsFromIDToken performs best-effort extraction of user info fields
// (email, plan_type, chatgpt_account_id, etc.) from id_token in credentials.
// Only applies to OpenAI OAuth accounts. Skips expired token errors silently.
// Existing credential values are never overwritten — only missing fields are filled.
func enrichCredentialsFromIDToken(item *DataAccount) {
	if item.Credentials == nil {
		return
	}
	// Only enrich OpenAI OAuth accounts
	platform := strings.ToLower(strings.TrimSpace(item.Platform))
	if platform != service.PlatformOpenAI {
		return
	}
	if strings.ToLower(strings.TrimSpace(item.Type)) != service.AccountTypeOAuth {
		return
	}

	idToken, _ := item.Credentials["id_token"].(string)
	if strings.TrimSpace(idToken) == "" {
		return
	}

	// DecodeIDToken skips expiry validation — safe for imported data
	claims, err := openai.DecodeIDToken(idToken)
	if err != nil {
		slog.Debug("import_enrich_id_token_decode_failed", "account", item.Name, "error", err)
		return
	}

	userInfo := claims.GetUserInfo()
	if userInfo == nil {
		return
	}

	// Fill missing fields only (never overwrite existing values)
	setIfMissing := func(key, value string) {
		if value == "" {
			return
		}
		if existing, _ := item.Credentials[key].(string); existing == "" {
			item.Credentials[key] = value
		}
	}

	setIfMissing("email", userInfo.Email)
	setIfMissing("plan_type", userInfo.PlanType)
	setIfMissing("chatgpt_account_id", userInfo.ChatGPTAccountID)
	setIfMissing("chatgpt_user_id", userInfo.ChatGPTUserID)
	setIfMissing("organization_id", userInfo.OrganizationID)
}

func normalizeProxyStatus(status string) string {
	normalized := strings.TrimSpace(strings.ToLower(status))
	switch normalized {
	case "":
		return ""
	case service.StatusActive:
		return service.StatusActive
	case "inactive", service.StatusDisabled:
		return "inactive"
	default:
		return normalized
	}
}

// parseDataAccountCreatedAt 兼容多种 created_at 形态：
//   - int64/float64：按 Unix 秒解析
//   - 字符串："1700000000"（秒）/ ISO 8601（time.RFC3339）/ 部分常见日期格式
//
// 仅在导入时使用，解析失败返回 nil 让 ent 默认填充当前时间。
func parseDataAccountCreatedAt(value any) *time.Time {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case float64:
		if v <= 0 {
			return nil
		}
		// 秒或毫秒：>= 10^12 视为毫秒
		var t time.Time
		if v >= 1e12 {
			t = time.Unix(0, int64(v)*int64(time.Millisecond))
		} else {
			t = time.Unix(int64(v), 0)
		}
		return &t
	case int64:
		if v <= 0 {
			return nil
		}
		var t time.Time
		if v >= 1e12 {
			t = time.Unix(0, v*int64(time.Millisecond))
		} else {
			t = time.Unix(v, 0)
		}
		return &t
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil
		}
		// 整数字符串
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return parseDataAccountCreatedAt(n)
		}
		layouts := []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02T15:04:05Z07:00",
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, s); err == nil {
				return &t
			}
		}
		return nil
	default:
		return nil
	}
}

// ----------------------------------------------------------------------------
// 回填账号 created_at（按 email 匹配）
// ----------------------------------------------------------------------------

// BackfillCreatedAtItem 描述单条回填项。
//   - email：用于匹配账号；与 credentials.email、extra.email、name 进行不区分大小写的比对。
//   - created_at：兼容秒级 Unix 时间戳与 ISO 8601 字符串。
type BackfillCreatedAtItem struct {
	Email     string `json:"email"`
	CreatedAt any    `json:"created_at"`
}

// BackfillCreatedAtRequest 是回填接口的请求体。
type BackfillCreatedAtRequest struct {
	Items []BackfillCreatedAtItem `json:"items"`
	// OverrideExisting 为 true 时强制覆盖已有的 created_at；
	// 默认仅在原值缺失或与 updated_at 在 2 秒内一致（视为导入瞬间填的占位时间）时才回填。
	OverrideExisting bool `json:"override_existing"`
}

// BackfillCreatedAtResultItem 描述单条回填结果。
type BackfillCreatedAtResultItem struct {
	Email     string `json:"email"`
	AccountID int64  `json:"account_id,omitempty"`
	Updated   bool   `json:"updated"`
	Skipped   bool   `json:"skipped,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// BackfillCreatedAtResult 是回填接口的响应体。
type BackfillCreatedAtResult struct {
	Total   int                           `json:"total"`
	Updated int                           `json:"updated"`
	Skipped int                           `json:"skipped"`
	Failed  int                           `json:"failed"`
	Items   []BackfillCreatedAtResultItem `json:"items"`
}

// BackfillCreatedAt 提供按 email 批量回填账号 created_at 的能力。
// 用于已有数据补齐源端真实创建时间，避免在管理界面看到错误的导入时间。
func (h *AccountHandler) BackfillCreatedAt(c *gin.Context) {
	var req BackfillCreatedAtRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if len(req.Items) == 0 {
		response.BadRequest(c, "items is required")
		return
	}
	if len(req.Items) > 5000 {
		response.BadRequest(c, "items size exceeds limit (5000)")
		return
	}

	ctx := c.Request.Context()
	result := h.backfillCreatedAt(ctx, req)
	response.Success(c, result)
}

func (h *AccountHandler) backfillCreatedAt(ctx context.Context, req BackfillCreatedAtRequest) BackfillCreatedAtResult {
	out := BackfillCreatedAtResult{Total: len(req.Items), Items: make([]BackfillCreatedAtResultItem, 0, len(req.Items))}

	updater, ok := h.adminService.(interface {
		BackfillAccountCreatedAtByEmail(ctx context.Context, email string, createdAt time.Time, overrideExisting bool) (int64, bool, string, error)
	})
	if !ok {
		out.Failed = len(req.Items)
		for _, item := range req.Items {
			out.Items = append(out.Items, BackfillCreatedAtResultItem{
				Email:  strings.TrimSpace(item.Email),
				Reason: "backfill not supported by current admin service",
			})
		}
		return out
	}

	for _, item := range req.Items {
		email := strings.TrimSpace(item.Email)
		entry := BackfillCreatedAtResultItem{Email: email}
		if email == "" {
			entry.Reason = "email is required"
			out.Failed++
			out.Items = append(out.Items, entry)
			continue
		}
		ts := parseDataAccountCreatedAt(item.CreatedAt)
		if ts == nil {
			entry.Reason = "invalid created_at"
			out.Failed++
			out.Items = append(out.Items, entry)
			continue
		}
		id, updated, reason, err := updater.BackfillAccountCreatedAtByEmail(ctx, email, *ts, req.OverrideExisting)
		entry.AccountID = id
		if err != nil {
			entry.Reason = err.Error()
			out.Failed++
			out.Items = append(out.Items, entry)
			continue
		}
		if updated {
			entry.Updated = true
			out.Updated++
		} else {
			entry.Skipped = true
			entry.Reason = reason
			out.Skipped++
		}
		out.Items = append(out.Items, entry)
	}
	return out
}
