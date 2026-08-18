package profile

import "testing"

func TestDisplayCPFRevealsFullNumberForAdmin(t *testing.T) {
	if got := DisplayCPF("52998224725", true); got != "529.982.247-25" {
		t.Fatalf("admin display = %q", got)
	}
	if got := DisplayCPF("529.982.247-25", true); got != "529.982.247-25" {
		t.Fatalf("formatted admin display = %q", got)
	}
}

func TestDisplayCPFMasksLastTwoForExternal(t *testing.T) {
	if got := DisplayCPF("52998224725", false); got != "***.***.***-25" {
		t.Fatalf("external display = %q", got)
	}
}

func TestDigitSumCPFUsesCanonicalDigits(t *testing.T) {
	sum := DigitSumCPF("529.982.247-25")
	if sum == nil || *sum != 5+2+9+9+8+2+2+4+7+2+5 {
		t.Fatalf("sum = %v", sum)
	}
	if DigitSumCPF("") != nil {
		t.Fatal("empty CPF should have no digit sum")
	}
}
