package phoneutil

import "strings"

// NormalizeRU приводит ввод к виду +7XXXXXXXXXX (как на бэкенде в валидации).
func NormalizeRU(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, " ", ""))
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "+7") {
		return s
	}
	if len(s) == 11 && s[0] == '8' {
		return "+7" + s[1:]
	}
	if len(s) == 10 && s[0] == '9' {
		return "+7" + s
	}
	return s
}
