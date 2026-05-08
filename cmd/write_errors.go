package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"atcli/internal/attio"
)

func classifyRecordWriteError(action string, err error) error {
	if err == nil {
		return nil
	}
	if isTimeoutError(err) {
		return fmt.Errorf("%s failed: network timeout while contacting Attio; retry the request or check connectivity: %w", action, err)
	}

	var apiErr *attio.APIError
	if !errors.As(err, &apiErr) {
		return fmt.Errorf("%s failed: %w", action, err)
	}

	body := strings.ToLower(apiErr.Body)
	switch {
	case apiErr.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("%s failed: Attio rate limit exceeded; retry after a short delay: %w", action, err)
	case isRecordWriteScopeError(apiErr):
		return fmt.Errorf("%s failed: token is missing Attio record write scope (record_permission:read-write): %w", action, err)
	case isNonUniqueMatchError(apiErr):
		return fmt.Errorf("%s failed: matching attribute must be unique for upsert; choose a unique attribute with --match: %w", action, err)
	case apiErr.StatusCode == http.StatusBadRequest || apiErr.StatusCode == http.StatusUnprocessableEntity:
		return fmt.Errorf("%s failed: Attio rejected the record values; check attribute names and value shapes: %w", action, err)
	case apiErr.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("%s failed: Attio rejected the token; run `atcli auth` or set ATTIO_ACCESS_TOKEN: %w", action, err)
	default:
		if strings.Contains(body, "validation") {
			return fmt.Errorf("%s failed: Attio rejected the record values; check attribute names and value shapes: %w", action, err)
		}
		return fmt.Errorf("%s failed: %w", action, err)
	}
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func isRecordWriteScopeError(apiErr *attio.APIError) bool {
	if apiErr.StatusCode != http.StatusForbidden && apiErr.StatusCode != http.StatusUnauthorized {
		return false
	}
	body := strings.ToLower(apiErr.Body)
	return strings.Contains(body, "record_permission:read-write") ||
		(strings.Contains(body, "record") && strings.Contains(body, "write") && strings.Contains(body, "scope"))
}

func isNonUniqueMatchError(apiErr *attio.APIError) bool {
	if apiErr.StatusCode != http.StatusBadRequest && apiErr.StatusCode != http.StatusUnprocessableEntity {
		return false
	}
	body := strings.ToLower(apiErr.Body)
	return strings.Contains(body, "unique") &&
		(strings.Contains(body, "matching_attribute") || strings.Contains(body, "matching attribute") || strings.Contains(body, "match"))
}
