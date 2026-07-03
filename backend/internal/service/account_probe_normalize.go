package service

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// ResolvedProbeStatusCode extracts the best-effort upstream HTTP status from a probe result.
func ResolvedProbeStatusCode(result *AccountQuickProbeResult) int {
	return resolvedProbeStatusCode(result)
}

func resolvedProbeStatusCode(result *AccountQuickProbeResult) int {
	if result == nil {
		return 0
	}
	if result.StatusCode > 0 {
		return result.StatusCode
	}

	if code := parseProbeStatusFromMessage(result.Message); code > 0 {
		return code
	}

	if len(result.Body) == 0 {
		return inferAuthFailureStatus(result.Message)
	}

	if status := int(gjson.GetBytes(result.Body, "status").Int()); status > 0 {
		return status
	}
	if isPermanentUpstreamAuthFailure(result.Body) || probeBodyIndicatesAuthFailure(result.Body) {
		return http.StatusUnauthorized
	}
	if detectQuickProbeOKBodyFailure(result.Body) {
		if isPermanentUpstreamAuthFailure(result.Body) || probeBodyIndicatesAuthFailure(result.Body) {
			return http.StatusUnauthorized
		}
		return http.StatusBadRequest
	}

	return inferAuthFailureStatus(result.Message)
}

func parseProbeStatusFromMessage(message string) int {
	idx := strings.Index(message, "API returned ")
	if idx < 0 {
		return 0
	}
	rest := strings.TrimSpace(message[idx+len("API returned "):])
	end := strings.Index(rest, ":")
	if end <= 0 {
		return 0
	}
	code, err := strconv.Atoi(strings.TrimSpace(rest[:end]))
	if err != nil || code <= 0 {
		return 0
	}
	return code
}

func inferAuthFailureStatus(message string) int {
	lower := strings.ToLower(strings.TrimSpace(message))
	if strings.Contains(lower, "token_invalidated") ||
		strings.Contains(lower, "token_revoked") ||
		strings.Contains(lower, "token has been invalidated") ||
		strings.Contains(lower, "invalid_grant") {
		return http.StatusUnauthorized
	}
	return 0
}

func probeBodyIndicatesAuthFailure(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	lower := strings.ToLower(string(body))
	if strings.Contains(lower, "token_invalidated") ||
		strings.Contains(lower, "token_revoked") ||
		strings.Contains(lower, "token has been invalidated") ||
		strings.Contains(lower, "invalid_grant") ||
		strings.Contains(lower, "authentication token") {
		return true
	}
	code := strings.TrimSpace(extractUpstreamErrorCode(body))
	return code == "token_invalidated" || code == "token_revoked" || code == "invalid_grant"
}

func detectQuickProbeOKBodyFailure(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return false
	}
	if trimmed[0] == '{' {
		return gjson.GetBytes(trimmed, "error").Exists() ||
			isPermanentUpstreamAuthFailure(trimmed) ||
			probeBodyIndicatesAuthFailure(trimmed)
	}
	lower := strings.ToLower(string(trimmed))
	return strings.Contains(lower, "event: error") ||
		strings.Contains(lower, `"type":"error"`) ||
		strings.Contains(lower, `"type":"response.failed"`) ||
		strings.Contains(lower, "token_invalidated") ||
		strings.Contains(lower, "token has been invalidated")
}

func extractQuickProbeOKBodyFailure(body []byte) (statusCode int, failBody []byte, failed bool) {
	if !detectQuickProbeOKBodyFailure(body) {
		return 0, nil, false
	}
	failBody = append([]byte(nil), body...)
	if isPermanentUpstreamAuthFailure(body) || probeBodyIndicatesAuthFailure(body) {
		return http.StatusUnauthorized, failBody, true
	}
	if status := int(gjson.GetBytes(body, "status").Int()); status > 0 {
		return status, failBody, true
	}
	return http.StatusBadRequest, failBody, true
}

func isPermanentUpstreamAuthFailure(body []byte) bool {
	code := strings.TrimSpace(extractUpstreamErrorCode(body))
	return code == "token_invalidated" || code == "token_revoked"
}

func probeResultForStateUpdate(result *AccountQuickProbeResult) *AccountQuickProbeResult {
	if result == nil {
		return nil
	}
	statusCode := resolvedProbeStatusCode(result)
	if statusCode == result.StatusCode {
		return result
	}
	cloned := *result
	cloned.StatusCode = statusCode
	if cloned.Message == "" {
		cloned.Message = fmt.Sprintf("API returned %d: %s", statusCode, truncateProbeMessage(string(result.Body)))
	}
	return &cloned
}
