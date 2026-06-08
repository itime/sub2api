package admin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDataAccountFromCPARaw(t *testing.T) {
	raw := map[string]any{
		"access_token":  "at-test",
		"refresh_token": "rt-test",
		"id_token":      "id-test",
		"account_id":    "acct-123",
		"email":         "user@example.com",
		"expired":       "2026-12-31T00:00:00Z",
		"type":          "oauth",
		"source":        "cli_proxy_api",
		"health_status": "ok",
	}

	account, err := dataAccountFromCPARaw(raw, "fallback-name")
	require.NoError(t, err)
	require.Equal(t, "user@example.com", account.Name)
	require.Equal(t, "openai", account.Platform)
	require.Equal(t, "oauth", account.Type)
	require.Equal(t, "at-test", account.Credentials["access_token"])
	require.Equal(t, "acct-123", account.Credentials["chatgpt_account_id"])
	require.NotNil(t, account.ExpiresAt)
	require.Equal(t, "cpa", account.Extra["import_source"])
	require.Equal(t, "cli_proxy_api", account.Extra["source"])
}

func TestParseImportJSONBytes_CPASingleFile(t *testing.T) {
	body := []byte(`{
		"access_token":"at",
		"refresh_token":"rt",
		"email":"cpa@example.com",
		"expired":"2026-01-02T03:04:05Z"
	}`)
	parsed := parseImportJSONBytes(body, "cpa-account.json")
	require.Len(t, parsed.accounts, 1)
	require.Equal(t, "cpa@example.com", parsed.accounts[0].Name)
	require.Empty(t, parsed.skipped)
}

func TestParseImportJSONBytes_Sub2APIBundle(t *testing.T) {
	body := []byte(`{
		"accounts":[{"name":"bundle-user","platform":"openai","type":"oauth","credentials":{"access_token":"at"}}]
	}`)
	parsed := parseImportJSONBytes(body, "bundle.json")
	require.Len(t, parsed.accounts, 1)
	require.Equal(t, "bundle-user", parsed.accounts[0].Name)
}

func TestIsCPARawMap(t *testing.T) {
	require.True(t, isCPARawMap(map[string]any{"access_token": "x"}))
	require.False(t, isCPARawMap(map[string]any{"accounts": []any{map[string]any{"name": "a"}}}))
}
