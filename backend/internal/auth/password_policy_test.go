package auth

import "testing"

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		pw, email, username string
		wantErr             bool
	}{
		{"Tr0ub4dor&3x", "", "", false},
		{"short", "", "", true},
		{"password", "", "", true},
		{"Password1", "", "", true},
		{"aaaaaaaa", "", "", true},
		{"myemail123", "myemail@test.com", "", true},
		{"johnsmith99", "", "johnsmith", true},

		{"xyxyxyxy", "", "", true},     // repeated pattern
		{"qwertyuiop1", "", "", true},  // keyboard walk 6+
		{"Abcdefgh1!x", "", "", false}, // fine
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "", "", true}, // >64
	}
	for _, c := range cases {
		err := ValidatePassword(c.pw, c.email, c.username)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidatePassword(%q, %q, %q) err=%v, wantErr=%v", c.pw, c.email, c.username, err, c.wantErr)
		}
	}
}
