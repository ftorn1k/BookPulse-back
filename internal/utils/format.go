package utils

func SplitCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			if i > start {
				parts = append(parts, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

func NullIfZero(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

func MaturityToAge(m string) string {
	if m == "MATURE" {
		return "18+"
	}
	return ""
}

func IsValidStatus(s string) bool {
	switch s {
	case "planned", "reading", "finished", "dropped":
		return true
	default:
		return false
	}
}