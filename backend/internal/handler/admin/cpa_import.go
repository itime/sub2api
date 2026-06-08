package admin

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// cliProxyAPI (CPA) flat account JSON — see field mapping in Nitmi/CliProxyAPI-2-Sub2API.
var cpaCredentialKeyMap = map[string]string{
	"access_token":  "access_token",
	"id_token":      "id_token",
	"refresh_token": "refresh_token",
	"account_id":    "chatgpt_account_id",
	"email":         "email",
}

var cpaExtraFieldKeys = map[string]struct{}{
	"password":              {},
	"source":                {},
	"type":                  {},
	"disabled":              {},
	"mailbox":               {},
	"mail_provider":         {},
	"health_status":         {},
	"backup_written":        {},
	"cpa_sync_status":       {},
	"created_at":            {},
	"last_cpa_sync_at":      {},
	"last_cpa_sync_error":   {},
	"last_probe_at":         {},
	"last_probe_detail":     {},
	"last_probe_result":     {},
	"last_probe_status_code": {},
	"last_refresh":          {},
}

func isCPARawMap(raw map[string]any) bool {
	if raw == nil {
		return false
	}
	if accounts, ok := raw["accounts"]; ok {
		if list, ok := accounts.([]any); ok && len(list) > 0 {
			return false
		}
	}
	_, hasAccess := raw["access_token"]
	_, hasRefresh := raw["refresh_token"]
	return hasAccess || hasRefresh
}

func parseCPAExpiredToUnix(value any) *int64 {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil
		}
		if digits := strings.TrimSpace(s); digits != "" {
			if allDigits(digits) {
				if ts, err := strconv.ParseInt(digits, 10, 64); err == nil && ts > 0 {
					return &ts
				}
			}
		}
		layouts := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05Z",
			"2006-01-02T15:04:05.000Z",
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, s); err == nil {
				unix := t.Unix()
				return &unix
			}
		}
	case float64:
		if v > 0 {
			unix := int64(v)
			return &unix
		}
	case int64:
		if v > 0 {
			return &v
		}
	}
	return nil
}

func dataAccountFromCPARaw(raw map[string]any, nameHint string) (DataAccount, error) {
	credentials := make(map[string]any)
	for cpaKey, subKey := range cpaCredentialKeyMap {
		if value, ok := raw[cpaKey]; ok && !isEmptyImportValue(value) {
			credentials[subKey] = value
		}
	}
	if expired, ok := raw["expired"]; ok {
		if unix := parseCPAExpiredToUnix(expired); unix != nil {
			credentials["expires_at"] = *unix
		}
	}
	if len(credentials) == 0 {
		return DataAccount{}, fmt.Errorf("cpa account missing access_token or refresh_token")
	}

	name := ""
	if email, _ := raw["email"].(string); strings.TrimSpace(email) != "" {
		name = strings.TrimSpace(email)
	} else if email, _ := credentials["email"].(string); strings.TrimSpace(email) != "" {
		name = strings.TrimSpace(email)
	}
	if name == "" {
		name = strings.TrimSpace(nameHint)
	}
	if name == "" {
		return DataAccount{}, fmt.Errorf("cpa account name is required")
	}

	extra := make(map[string]any)
	for key, value := range raw {
		if _, mapped := cpaCredentialKeyMap[key]; mapped {
			continue
		}
		if key == "expired" {
			continue
		}
		if _, keep := cpaExtraFieldKeys[key]; keep && !isEmptyImportValue(value) {
			extra[key] = value
		}
	}
	if _, ok := extra["import_source"]; !ok {
		extra["import_source"] = "cpa"
	}

	platform := service.PlatformOpenAI
	if rawPlatform, _ := raw["platform"].(string); strings.TrimSpace(rawPlatform) != "" {
		platform = strings.TrimSpace(rawPlatform)
	}

	accountType := service.AccountTypeOAuth
	if rawType, _ := raw["type"].(string); strings.TrimSpace(rawType) != "" {
		normalized := strings.ToLower(strings.TrimSpace(rawType))
		switch normalized {
		case "codex", "":
			accountType = service.AccountTypeOAuth
		case service.AccountTypeOAuth, service.AccountTypeSetupToken, service.AccountTypeAPIKey, service.AccountTypeUpstream:
			accountType = normalized
		default:
			accountType = service.AccountTypeOAuth
		}
	}

	concurrency := 3
	if v, ok := raw["concurrency"].(float64); ok && v >= 0 {
		concurrency = int(v)
	}
	priority := 50
	if v, ok := raw["priority"].(float64); ok && v >= 0 {
		priority = int(v)
	}

	item := DataAccount{
		Name:        name,
		Platform:    platform,
		Type:        accountType,
		Credentials: credentials,
		Extra:       extra,
		Concurrency: concurrency,
		Priority:    priority,
	}
	rate := 1.0
	item.RateMultiplier = &rate
	autoPause := true
	item.AutoPauseOnExpired = &autoPause

	if unix := parseCPAExpiredToUnix(raw["expired"]); unix != nil {
		item.ExpiresAt = unix
	} else if credExp, ok := credentials["expires_at"]; ok {
		if unix := parseCPAExpiredToUnix(credExp); unix != nil {
			item.ExpiresAt = unix
		}
	}

	if createdAt := parseDataAccountCreatedAt(raw["created_at"]); createdAt != nil {
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	} else if createdAt := parseDataAccountCreatedAt(extra["created_at"]); createdAt != nil {
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	}

	normalizeImportAccount(&item)
	return item, nil
}

func applyCPAImportNormalizations(item *DataAccount) {
	if item == nil {
		return
	}
	if item.Credentials == nil {
		item.Credentials = map[string]any{}
	}
	for cpaKey, subKey := range cpaCredentialKeyMap {
		if existing, _ := item.Credentials[subKey].(string); strings.TrimSpace(existing) != "" {
			continue
		}
		if value, ok := item.Credentials[cpaKey]; ok && !isEmptyImportValue(value) {
			item.Credentials[subKey] = value
		}
	}
	if item.ExpiresAt == nil {
		if unix := parseCPAExpiredToUnix(item.Credentials["expired"]); unix != nil {
			item.ExpiresAt = unix
		}
	}
	if item.Extra == nil {
		item.Extra = map[string]any{}
	}
	if _, ok := item.Extra["import_source"]; !ok {
		if _, hasAccess := item.Credentials["access_token"]; hasAccess {
			item.Extra["import_source"] = "cpa"
		}
	}
	if item.RateMultiplier == nil {
		rate := 1.0
		item.RateMultiplier = &rate
	}
	if item.AutoPauseOnExpired == nil {
		autoPause := true
		item.AutoPauseOnExpired = &autoPause
	}
	if item.Concurrency <= 0 {
		item.Concurrency = 3
	}
	if item.Priority <= 0 {
		item.Priority = 50
	}
	if strings.TrimSpace(item.Platform) == "" {
		item.Platform = service.PlatformOpenAI
	}
}

func isEmptyImportValue(value any) bool {
	if value == nil {
		return true
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	default:
		return false
	}
}

func resolveImportEntryName(entryName string, raw map[string]any) string {
	if email, _ := raw["email"].(string); strings.TrimSpace(email) != "" {
		return strings.TrimSpace(email)
	}
	base := strings.TrimSuffix(filepath.Base(entryName), filepath.Ext(entryName))
	return strings.TrimSpace(base)
}

func shouldSkipArchiveEntry(name string) bool {
	normalized := strings.ReplaceAll(name, "\\", "/")
	lower := strings.ToLower(normalized)
	if strings.HasPrefix(lower, "__macosx/") || strings.Contains(lower, "/.") {
		return true
	}
	base := strings.ToLower(filepath.Base(normalized))
	if base == "" || base == ".ds_store" {
		return true
	}
	return !strings.HasSuffix(base, ".json")
}

type importJSONParseResult struct {
	accounts []DataAccount
	skipped  []DataImportItem
}

func parseImportJSONBytes(data []byte, entryName string) importJSONParseResult {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return importJSONParseResult{
			skipped: []DataImportItem{{
				Kind:    "account",
				Name:    entryName,
				Action:  "skipped",
				Message: "empty file",
			}},
		}
	}

	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return importJSONParseResult{
			skipped: []DataImportItem{{
				Kind:    "account",
				Name:    entryName,
				Action:  "skipped",
				Message: "invalid json: " + err.Error(),
			}},
		}
	}

	return parseImportJSONValue(parsed, entryName)
}

func parseImportJSONValue(parsed any, entryName string) importJSONParseResult {
	switch typed := parsed.(type) {
	case []any:
		out := importJSONParseResult{}
		for idx, item := range typed {
			label := fmt.Sprintf("%s#%d", entryName, idx+1)
			chunk := parseImportJSONValue(item, label)
			out.accounts = append(out.accounts, chunk.accounts...)
			out.skipped = append(out.skipped, chunk.skipped...)
		}
		return out
	case map[string]any:
		if accounts, ok := typed["accounts"].([]any); ok {
			payload := map[string]any{"accounts": accounts}
			if proxies, ok := typed["proxies"].([]any); ok {
				payload["proxies"] = proxies
			}
			return parseImportJSONValue(payload, entryName)
		}
		if isCPARawMap(typed) {
			account, err := dataAccountFromCPARaw(typed, resolveImportEntryName(entryName, typed))
			if err != nil {
				return importJSONParseResult{
					skipped: []DataImportItem{{
						Kind:    "account",
						Name:    entryName,
						Action:  "skipped",
						Message: err.Error(),
					}},
				}
			}
			return importJSONParseResult{accounts: []DataAccount{account}}
		}
		account, err := dataAccountFromStructuredRaw(typed, entryName)
		if err != nil {
			return importJSONParseResult{
				skipped: []DataImportItem{{
					Kind:    "account",
					Name:    entryName,
					Action:  "skipped",
					Message: err.Error(),
				}},
			}
		}
		return importJSONParseResult{accounts: []DataAccount{account}}
	default:
		return importJSONParseResult{
			skipped: []DataImportItem{{
				Kind:    "account",
				Name:    entryName,
				Action:  "skipped",
				Message: "unsupported json root type",
			}},
		}
	}
}

func dataAccountFromStructuredRaw(raw map[string]any, entryName string) (DataAccount, error) {
	rawBytes, err := json.Marshal(raw)
	if err != nil {
		return DataAccount{}, err
	}
	var item DataAccount
	if err := json.Unmarshal(rawBytes, &item); err != nil {
		return DataAccount{}, err
	}
	if strings.TrimSpace(item.Name) == "" {
		item.Name = resolveImportEntryName(entryName, raw)
	}
	if item.Credentials == nil {
		item.Credentials = map[string]any{}
	}
	for cpaKey, subKey := range cpaCredentialKeyMap {
		if value, ok := raw[cpaKey]; ok && !isEmptyImportValue(value) {
			if existing, _ := item.Credentials[subKey].(string); existing == "" {
				item.Credentials[subKey] = value
			}
		}
	}
	if item.ExpiresAt == nil {
		if unix := parseCPAExpiredToUnix(raw["expired"]); unix != nil {
			item.ExpiresAt = unix
		}
	}
	normalizeImportAccount(&item)
	if err := validateDataAccount(item); err != nil {
		return DataAccount{}, err
	}
	return item, nil
}

func parseArchiveAccounts(reader io.Reader, archiveName string, isGzip bool) ([]DataAccount, []DataImportItem, error) {
	if isGzip {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return nil, nil, fmt.Errorf("open gzip archive: %w", err)
		}
		defer func() { _ = gz.Close() }()
		body, err := io.ReadAll(io.LimitReader(gz, 32<<20))
		if err != nil {
			return nil, nil, fmt.Errorf("read gzip archive: %w", err)
		}
		parsed := parseImportJSONBytes(body, archiveName)
		return parsed.accounts, parsed.skipped, nil
	}

	// ZIP handled in account_archive_import.go
	return nil, nil, fmt.Errorf("unsupported archive reader for %s", archiveName)
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
