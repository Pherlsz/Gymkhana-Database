package profile

import (
	"strings"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
)

func CanonicalCPFDigits(value string) string {
	return normalize.Digits(value)
}

func DisplayCPF(canonical string, reveal bool) string {
	digits := normalize.Digits(canonical)
	if len(digits) != 11 {
		return strings.TrimSpace(canonical)
	}
	if reveal {
		return digits[0:3] + "." + digits[3:6] + "." + digits[6:9] + "-" + digits[9:11]
	}
	return "***.***.***-" + digits[9:11]
}

func DigitSumCPF(canonical string) *int {
	digits := normalize.Digits(canonical)
	if len(digits) == 0 {
		return nil
	}
	var sum int
	for i := 0; i < len(digits); i++ {
		sum += int(digits[i] - '0')
	}
	return &sum
}
