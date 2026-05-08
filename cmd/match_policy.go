package cmd

import (
	"fmt"
	"strings"
)

var safeDefaultMatchAttributes = map[string]string{
	"companies":  "domains",
	"people":     "email_addresses",
	"users":      "primary_email_address",
	"workspaces": "workspace_id",
}

func resolveRecordMatchAttribute(object, explicitMatch string) (string, bool, error) {
	match := strings.TrimSpace(explicitMatch)
	if match != "" {
		return match, false, nil
	}

	defaultMatch, ok := safeDefaultMatchAttributes[object]
	if ok {
		return defaultMatch, true, nil
	}

	return "", false, fmt.Errorf("--match is required for object %q; safe defaults exist only for companies, people, users, and workspaces", object)
}
