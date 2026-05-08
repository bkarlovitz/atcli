package cmd

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"atcli/internal/attio"
)

func TestClassifyRecordWriteAPIErrorMessages(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		status     string
		body       string
		want       []string
	}{
		{
			name:       "missing record write scope",
			statusCode: http.StatusForbidden,
			status:     "403 Forbidden",
			body:       `{"error":"missing record_permission:read-write"}`,
			want:       []string{"missing Attio record write scope", "record_permission:read-write", "403 Forbidden"},
		},
		{
			name:       "validation failure",
			statusCode: http.StatusBadRequest,
			status:     "400 Bad Request",
			body:       `{"error":"name is required"}`,
			want:       []string{"Attio rejected the record values", "name is required", "400 Bad Request"},
		},
		{
			name:       "non unique matching attribute",
			statusCode: http.StatusUnprocessableEntity,
			status:     "422 Unprocessable Entity",
			body:       `{"error":"matching_attribute must be unique"}`,
			want:       []string{"matching attribute must be unique", "matching_attribute must be unique", "422 Unprocessable Entity"},
		},
		{
			name:       "rate limit",
			statusCode: http.StatusTooManyRequests,
			status:     "429 Too Many Requests",
			body:       `{"error":"rate limit exceeded"}`,
			want:       []string{"rate limit exceeded", "retry after a short delay", "429 Too Many Requests"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := &attio.APIError{
				StatusCode: tt.statusCode,
				Status:     tt.status,
				Body:       tt.body,
			}
			err := classifyRecordWriteError("upsert record", fmt.Errorf("assert record: %w", apiErr))
			for _, want := range tt.want {
				assertErrorContains(t, err, want)
			}
		})
	}
}

func TestClassifyRecordWriteTimeout(t *testing.T) {
	err := classifyRecordWriteError("create record", fmt.Errorf("send request: %w", context.DeadlineExceeded))

	assertErrorContains(t, err, "network timeout")
	assertErrorContains(t, err, "create record")
}
