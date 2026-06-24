package cache

import "strings"

func Key(parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			clean = append(clean, strings.Trim(p, ":"))
		}
	}
	return strings.Join(clean, ":")
}
