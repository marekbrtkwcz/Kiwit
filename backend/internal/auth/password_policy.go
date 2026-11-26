package auth

import (
	"errors"
	"strings"
)

var ErrWeakPassword = errors.New("password is too weak")

// commonPasswords is a compact list of the most common passwords (top ~100).
var commonPasswords = map[string]bool{
	"password": true, "123456": true, "12345678": true, "12345": true, "1234567": true,
	"qwerty": true, "abc123": true, "password1": true, "111111": true, "123123": true,
	"admin": true, "letmein": true, "welcome": true, "monkey": true, "dragon": true,
	"football": true, "baseball": true, "sunshine": true, "master": true, "shadow": true,
	"superman": true, "michael": true, "ninja": true, "mustang": true, "password123": true,
	"123456789": true, "1234567890": true, "qwerty123": true, "1q2w3e4r": true, "qazwsx": true,
	"trustno1": true, "iloveyou": true, "starwars": true, "whatever": true, "princess": true,
	"solo": true, "passw0rd": true, "p@ssw0rd": true, "p@ssword": true, "zaq12wsx": true,
	"123qwe": true, "qwe123": true, "qweasd": true, "asd123": true, "asdfgh": true,
	"1qaz2wsx": true, "qazxsw": true, "666666": true, "888888": true, "000000": true,
	"159753": true, "147258369": true, "987654321": true, "1234": true, "12345678910": true,
	"kiwit": true, "kiwi": true, "test": true, "test123": true, "tester": true,
	"secret": true, "changeme": true, "default": true, "root": true, "toor": true,
	"god": true, "love": true, "hello": true, "freedom": true, "whatever1": true,
	"computer": true, "internet": true, "pokemon": true, "samsung": true, "google": true,
	"facebook": true, "linkedin": true, "amazon": true, "netflix": true, "youtube": true,
	"apple": true, "orange": true, "banana": true, "summer": true, "winter": true,
	"spring": true, "autumn": true, "january": true, "february": true, "march": true,
	"april": true, "june": true, "july": true, "august": true, "september": true,
	"october": true, "november": true, "december": true, "flower": true, "happy": true,
	"smile": true, "angel": true, "magic": true, "wizard": true, "matrix": true,
}

// keyboardRows are common keyboard sequences (lowercase) used for pattern detection.
// Only sequences of 6+ consecutive chars count, to avoid false positives on
// ordinary words that contain short letter runs.
var keyboardRows = []string{
	"qwertyuiop", "asdfghjkl", "zxcvbnm",
	"1qaz2wsx3edc", "qazwsxedcrfv", "1234567890",
}

const keyboardSeqLen = 6

// ValidatePassword enforces the password policy per NIST SP 800-63B:
// 8–64 chars, screened against common passwords, must not contain the user's
// email/username, and must not be a keyboard walk or repeated pattern.
func ValidatePassword(password, email, username string) error {
	if len(password) < 8 || len(password) > 64 {
		return ErrWeakPassword
	}
	lower := strings.ToLower(password)
	if commonPasswords[lower] {
		return ErrWeakPassword
	}
	if email != "" {
		local := strings.ToLower(strings.SplitN(email, "@", 2)[0])
		if len(local) >= 4 && strings.Contains(lower, local) {
			return ErrWeakPassword
		}
	}
	if username != "" && len(username) >= 3 && strings.Contains(lower, strings.ToLower(username)) {
		return ErrWeakPassword
	}
	if allSame(password) || isKeyboardWalk(lower) || isRepeatedPattern(lower) {
		return ErrWeakPassword
	}
	return nil
}

func allSame(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return len(s) > 0
}

// isKeyboardWalk detects sequences like "qwertyui", "asdfgh", "1qaz2w" (6+ chars).
func isKeyboardWalk(s string) bool {
	for _, row := range keyboardRows {
		for i := 0; i+keyboardSeqLen <= len(row); i++ {
			seq := row[i : i+keyboardSeqLen]
			if strings.Contains(s, seq) || strings.Contains(s, reverse(seq)) {
				return true
			}
		}
	}
	return false
}

// isRepeatedPattern detects "abcabc", "xyxyxy" (pattern of 2-4 chars repeated 2+).
func isRepeatedPattern(s string) bool {
	for p := 2; p <= 4; p++ {
		if len(s) < p*2 {
			continue
		}
		pattern := s[:p]
		repeated := strings.Repeat(pattern, len(s)/p)
		if repeated == s[:len(repeated)] && len(s)%p == 0 {
			return true
		}
	}
	return false
}

func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}
