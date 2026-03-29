package crypto

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// --- Encrypt / Decrypt ---

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := mustGenKey(t)
	plaintext := []byte("hello, dynamoctl!")

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	got, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(plaintext, got) {
		t.Errorf("roundtrip mismatch: want %q, got %q", plaintext, got)
	}
}

func TestEncryptDecryptEmptyPlaintext(t *testing.T) {
	key := mustGenKey(t)
	ciphertext, err := Encrypt([]byte{}, key)
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}
	got, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt empty: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty plaintext, got %q", got)
	}
}

func TestEncryptProducesUniqueOutputEachCall(t *testing.T) {
	key := mustGenKey(t)
	plaintext := []byte("same input")

	ct1, _ := Encrypt(plaintext, key)
	ct2, _ := Encrypt(plaintext, key)

	if ct1 == ct2 {
		t.Error("expected different ciphertexts for same plaintext (random nonce)")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := mustGenKey(t)
	key2 := mustGenKey(t)

	ct, _ := Encrypt([]byte("secret"), key1)
	_, err := Decrypt(ct, key2)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	key := mustGenKey(t)
	ct, _ := Encrypt([]byte("secret"), key)

	// Flip the last byte of the base64 payload.
	raw, _ := base64.StdEncoding.DecodeString(ct)
	raw[len(raw)-1] ^= 0xff
	tampered := base64.StdEncoding.EncodeToString(raw)

	_, err := Decrypt(tampered, key)
	if err == nil {
		t.Error("expected error for tampered ciphertext")
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	key := mustGenKey(t)
	_, err := Decrypt("not!valid!base64", key)
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestDecryptTooShort(t *testing.T) {
	key := mustGenKey(t)
	// 5 bytes — well below nonce (12) + tag (16) minimum.
	short := base64.StdEncoding.EncodeToString([]byte("short"))
	_, err := Decrypt(short, key)
	if err == nil {
		t.Error("expected error for ciphertext too short")
	}
}

func TestEncryptDecryptLargePayload(t *testing.T) {
	key := mustGenKey(t)
	plaintext := []byte(strings.Repeat("A", 1<<20)) // 1 MiB

	ct, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt large: %v", err)
	}
	got, err := Decrypt(ct, key)
	if err != nil {
		t.Fatalf("Decrypt large: %v", err)
	}
	if !bytes.Equal(plaintext, got) {
		t.Error("large payload roundtrip mismatch")
	}
}

// --- ParseKey ---

func TestParseKeyValidHex(t *testing.T) {
	key := mustGenKey(t)
	hexStr := FormatKey(key)

	parsed, err := ParseKey(hexStr)
	if err != nil {
		t.Fatalf("ParseKey: %v", err)
	}
	if parsed != key {
		t.Error("ParseKey returned a different key")
	}
}

func TestParseKeyTooShort(t *testing.T) {
	_, err := ParseKey("deadbeef") // 4 bytes, not 32
	if err == nil {
		t.Error("expected error for too-short key")
	}
}

func TestParseKeyTooLong(t *testing.T) {
	_, err := ParseKey(strings.Repeat("aa", 33)) // 33 bytes
	if err == nil {
		t.Error("expected error for too-long key")
	}
}

func TestParseKeyInvalidHex(t *testing.T) {
	_, err := ParseKey(strings.Repeat("zz", 32)) // invalid hex chars
	if err == nil {
		t.Error("expected error for invalid hex")
	}
}

func TestParseKeyEmpty(t *testing.T) {
	_, err := ParseKey("")
	if err == nil {
		t.Error("expected error for empty key")
	}
}

// --- GenerateKey ---

func TestGenerateKeyProducesUniqueKeys(t *testing.T) {
	k1, err1 := GenerateKey()
	k2, err2 := GenerateKey()
	if err1 != nil || err2 != nil {
		t.Fatalf("GenerateKey errors: %v / %v", err1, err2)
	}
	if k1 == k2 {
		t.Error("expected unique keys from consecutive GenerateKey calls")
	}
}

func TestGenerateKeyParseRoundtrip(t *testing.T) {
	key, _ := GenerateKey()
	parsed, err := ParseKey(FormatKey(key))
	if err != nil {
		t.Fatalf("ParseKey(FormatKey(key)): %v", err)
	}
	if parsed != key {
		t.Error("generate → format → parse roundtrip failed")
	}
}

// --- helpers ---

func mustGenKey(t *testing.T) Key {
	t.Helper()
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return key
}
