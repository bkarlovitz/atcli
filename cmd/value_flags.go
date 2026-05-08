package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

func parseRecordValueFlags(setValues, setJSONValues []string) (map[string]any, error) {
	values := make(map[string]any, len(setValues)+len(setJSONValues))

	for _, raw := range setValues {
		name, value, err := splitValueFlag(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid --set %q: %w", raw, err)
		}
		if _, exists := values[name]; exists {
			return nil, fmt.Errorf("duplicate attribute %q", name)
		}
		values[name] = value
	}

	for _, raw := range setJSONValues {
		name, rawJSON, err := splitValueFlag(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid --set-json %q: %w", raw, err)
		}
		if _, exists := values[name]; exists {
			return nil, fmt.Errorf("duplicate attribute %q", name)
		}

		value, err := decodeJSONValue(rawJSON)
		if err != nil {
			return nil, fmt.Errorf("invalid JSON for attribute %q: %w", name, err)
		}
		values[name] = value
	}

	return values, nil
}

func splitValueFlag(raw string) (string, string, error) {
	name, value, ok := strings.Cut(raw, "=")
	if !ok {
		return "", "", errors.New("expected attr=value")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", errors.New("attribute name cannot be empty")
	}
	return name, value, nil
}

func decodeJSONValue(raw string) (any, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, errors.New("multiple JSON values")
		}
		return nil, err
	}

	return value, nil
}
