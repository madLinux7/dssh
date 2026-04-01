package crypto

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	passphrase := "test-master-passphrase"
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt: %v", err)
	}

	key := DeriveKey(passphrase, salt)
	plaintext := []byte("s3cret-ssh-password!")

	ciphertext, nonce, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := Decrypt(key, ciphertext, nonce)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted %q != original %q", decrypted, plaintext)
	}
}

func TestWrongPassphrase(t *testing.T) {
	salt, _ := GenerateSalt()
	key1 := DeriveKey("correct", salt)
	key2 := DeriveKey("wrong", salt)

	plaintext := []byte("secret")
	ciphertext, nonce, err := Encrypt(key1, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = Decrypt(key2, ciphertext, nonce)
	if err == nil {
		t.Fatal("expected decryption to fail with wrong key")
	}
}

func TestDifferentSaltsDifferentKeys(t *testing.T) {
	salt1, _ := GenerateSalt()
	salt2, _ := GenerateSalt()

	key1 := DeriveKey("same-passphrase", salt1)
	key2 := DeriveKey("same-passphrase", salt2)

	if bytes.Equal(key1, key2) {
		t.Fatal("different salts should produce different keys")
	}
}
