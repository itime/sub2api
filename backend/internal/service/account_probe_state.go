package service

import (
	"context"
	"fmt"
	"net/http"
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

	normalized := probeResultForStateUpdate(result)
	if normalized == nil || s.accountRepo == nil {
		return false, nil
	}

	if applied := s.applyDirectProbeAuthErrors(ctx, account, normalized); applied {
		return true, nil
	}

	statusCode := normalized.StatusCode
	if statusCode <= 0 {
		return false, nil
	}

	if rateLimit != nil {
		headers := normalized.Headers
		if headers == nil {
			headers = make(http.Header)
		}
		// Auth failures must not be masked as short-lived rate limits during batch probes.
		if statusCode == http.StatusTooManyRequests &&
			account.IsOpenAIOAuth() &&
			(probeBodyIndicatesAuthFailure(normalized.Body) || isPermanentUpstreamAuthFailure(normalized.Body)) {
			errMsg := fmt.Sprintf("Authentication failed (401): %s", truncateProbeMessage(string(normalized.Body)))
			if err := s.accountRepo.SetError(ctx, account.ID, errMsg); err != nil {
				return false, err
			}
			return true, nil
		}
		rateLimit.HandleUpstreamError(ctx, account, statusCode, headers, normalized.Body)
		return true, nil
	}

	return false, nil
}

func (s *AccountTestService) applyDirectProbeAuthErrors(
	ctx context.Context,
	account *Account,
	result *AccountQuickProbeResult,
) bool {
	if s == nil || account == nil || result == nil || s.accountRepo == nil {
		return false
	}

	statusCode := resolvedProbeStatusCode(result)
	bodySnippet := truncateProbeMessage(string(result.Body))

	switch {
	case statusCode == http.StatusUnauthorized:
		if account.IsOpenAI() || isPermanentUpstreamAuthFailure(result.Body) || probeBodyIndicatesAuthFailure(result.Body) {
			errMsg := fmt.Sprintf("Authentication failed (401): %s", bodySnippet)
			if err := s.accountRepo.SetError(ctx, account.ID, errMsg); err != nil {
				return false
			}
			return true
		}
	case statusCode == http.StatusForbidden:
		if account.IsOpenAI() {
			errMsg := fmt.Sprintf("API returned 403: %s", bodySnippet)
			if err := s.accountRepo.SetError(ctx, account.ID, errMsg); err != nil {
				return false
			}
			return true
		}
	case account.IsOpenAIOAuth() && probeBodyIndicatesAuthFailure(result.Body):
		errMsg := fmt.Sprintf("Authentication failed (401): %s", bodySnippet)
		if err := s.accountRepo.SetError(ctx, account.ID, errMsg); err != nil {
			return false
		}
		return true
	}

	return false
}
