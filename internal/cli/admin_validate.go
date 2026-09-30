package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var macroNameRe = regexp.MustCompile(`^\{\$[A-Z0-9_.]+(:.+)?\}$`)

func oneOf(flag, v string, allowed ...string) error {
	for _, a := range allowed {
		if v == a {
			return nil
		}
	}
	return fmt.Errorf("invalid %s '%s'; expected one of: %s", flag, v, strings.Join(allowed, ", "))
}

func validateMacro(m string) error {
	if !macroNameRe.MatchString(m) {
		return fmt.Errorf("invalid macro '%s'; expected {$NAME} using A-Z, 0-9, _ or .", m)
	}
	return nil
}

func validateMacroType(t int) error {
	if t < 0 || t > 2 {
		return fmt.Errorf("invalid --type %d; expected 0 (text), 1 (secret) or 2 (vault)", t)
	}
	return nil
}

func validateIDs(kind string, ids []string) error {
	if len(ids) == 0 {
		return fmt.Errorf("no %s IDs specified", kind)
	}
	for _, id := range ids {
		if _, err := strconv.ParseUint(id, 10, 64); err != nil {
			return fmt.Errorf("invalid %s ID '%s'", kind, id)
		}
	}
	return nil
}

func idObjects(key string, ids []string) []map[string]string {
	out := make([]map[string]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, map[string]string{key: id})
	}
	return out
}
