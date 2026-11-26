package captcha

import (
	"fmt"
	"testing"
	"time"
)

func TestIssueAndVerify(t *testing.T) {
	s := NewStore(5*time.Minute, 0) // no min time for this test
	ch := s.Issue()
	if ch.CaptchaID == "" || ch.Question == "" {
		t.Fatal("expected non-empty challenge")
	}
	// Extract numbers from "Ile to A + B?"
	var a, b int
	_, err := fmt.Sscanf(ch.Question, "Ile to %d + %d?", &a, &b)
	if err != nil {
		t.Fatalf("cannot parse question %q: %v", ch.Question, err)
	}
	answer := itoa(a + b)
	if err := s.Verify(ch.CaptchaID, answer); err != nil {
		t.Fatalf("expected valid answer to pass, got: %v", err)
	}
}

func TestVerifyWrongAnswer(t *testing.T) {
	s := NewStore(5*time.Minute, 0)
	ch := s.Issue()
	if err := s.Verify(ch.CaptchaID, "999999"); err == nil {
		t.Fatal("expected wrong answer to fail")
	}
}

func TestVerifyUnknownID(t *testing.T) {
	s := NewStore(5*time.Minute, 0)
	if err := s.Verify("nonexistent", "1"); err == nil {
		t.Fatal("expected unknown captcha_id to fail")
	}
}

func TestSingleUse(t *testing.T) {
	s := NewStore(5*time.Minute, 0)
	ch := s.Issue()
	var a, b int
	fmt.Sscanf(ch.Question, "Ile to %d + %d?", &a, &b)
	answer := itoa(a + b)
	if err := s.Verify(ch.CaptchaID, answer); err != nil {
		t.Fatalf("first verify should pass: %v", err)
	}
	if err := s.Verify(ch.CaptchaID, answer); err == nil {
		t.Fatal("second verify with same ID should fail (single-use)")
	}
}

func TestTimeTrap(t *testing.T) {
	s := NewStore(5*time.Minute, 2*time.Second)
	ch := s.Issue()
	var a, b int
	fmt.Sscanf(ch.Question, "Ile to %d + %d?", &a, &b)
	answer := itoa(a + b)
	// Submitted instantly → should fail time-trap
	if err := s.Verify(ch.CaptchaID, answer); err == nil {
		t.Fatal("expected instant submission to fail time-trap")
	}
}

func TestExpired(t *testing.T) {
	s := NewStore(1*time.Millisecond, 0)
	ch := s.Issue()
	time.Sleep(10 * time.Millisecond)
	var a, b int
	fmt.Sscanf(ch.Question, "Ile to %d + %d?", &a, &b)
	if err := s.Verify(ch.CaptchaID, itoa(a+b)); err == nil {
		t.Fatal("expected expired captcha to fail")
	}
}

func TestHoneypot(t *testing.T) {
	if !IsHoneypotTripped("http://spam.com") {
		t.Error("expected honeypot to trip on non-empty value")
	}
	if IsHoneypotTripped("") {
		t.Error("expected honeypot to pass on empty value")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
