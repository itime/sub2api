//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type probeStateTrackingRepo struct {
	mockAccountRepoForGemini
	setErrorIDs []int64
}

func (r *probeStateTrackingRepo) SetError(_ context.Context, id int64, _ string) error {
	r.setErrorIDs = append(r.setErrorIDs, id)
	return nil
}

func TestApplyProbeResultToAccountState_OpenAI401SetsError(t *testing.T) {
	body := `{"error":{"message":"Your authentication token has been invalidated. Please try signing in again.","type":"invalid_request_error","code":"token_invalidated","param":null},"status":401}`
	account := &Account{
		ID:          501,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Credentials: map[string]any{"access_token": "token", "refresh_token": "refresh"},
	}
	repo := &probeStateTrackingRepo{
		mockAccountRepoForGemini: mockAccountRepoForGemini{
			accountsByID: map[int64]*Account{501: account},
		},
	}
	svc := &AccountTestService{accountRepo: repo}
	result := &AccountQuickProbeResult{
		Success:    false,
		StatusCode: http.StatusUnauthorized,
		Body:       []byte(body),
	}

	updated, err := svc.ApplyProbeResultToAccountState(context.Background(), account, result, nil)
	require.NoError(t, err)
	require.True(t, updated)
	require.Equal(t, []int64{501}, repo.setErrorIDs)
}

func TestApplyProbeResultToAccountState_Inferred401FromBodySetsError(t *testing.T) {
	body := `{"error":{"message":"Your authentication token has been invalidated.","code":"token_invalidated"},"status":401}`
	account := &Account{
		ID:          502,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Credentials: map[string]any{"access_token": "token", "refresh_token": "refresh"},
	}
	repo := &probeStateTrackingRepo{
		mockAccountRepoForGemini: mockAccountRepoForGemini{
			accountsByID: map[int64]*Account{502: account},
		},
	}
	svc := &AccountTestService{accountRepo: repo}
	result := &AccountQuickProbeResult{
		Success:    false,
		StatusCode: 0,
		Message:    "Request failed: context deadline exceeded",
		Body:       []byte(body),
	}

	updated, err := svc.ApplyProbeResultToAccountState(context.Background(), account, result, nil)
	require.NoError(t, err)
	require.True(t, updated)
	require.Equal(t, []int64{502}, repo.setErrorIDs)
}
