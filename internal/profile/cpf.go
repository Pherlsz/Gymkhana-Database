package profile

import (
	"strings"
)

func CanonicalCPFDigits(value string) string {
	var digits strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	return digits.String()
}

func DisplayCPF(canonical string, reveal bool) string {
	digits := CanonicalCPFDigits(canonical)
	if len(digits) != 11 {
		return strings.TrimSpace(canonical)
	}
	if reveal {
		return digits[0:3] + "." + digits[3:6] + "." + digits[6:9] + "-" + digits[9:11]
	}
	return "***.***.***-" + digits[9:11]
}

func DigitSumCPF(canonical string) *int {
	var sum int
	var found bool
	for _, r := range canonical {
		if r >= '0' && r <= '9' {
			sum += int(r - '0')
			found = true
		}
	}
	if !found {
		return nil
	}
	return &sum
}
