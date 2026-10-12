package security

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("correct horse 42")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("correct horse 42", h) {
		t.Error("valid password rejected")
	}
	if VerifyPassword("correct horse 43", h) {
		t.Error("wrong password accepted")
	}
	if VerifyPassword("x", "garbage") {
		t.Error("malformed hash accepted")
	}
}

func TestPasswordPolicy(t *testing.T) {
	cases := map[string]bool{
		"short1":          false,
		"alllettersonly":  false,
		"1234567890123":   false,
		"goodpassword123": true,
	}
	for pw, ok := range cases {
		if (ValidatePassword(pw, "someone") == nil) != ok {
			t.Errorf("policy mismatch for %q", pw)
		}
	}
}

func TestTempPasswordMeetsPolicy(t *testing.T) {
	for i := 0; i < 20; i++ {
		pw, err := GenerateTempPassword()
		if err != nil || ValidatePassword(pw, "") != nil {
			t.Fatalf("bad temp password %q: %v", pw, err)
		}
	}
}

func TestEncryptBoundToOwner(t *testing.T) {
	key := make([]byte, 32)
	enc, err := Encrypt(key, "0123456789", "user-7")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Decrypt(key, enc, "user-7"); err != nil || got != "0123456789" {
		t.Fatalf("round trip failed: %q %v", got, err)
	}
	if _, err := Decrypt(key, enc, "user-8"); err == nil {
		t.Error("ciphertext decrypted under a different owner")
	}
}
