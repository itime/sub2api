package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
)

const (
	quickProbeTimeout   = 40 * time.Second
	quickProbeBodyLimit = 8 << 10
)

// AccountQuickProbeResult is the outcome of a lightweight connectivity probe.
type AccountQuickProbeResult struct {
	Success    bool
	StatusCode int
	Message    string
	Headers    http.Header
	Body       []byte
	LatencyMs  int64
}

const quickProbeRetryDelay = 400 * time.Millisecond

// QuickProbeAccountConnection sends a minimal upstream request and returns without
// consuming full SSE streams. Used by batch test to probe many accounts quickly.
func (s *AccountTestService) QuickProbeAccountConnection(ctx context.Context, accountID int64, modelID string) (*AccountQuickProbeResult, *Account, error) {
	result, account, err := s.quickProbeAccountConnectionOnce(ctx, accountID, modelID)
	if !shouldRetryQuickProbe(account, result) {
		return result, account, err
	}

	if err := probeSleepWithContext(ctx, quickProbeRetryDelay); err != nil {
		return result, account, err
	}

	retryResult, retryAccount, retryErr := s.quickProbeAccountConnectionOnce(ctx, accountID, modelID)
	if preferProbeResult(retryResult, result) {
		return retryResult, retryAccount, retryErr
	}
	return result, account, err
}

func (s *AccountTestService) quickProbeAccountConnectionOnce(ctx context.Context, accountID int64, modelID string) (*AccountQuickProbeResult, *Account, error) {
	if s == nil || s.accountRepo == nil {
		return nil, nil, fmt.Errorf("account test service is not configured")
	}

	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, nil, fmt.Errorf("account not found")
	}

	probeCtx, cancel := context.WithTimeout(ctx, quickProbeTimeout)
	defer cancel()

	startedAt := time.Now()

	switch {
	case account.IsOpenAI():
		result, probeErr := s.quickProbeOpenAI(probeCtx, account, modelID)
		if result != nil {
			result.LatencyMs = time.Since(startedAt).Milliseconds()
		}
		return result, account, probeErr
	case account.IsGemini():
		result, probeErr := s.quickProbeGemini(probeCtx, account, modelID)
		if result != nil {
			result.LatencyMs = time.Since(startedAt).Milliseconds()
		}
		return result, account, probeErr
	case account.Platform == PlatformAntigravity:
		result, probeErr := s.quickProbeAntigravity(probeCtx, account, modelID)
		if result != nil {
			result.LatencyMs = time.Since(startedAt).Milliseconds()
		}
		return result, account, probeErr
	case account.IsBedrock() || account.Type == AccountTypeServiceAccount:
		return s.quickProbeViaBackgroundTest(probeCtx, accountID, modelID, account, startedAt)
	default:
		result, probeErr := s.quickProbeClaude(probeCtx, account, modelID)
		if result != nil {
			result.LatencyMs = time.Since(startedAt).Milliseconds()
		}
		return result, account, probeErr
	}
}

func shouldRetryQuickProbe(account *Account, result *AccountQuickProbeResult) bool {
	if account == nil || result == nil || result.Success {
		return false
	}
	if !account.IsOpenAI() {
		return resolvedProbeStatusCode(result) <= 0
	}
	status := resolvedProbeStatusCode(result)
	if status <= 0 {
		return true
	}
	// Concurrent batch probes often hit transient 429s while the account is actually invalid.
	return account.IsOpenAIOAuth() && status == http.StatusTooManyRequests
}

func preferProbeResult(retry, first *AccountQuickProbeResult) bool {
	if retry == nil {
		return false
	}
	if first == nil {
		return true
	}
	retryStatus := resolvedProbeStatusCode(retry)
	firstStatus := resolvedProbeStatusCode(first)
	if retryStatus == http.StatusUnauthorized && firstStatus == http.StatusTooManyRequests {
		return true
	}
	if retryStatus > 0 && firstStatus <= 0 {
		return true
	}
	if probeBodyIndicatesAuthFailure(retry.Body) && !probeBodyIndicatesAuthFailure(first.Body) {
		return true
	}
	if retry.Success && !first.Success {
		return true
	}
	return false
}

func probeSleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *AccountTestService) quickProbeViaBackgroundTest(ctx context.Context, accountID int64, modelID string, account *Account, startedAt time.Time) (*AccountQuickProbeResult, *Account, error) {
	bgResult, err := s.RunTestBackground(ctx, accountID, modelID)
	result := &AccountQuickProbeResult{
		LatencyMs: time.Since(startedAt).Milliseconds(),
	}
	if err != nil {
		result.Success = false
		result.Message = err.Error()
		return result, account, nil
	}
	if bgResult == nil || bgResult.Status != "success" {
		result.Success = false
		if bgResult != nil && bgResult.ErrorMessage != "" {
			result.Message = bgResult.ErrorMessage
		} else {
			result.Message = "probe failed"
		}
		return result, account, nil
	}
	result.Success = true
	if strings.TrimSpace(bgResult.ResponseText) != "" {
		result.Message = truncateProbeMessage(bgResult.ResponseText)
	} else {
		result.Message = "ok"
	}
	return result, account, nil
}

func (s *AccountTestService) quickProbeOpenAI(ctx context.Context, account *Account, modelID string) (*AccountQuickProbeResult, error) {
	testModelID := modelID
	if testModelID == "" {
		testModelID = openai.DefaultTestModel
	}
	testModelID = account.GetMappedModel(testModelID)

	if account.IsOAuth() {
		return s.quickProbeOpenAIOAuth(ctx, account, testModelID)
	}
	if account.Type == AccountTypeAPIKey {
		return s.quickProbeOpenAIAPIKey(ctx, account, testModelID)
	}
	return nil, fmt.Errorf("unsupported OpenAI account type: %s", account.Type)
}

func (s *AccountTestService) quickProbeOpenAIOAuth(ctx context.Context, account *Account, testModelID string) (*AccountQuickProbeResult, error) {
	authToken := account.GetOpenAIAccessToken()
	if authToken == "" {
		return &AccountQuickProbeResult{Success: false, Message: "No access token available"}, nil
	}

	payloadBytes, _ := json.Marshal(createOpenAITestPayload(testModelID, true))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatgptCodexAPIURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Host = "chatgpt.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)
	req.Header.Set("Accept", "text/event-stream")
	if chatgptAccountID := account.GetChatGPTAccountID(); chatgptAccountID != "" {
		req.Header.Set("chatgpt-account-id", chatgptAccountID)
	}

	return s.executeQuickProbe(ctx, account, req)
}

func (s *AccountTestService) quickProbeOpenAIAPIKey(ctx context.Context, account *Account, testModelID string) (*AccountQuickProbeResult, error) {
	authToken := account.GetOpenAIApiKey()
	if authToken == "" {
		return &AccountQuickProbeResult{Success: false, Message: "No API key available"}, nil
	}

	baseURL := account.GetOpenAIBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	normalizedBaseURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return &AccountQuickProbeResult{Success: false, Message: fmt.Sprintf("Invalid base URL: %s", err.Error())}, nil
	}

	var apiURL string
	var payloadBytes []byte
	if openai_compat.ShouldUseResponsesAPI(account.Extra) {
		apiURL = buildOpenAIResponsesURL(normalizedBaseURL)
		payloadBytes, _ = json.Marshal(createOpenAITestPayload(testModelID, false))
	} else {
		apiURL = buildOpenAIChatCompletionsURL(normalizedBaseURL)
		payloadBytes, _ = json.Marshal(createOpenAIChatCompletionsTestPayload(testModelID, ""))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+authToken)

	return s.executeQuickProbe(ctx, account, req)
}

func (s *AccountTestService) quickProbeClaude(ctx context.Context, account *Account, modelID string) (*AccountQuickProbeResult, error) {
	testModelID := modelID
	if testModelID == "" {
		testModelID = claude.DefaultTestModel
	}
	if account.Type == AccountTypeAPIKey {
		testModelID = account.GetMappedModel(testModelID)
	}

	var authToken string
	var useBearer bool
	var apiURL string

	switch {
	case account.IsOAuth():
		useBearer = true
		apiURL = testClaudeAPIURL
		authToken = account.GetCredential("access_token")
	case account.Type == AccountTypeAPIKey:
		authToken = account.GetCredential("api_key")
		baseURL := account.GetBaseURL()
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		normalizedBaseURL, err := s.validateUpstreamBaseURL(baseURL)
		if err != nil {
			return &AccountQuickProbeResult{Success: false, Message: fmt.Sprintf("Invalid base URL: %s", err.Error())}, nil
		}
		apiURL = strings.TrimSuffix(normalizedBaseURL, "/") + "/v1/messages?beta=true"
	default:
		return nil, fmt.Errorf("unsupported Claude account type: %s", account.Type)
	}

	if authToken == "" {
		return &AccountQuickProbeResult{Success: false, Message: "No credentials available"}, nil
	}

	payload, err := createTestPayload(testModelID)
	if err != nil {
		return nil, err
	}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	for key, value := range claude.DefaultHeaders {
		req.Header.Set(key, value)
	}
	if useBearer {
		req.Header.Set("anthropic-beta", claude.DefaultBetaHeader)
		req.Header.Set("Authorization", "Bearer "+authToken)
	} else {
		req.Header.Set("anthropic-beta", claude.APIKeyBetaHeader)
		req.Header.Set("x-api-key", authToken)
	}

	return s.executeQuickProbe(ctx, account, req)
}

func (s *AccountTestService) quickProbeGemini(ctx context.Context, account *Account, modelID string) (*AccountQuickProbeResult, error) {
	testModelID := modelID
	if testModelID == "" {
		testModelID = geminicli.DefaultTestModel
	}
	if account.Type == AccountTypeAPIKey || account.Type == AccountTypeServiceAccount {
		mapping := account.GetModelMapping()
		if mappedModel, exists := mapping[testModelID]; exists {
			testModelID = mappedModel
		}
	}

	payload := createGeminiTestPayload(testModelID, defaultGeminiTextTestPrompt)
	payloadBytes, _ := json.Marshal(payload)

	var req *http.Request
	var err error
	switch account.Type {
	case AccountTypeAPIKey:
		req, err = s.buildGeminiAPIKeyRequest(ctx, account, testModelID, payloadBytes)
	case AccountTypeOAuth:
		req, err = s.buildGeminiOAuthRequest(ctx, account, testModelID, payloadBytes)
	case AccountTypeServiceAccount:
		req, err = s.buildGeminiServiceAccountRequest(ctx, account, testModelID, payloadBytes)
	default:
		return nil, fmt.Errorf("unsupported Gemini account type: %s", account.Type)
	}
	if err != nil {
		return &AccountQuickProbeResult{Success: false, Message: err.Error()}, nil
	}

	return s.executeQuickProbe(ctx, account, req)
}

func (s *AccountTestService) quickProbeAntigravity(ctx context.Context, account *Account, modelID string) (*AccountQuickProbeResult, error) {
	if s.antigravityGatewayService == nil {
		return &AccountQuickProbeResult{Success: false, Message: "Antigravity gateway service not configured"}, nil
	}

	testModelID := modelID
	if testModelID == "" {
		testModelID = "claude-sonnet-4-5"
	}

	result, err := s.antigravityGatewayService.TestConnection(ctx, account, testModelID)
	if err != nil {
		return &AccountQuickProbeResult{Success: false, Message: err.Error()}, nil
	}
	message := "ok"
	if result != nil && strings.TrimSpace(result.Text) != "" {
		message = truncateProbeMessage(result.Text)
	}
	return &AccountQuickProbeResult{Success: true, StatusCode: http.StatusOK, Message: message}, nil
}

func (s *AccountTestService) executeQuickProbe(ctx context.Context, account *Account, req *http.Request) (*AccountQuickProbeResult, error) {
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	if err != nil {
		return &AccountQuickProbeResult{Success: false, Message: fmt.Sprintf("Request failed: %s", err.Error())}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, quickProbeBodyLimit))

	if account.IsOpenAI() && account.IsOAuth() && s.accountRepo != nil {
		if updates, extractErr := extractOpenAICodexProbeUpdates(resp); extractErr == nil && len(updates) > 0 {
			_ = s.accountRepo.UpdateExtra(ctx, account.ID, updates)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			s.reconcileOpenAI429State(ctx, account, resp.Header, body)
		}
	}

	if resp.StatusCode == http.StatusOK {
		if failStatus, failBody, isFailure := extractQuickProbeOKBodyFailure(body); isFailure {
			return &AccountQuickProbeResult{
				Success:    false,
				StatusCode: failStatus,
				Message:    fmt.Sprintf("API returned %d: %s", failStatus, truncateProbeMessage(string(failBody))),
				Headers:    resp.Header.Clone(),
				Body:       failBody,
			}, nil
		}
		message := "ok"
		if len(strings.TrimSpace(string(body))) > 0 {
			message = truncateProbeMessage(string(body))
		}
		return &AccountQuickProbeResult{
			Success:    true,
			StatusCode: resp.StatusCode,
			Message:    message,
			Headers:    resp.Header.Clone(),
			Body:       body,
		}, nil
	}

	return &AccountQuickProbeResult{
		Success:    false,
		StatusCode: resp.StatusCode,
		Message:    fmt.Sprintf("API returned %d: %s", resp.StatusCode, string(body)),
		Headers:    resp.Header.Clone(),
		Body:       body,
	}, nil
}

func truncateProbeMessage(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) <= 160 {
		return raw
	}
	return raw[:160] + "..."
}
