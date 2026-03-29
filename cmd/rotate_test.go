package cmd

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/ffreis/dynamoctl/internal/crypto"
	"github.com/ffreis/dynamoctl/internal/store"
)

type rotateTestStore struct {
	item store.Item

	updateCalls []int
	conflictOn  map[int]bool
}

func (s *rotateTestStore) Put(context.Context, store.Item) error              { panic("not used") }
func (s *rotateTestStore) List(context.Context, string) ([]store.Item, error) { panic("not used") }
func (s *rotateTestStore) Delete(context.Context, string, string) error       { panic("not used") }
func (s *rotateTestStore) ScanNamespace(context.Context, string) ([]store.Item, error) {
	panic("not used")
}
func (s *rotateTestStore) ScanAll(context.Context) ([]store.Item, error) { panic("not used") }
func (s *rotateTestStore) Restore(context.Context, store.Item) error     { panic("not used") }

func (s *rotateTestStore) Get(_ context.Context, namespace, name string) (*store.Item, error) {
	if namespace != s.item.Namespace || name != s.item.Name {
		return nil, errors.New("not found")
	}
	copy := s.item
	return &copy, nil
}

func (s *rotateTestStore) UpdateEncrypted(_ context.Context, namespace, name, newValue string, expectedVersion int) error {
	if namespace != s.item.Namespace || name != s.item.Name {
		return errors.New("not found")
	}
	s.updateCalls = append(s.updateCalls, expectedVersion)
	if s.conflictOn[expectedVersion] {
		// Simulate conflict by bumping version.
		s.item.Version++
		return store.ErrConflict
	}
	s.item.Value = newValue
	s.item.Encrypted = true
	s.item.Version = expectedVersion + 1
	return nil
}

func TestParseRotateKeys_RejectsMissingAndSameKey(t *testing.T) {
	_, _, err := parseRotateKeys("", "")
	if err == nil {
		t.Fatal("expected error for missing current key")
	}

	_, _, err = parseRotateKeys("a", "")
	if err == nil {
		t.Fatal("expected error for missing new key")
	}

	// Valid hex but same key.
	k, _ := crypto.GenerateKey()
	hex := crypto.FormatKey(k)
	_, _, err = parseRotateKeys(hex, hex)
	if err == nil {
		t.Fatal("expected error for same key")
	}
}

func TestUpdateEncryptedWithRetry_ConflictsOnceThenSucceeds(t *testing.T) {
	oldKey, _ := crypto.GenerateKey()
	newKey, _ := crypto.GenerateKey()

	plaintext := []byte("hello")
	enc, err := crypto.Encrypt(plaintext, oldKey)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	st := &rotateTestStore{
		item: store.Item{
			Namespace: "ns",
			Name:      "name",
			Value:     enc,
			Encrypted: true,
			Version:   1,
		},
		conflictOn: map[int]bool{1: true},
	}

	item := st.item
	log := slog.New(slog.NewTextHandler(ioDiscard{}, nil))

	newCiphertext, err := crypto.Encrypt(plaintext, newKey)
	if err != nil {
		t.Fatalf("Encrypt new: %v", err)
	}

	err = updateEncryptedWithRetry(context.Background(), st, item, oldKey, newKey, newCiphertext, log)
	if err != nil {
		t.Fatalf("updateEncryptedWithRetry: %v", err)
	}
	if len(st.updateCalls) != 2 || st.updateCalls[0] != 1 || st.updateCalls[1] != 2 {
		t.Fatalf("unexpected update calls: %v", st.updateCalls)
	}
}

func TestRotateEncryptedItem_WrongKeyFails(t *testing.T) {
	oldKey, _ := crypto.GenerateKey()
	wrongKey, _ := crypto.GenerateKey()
	newKey, _ := crypto.GenerateKey()

	enc, err := crypto.Encrypt([]byte("hello"), oldKey)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	st := &rotateTestStore{
		item:       store.Item{Namespace: "ns", Name: "name", Value: enc, Encrypted: true, Version: 1},
		conflictOn: map[int]bool{},
	}

	log := slog.New(slog.NewTextHandler(ioDiscard{}, nil))
	err = rotateEncryptedItem(context.Background(), st, st.item, wrongKey, newKey, log)
	if err == nil {
		t.Fatal("expected decrypt error, got nil")
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (n int, err error) { return len(p), nil }
