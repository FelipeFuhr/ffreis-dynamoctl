package cmd

import (
	"bytes"
	"testing"

	"github.com/ffreis/dynamoctl/internal/crypto"
)

func TestResolveSetValue_FromArgs(t *testing.T) {
	got, err := resolveSetValue(bytes.NewBufferString("ignored"), false, []string{"k", "v"})
	if err != nil {
		t.Fatalf("resolveSetValue: %v", err)
	}
	if got != "v" {
		t.Fatalf("expected %q, got %q", "v", got)
	}
}

func TestResolveSetValue_FromStdinFlag(t *testing.T) {
	got, err := resolveSetValue(bytes.NewBufferString("a\nb\n"), true, []string{"k"})
	if err != nil {
		t.Fatalf("resolveSetValue: %v", err)
	}
	if got != "a\nb" {
		t.Fatalf("expected %q, got %q", "a\nb", got)
	}
}

func TestResolveSetValue_FromDashArg(t *testing.T) {
	got, err := resolveSetValue(bytes.NewBufferString("x\n"), false, []string{"k", "-"})
	if err != nil {
		t.Fatalf("resolveSetValue: %v", err)
	}
	if got != "x" {
		t.Fatalf("expected %q, got %q", "x", got)
	}
}

func TestEncryptValue_NoEncrypt(t *testing.T) {
	got, encrypted, err := encryptValue("v", true, "")
	if err != nil {
		t.Fatalf("encryptValue: %v", err)
	}
	if encrypted {
		t.Fatal("expected encrypted=false")
	}
	if got != "v" {
		t.Fatalf("expected %q, got %q", "v", got)
	}
}

func TestEncryptValue_EncryptsWithKey(t *testing.T) {
	k, _ := crypto.GenerateKey()
	keyHex := crypto.FormatKey(k)

	ciphertext, encrypted, err := encryptValue("hello", false, keyHex)
	if err != nil {
		t.Fatalf("encryptValue: %v", err)
	}
	if !encrypted {
		t.Fatal("expected encrypted=true")
	}

	plain, err := crypto.Decrypt(ciphertext, k)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(plain) != "hello" {
		t.Fatalf("expected %q, got %q", "hello", string(plain))
	}
}
