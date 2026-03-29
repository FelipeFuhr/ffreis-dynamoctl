package cmd

import (
	"testing"

	"github.com/ffreis/dynamoctl/internal/crypto"
	"github.com/ffreis/dynamoctl/internal/store"
)

func TestDecryptForGetCmd_NoDecryptWhenRawOrPlaintext(t *testing.T) {
	item := &store.Item{Encrypted: false, Value: "v"}
	got, err := decryptForGetCmd(item, false, "")
	if err != nil {
		t.Fatalf("decryptForGetCmd: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty decrypted value, got %q", got)
	}

	item = &store.Item{Encrypted: true, Value: "cipher"}
	got, err = decryptForGetCmd(item, true, "")
	if err != nil {
		t.Fatalf("decryptForGetCmd(raw=true): %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty decrypted value, got %q", got)
	}
}

func TestDecryptForGetCmd_Decrypts(t *testing.T) {
	k, _ := crypto.GenerateKey()
	ct, err := crypto.Encrypt([]byte("hello"), k)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	item := &store.Item{Encrypted: true, Value: ct}
	got, err := decryptForGetCmd(item, false, crypto.FormatKey(k))
	if err != nil {
		t.Fatalf("decryptForGetCmd: %v", err)
	}
	if got != "hello" {
		t.Fatalf("expected %q, got %q", "hello", got)
	}
}
