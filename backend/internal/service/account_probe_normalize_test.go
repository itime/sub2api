//go:build unit

package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolvedProbeStatusCode_From401JSONBody(t *testing.T) {
	body := `{"error":{"message":"Your authentication token has been invalidated.","code":"token_invalidated"},"status":401}`
	result := &AccountQuickProbeResult{
		Success:    false,
		StatusCode: 0,
		Message:    "Request failed: unexpected EOF",
		Body:       []byte(body),
	}
	require.Equal(t, http.StatusUnauthorized, ResolvedProbeStatusCode(result))
}

func TestExtractQuickProbeOKBodyFailure_JSON401(t *testing.T) {
	body := []byte(`{"error":{"code":"token_invalidated","message":"invalid"},"status":401}`)
	status, failBody, failed := extractQuickProbeOKBodyFailure(body)
	require.True(t, failed)
	require.Equal(t, http.StatusUnauthorized, status)
	require.Equal(t, body, failBody)
}

func TestDetectQuickProbeOKBodyFailure_SSEError(t *testing.T) {
	body := []byte("event: error\ndata: {\"error\":{\"code\":\"token_invalidated\"}}\n\n")
	require.True(t, detectQuickProbeOKBodyFailure(body))
}

func TestShouldRetryQuickProbe_OpenAIOAuth429(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	result := &AccountQuickProbeResult{Success: false, StatusCode: http.StatusTooManyRequests}
	require.True(t, shouldRetryQuickProbe(account, result))
}

func TestPreferProbeResult_401Over429(t *testing.T) {
	first := &AccountQuickProbeResult{Success: false, StatusCode: http.StatusTooManyRequests}
	retry := &AccountQuickProbeResult{
		Success:    false,
		StatusCode: http.StatusUnauthorized,
		Body:       []byte(`{"error":{"code":"token_invalidated"}}`),
	}
	require.True(t, preferProbeResult(retry, first))
}
