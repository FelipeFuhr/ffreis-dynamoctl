package cmd

import (
	"bytes"
	"testing"

	"github.com/ffreis/dynamoctl/internal/crypto"
)

const (
	errFmtResolveSetValue = "resolveSetValue: %v"
	errFmtEncryptValue    = "encryptValue: %v"
	wantGotFmt            = "expected %q, got %q"
)

func TestResolveSetValueFromArgs(t *testing.T) {
	got, err := resolveSetValue(bytes.NewBufferString("ignored"), false, []string{"k", "v"})
	if err != nil {
		t.Fatalf(errFmtResolveSetValue, err)
	}
	if got != "v" {
		t.Fatalf(wantGotFmt, "v", got)
	}
}

func TestResolveSetValueFromStdinFlag(t *testing.T) {
	got, err := resolveSetValue(bytes.NewBufferString("a\nb\n"), true, []string{"k"})
	if err != nil {
		t.Fatalf(errFmtResolveSetValue, err)
	}
	if got != "a\nb" {
		t.Fatalf(wantGotFmt, "a\nb", got)
	}
}

func TestResolveSetValueFromDashArg(t *testing.T) {
	got, err := resolveSetValue(bytes.NewBufferString("x\n"), false, []string{"k", "-"})
	if err != nil {
		t.Fatalf(errFmtResolveSetValue, err)
	}
	if got != "x" {
		t.Fatalf(wantGotFmt, "x", got)
	}
}

func TestEncryptValueNoEncrypt(t *testing.T) {
	got, encrypted, err := encryptValue("v", true, "")
	if err != nil {
		t.Fatalf(errFmtEncryptValue, err)
	}
	if encrypted {
		t.Fatal("expected encrypted=false")
	}
	if got != "v" {
		t.Fatalf(wantGotFmt, "v", got)
	}
}

func TestEncryptValueEncryptsWithKey(t *testing.T) {
	k, _ := crypto.GenerateKey()
	keyHex := crypto.FormatKey(k)

	ciphertext, encrypted, err := encryptValue("hello", false, keyHex)
	if err != nil {
		t.Fatalf(errFmtEncryptValue, err)
	}
	if !encrypted {
		t.Fatal("expected encrypted=true")
	}

	plain, err := crypto.Decrypt(ciphertext, k)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(plain) != "hello" {
		t.Fatalf(wantGotFmt, "hello", string(plain))
	}
}
