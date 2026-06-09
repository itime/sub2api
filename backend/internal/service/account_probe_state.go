package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// ApplyProbeResultToAccountState mirrors single-account test status side effects:
// success clears recoverable runtime errors; upstream failures update account state.
func (s *AccountTestService) ApplyProbeResultToAccountState(
	ctx context.Context,
	account *Account,
	result *AccountQuickProbeResult,
	rateLimit *RateLimitService,
) (updated bool, err error) {
	if s == nil || account == nil || result == nil {
		return false, nil
	}

	if result.Success {
		if rateLimit == nil {
			return false, nil
		}
		_, err = rateLimit.RecoverAccountAfterSuccessfulTest(ctx, account.ID)
		return true, err
	}

	if result.StatusCode <= 0 || s.accountRepo == nil {
		return false, nil
	}

	if applied := s.applyDirectProbeAuthErrors(ctx, account, result); applied {
		return true, nil
	}

	if rateLimit != nil {
		headers := result.Headers
		if headers == nil {
			headers = make(http.Header)
		}
		rateLimit.HandleUpstreamError(ctx, account, result.StatusCode, headers, result.Body)
		return true, nil
	}

	return false, nil
}

func (s *AccountTestService) applyDirectProbeAuthErrors(
	ctx context.Context,
	account *Account,
	result *AccountQuickProbeResult,
) bool {
	bodySnippet := truncateProbeMessage(string(result.Body))

	switch result.StatusCode {
	case http.StatusUnauthorized:
		if account.IsOpenAI() {
			errMsg := fmt.Sprintf("Authentication failed (401): %s", bodySnippet)
			if err := s.accountRepo.SetError(ctx, account.ID, errMsg); err != nil {
				return false
			}
			return true
		}
		if isPermanentUpstreamAuthFailure(result.Body) {
			errMsg := fmt.Sprintf("Authentication failed (401): %s", bodySnippet)
			if err := s.accountRepo.SetError(ctx, account.ID, errMsg); err != nil {
				return false
			}
			return true
		}
	case http.StatusForbidden:
		if account.IsOpenAI() {
			errMsg := fmt.Sprintf("API returned 403: %s", bodySnippet)
			if err := s.accountRepo.SetError(ctx, account.ID, errMsg); err != nil {
				return false
			}
			return true
		}
	}

	return false
}

func isPermanentUpstreamAuthFailure(body []byte) bool {
	code := strings.TrimSpace(extractUpstreamErrorCode(body))
	return code == "token_invalidated" || code == "token_revoked"
}
