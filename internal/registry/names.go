package registry

import (
	"fmt"
	"strings"
	"unicode"
)

const maxNameLen = 64

func normalizeName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", nil
	}
	if len(name) > maxNameLen {
		return "", fmt.Errorf("name %q is too long (max %d characters)", name, maxNameLen)
	}
	for _, r := range name {
		if isAllowedNameRune(r) {
			continue
		}
		return "", fmt.Errorf("name %q contains invalid character %q (allowed: letters, digits, '.', '-', '_')", name, r)
	}
	return name, nil
}

func isAllowedNameRune(r rune) bool {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return true
	}
	switch r {
	case '-', '_', '.':
		return true
	default:
		return false
	}
}
