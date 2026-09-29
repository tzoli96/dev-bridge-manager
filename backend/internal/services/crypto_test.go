package services

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	original := "hunter2"
	ciphertext, err := Encrypt(original)
	if err != nil {
		t.Fatalf("unexpected encrypt error: %v", err)
	}
	if ciphertext == original {
		t.Fatal("expected ciphertext to differ from plaintext")
	}
	decrypted, err := Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("unexpected decrypt error: %v", err)
	}
	if decrypted != original {
		t.Fatalf("got %q, want %q", decrypted, original)
	}
}

func TestEncryptProducesDifferentCiphertextEachTime(t *testing.T) {
	a, err := Encrypt("same-password")
	if err != nil {
		t.Fatalf("unexpected encrypt error: %v", err)
	}
	b, err := Encrypt("same-password")
	if err != nil {
		t.Fatalf("unexpected encrypt error: %v", err)
	}
	if a == b {
		t.Fatal("expected two encryptions of the same plaintext to differ (random nonce)")
	}
}

func TestDecryptRejectsTamperedCiphertext(t *testing.T) {
	ciphertext, err := Encrypt("hunter2")
	if err != nil {
		t.Fatalf("unexpected encrypt error: %v", err)
	}
	tampered := ciphertext[:len(ciphertext)-4] + "abcd"
	if _, err := Decrypt(tampered); err == nil {
		t.Fatal("expected tampered ciphertext to fail to decrypt")
	}
}

func TestEncryptDecryptEmptyString(t *testing.T) {
	ciphertext, err := Encrypt("")
	if err != nil {
		t.Fatalf("unexpected encrypt error: %v", err)
	}
	decrypted, err := Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("unexpected decrypt error: %v", err)
	}
	if decrypted != "" {
		t.Fatalf("got %q, want empty string", decrypted)
	}
}
