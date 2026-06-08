//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountTestService_QuickProbeOpenAI402DeactivatedWorkspace(t *testing.T) {
	body := `{"detail":{"code":"deactivated_workspace"}}`
	resp := newJSONResponse(http.StatusPaymentRequired, body)

	account := &Account{
		ID:          101,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}
	repo := &openAIAccountTestRepo{
		mockAccountRepoForGemini: mockAccountRepoForGemini{
			accountsByID: map[int64]*Account{101: account},
		},
	}
	svc := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: &queuedHTTPUpstream{responses: []*http.Response{resp}},
	}

	result, probedAccount, err := svc.QuickProbeAccountConnection(context.Background(), 101, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, probedAccount)
	require.False(t, result.Success)
	require.Equal(t, http.StatusPaymentRequired, result.StatusCode)
	require.Contains(t, result.Message, "402")
	require.Contains(t, string(result.Body), "deactivated_workspace")
}

func TestAccountTestService_QuickProbeOpenAI200MarksSuccess(t *testing.T) {
	body := `data: {"type":"response.output_text.delta","delta":"hi"}` + "\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	account := &Account{
		ID:          102,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusError,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}
	repo := &openAIAccountTestRepo{
		mockAccountRepoForGemini: mockAccountRepoForGemini{
			accountsByID: map[int64]*Account{102: account},
		},
	}
	svc := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: &queuedHTTPUpstream{responses: []*http.Response{resp}},
	}

	result, _, err := svc.QuickProbeAccountConnection(context.Background(), 102, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Success)
	require.Equal(t, http.StatusOK, result.StatusCode)
}
