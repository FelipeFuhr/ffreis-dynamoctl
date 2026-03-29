package store

import (
	"context"
	"errors"
	"testing"
	"time"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ---------------------------------------------------------------------------
// Mock DynamoDB client
// ---------------------------------------------------------------------------

// memDB is an in-memory DynamoDBClient for testing.
// Items are stored by "PK/SK" composite key.
type memDB struct {
	items    map[string]map[string]dbtypes.AttributeValue
	putCalls int
	getCalls int
	putErr   error // if set, PutItem returns this error
}

func newMemDB() *memDB {
	return &memDB{items: make(map[string]map[string]dbtypes.AttributeValue)}
}

func (m *memDB) key(pk, sk string) string { return pk + "\x00" + sk }

func (m *memDB) PutItem(_ context.Context, params *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	m.putCalls++
	if m.putErr != nil {
		return nil, m.putErr
	}
	pk := params.Item["PK"].(*dbtypes.AttributeValueMemberS).Value
	sk := params.Item["SK"].(*dbtypes.AttributeValueMemberS).Value
	m.items[m.key(pk, sk)] = params.Item
	return &dynamodb.PutItemOutput{}, nil
}

func (m *memDB) GetItem(_ context.Context, params *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	m.getCalls++
	pk := params.Key["PK"].(*dbtypes.AttributeValueMemberS).Value
	sk := params.Key["SK"].(*dbtypes.AttributeValueMemberS).Value
	item, ok := m.items[m.key(pk, sk)]
	if !ok {
		return &dynamodb.GetItemOutput{}, nil
	}
	return &dynamodb.GetItemOutput{Item: item}, nil
}

func (m *memDB) Query(_ context.Context, params *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	targetPK := params.ExpressionAttributeValues[":pk"].(*dbtypes.AttributeValueMemberS).Value

	var result []map[string]dbtypes.AttributeValue
	for _, item := range m.items {
		pk := item["PK"].(*dbtypes.AttributeValueMemberS).Value
		if pk == targetPK {
			result = append(result, item)
		}
	}
	return &dynamodb.QueryOutput{Items: result}, nil
}

func (m *memDB) DeleteItem(_ context.Context, params *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	pk := params.Key["PK"].(*dbtypes.AttributeValueMemberS).Value
	sk := params.Key["SK"].(*dbtypes.AttributeValueMemberS).Value
	delete(m.items, m.key(pk, sk))
	return &dynamodb.DeleteItemOutput{}, nil
}

func (m *memDB) Scan(_ context.Context, _ *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	items := make([]map[string]dbtypes.AttributeValue, 0, len(m.items))
	for _, item := range m.items {
		items = append(items, item)
	}
	return &dynamodb.ScanOutput{Items: items}, nil
}

func (m *memDB) UpdateItem(_ context.Context, params *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	pk := params.Key["PK"].(*dbtypes.AttributeValueMemberS).Value
	sk := params.Key["SK"].(*dbtypes.AttributeValueMemberS).Value
	k := m.key(pk, sk)

	item, ok := m.items[k]
	if !ok {
		return nil, &dbtypes.ConditionalCheckFailedException{}
	}

	// Check version condition.
	expectedVer := params.ExpressionAttributeValues[":expected_ver"].(*dbtypes.AttributeValueMemberN).Value
	currentVer := item["version"].(*dbtypes.AttributeValueMemberN).Value
	if expectedVer != currentVer {
		return nil, &dbtypes.ConditionalCheckFailedException{}
	}

	// Apply the update.
	item["value"] = params.ExpressionAttributeValues[":val"]
	item["version"] = params.ExpressionAttributeValues[":new_ver"]
	item["updated_at"] = params.ExpressionAttributeValues[":now"]
	m.items[k] = item

	return &dynamodb.UpdateItemOutput{}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestStore(db *memDB) *DynamoStore {
	return New(db, testTableName)
}

// ---------------------------------------------------------------------------
// Tests: Put
// ---------------------------------------------------------------------------

func TestPutNewItemVersionOne(t *testing.T) {
	db := newMemDB()
	s := newTestStore(db)

	err := s.Put(context.Background(), &Item{
		Namespace: testNamespaceDefault,
		Name:      "foo",
		Value:     "bar",
		Encrypted: false,
	})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := s.Get(context.Background(), testNamespaceDefault, "foo")
	if err != nil {
		t.Fatalf("Get after Put: %v", err)
	}
	if got.Version != 1 {
		t.Errorf("version: want 1, got %d", got.Version)
	}
}

func TestPutExistingItemIncrementsVersion(t *testing.T) {
	db := newMemDB()
	s := newTestStore(db)
	ctx := context.Background()

	_ = s.Put(ctx, &Item{Namespace: testNamespaceDefault, Name: "key", Value: "v1"})
	_ = s.Put(ctx, &Item{Namespace: testNamespaceDefault, Name: "key", Value: "v2"})

	got, err := s.Get(ctx, testNamespaceDefault, "key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Version != 2 {
		t.Errorf("version: want 2 after second put, got %d", got.Version)
	}
	if got.Value != "v2" {
		t.Errorf("value: want v2, got %s", got.Value)
	}
}

func TestPutPreservesCreatedAt(t *testing.T) {
	db := newMemDB()
	s := newTestStore(db)
	ctx := context.Background()

	_ = s.Put(ctx, &Item{Namespace: testNamespaceDefault, Name: "k", Value: "v1"})
	first, _ := s.Get(ctx, testNamespaceDefault, "k")
	created := first.CreatedAt

	time.Sleep(time.Millisecond)
	_ = s.Put(ctx, &Item{Namespace: testNamespaceDefault, Name: "k", Value: "v2"})
	second, _ := s.Get(ctx, testNamespaceDefault, "k")

	if !second.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt changed on update: was %v, now %v", created, second.CreatedAt)
	}
}

// ---------------------------------------------------------------------------
// Tests: Get
// ---------------------------------------------------------------------------

func TestGetNotFound(t *testing.T) {
	db := newMemDB()
	s := newTestStore(db)

	_, err := s.Get(context.Background(), testNamespaceDefault, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestGetCorrectNamespaceIsolation(t *testing.T) {
	db := newMemDB()
	s := newTestStore(db)
	ctx := context.Background()

	_ = s.Put(ctx, &Item{Namespace: "ns1", Name: "key", Value: "ns1-val"})
	_ = s.Put(ctx, &Item{Namespace: "ns2", Name: "key", Value: "ns2-val"})

	got1, _ := s.Get(ctx, "ns1", "key")
	got2, _ := s.Get(ctx, "ns2", "key")

	if got1.Value != "ns1-val" || got2.Value != "ns2-val" {
		t.Errorf("namespace isolation broken: ns1=%q ns2=%q", got1.Value, got2.Value)
	}
}

// ---------------------------------------------------------------------------
// Tests: List
// ---------------------------------------------------------------------------

func TestListReturnsAllItemsInNamespace(t *testing.T) {
	db := newMemDB()
	s := newTestStore(db)
	ctx := context.Background()

	_ = s.Put(ctx, &Item{Namespace: testNamespaceProd, Name: "a", Value: "1"})
	_ = s.Put(ctx, &Item{Namespace: testNamespaceProd, Name: "b", Value: "2"})
	_ = s.Put(ctx, &Item{Namespace: "staging", Name: "c", Value: "3"}) // different namespace

	items, err := s.List(ctx, testNamespaceProd)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("want 2 items in prod, got %d", len(items))
	}
}

func TestListEmptyNamespace(t *testing.T) {
	db := newMemDB()
	s := newTestStore(db)

	items, err := s.List(context.Background(), "empty")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("want 0 items, got %d", len(items))
	}
}

func TestScanNamespaceUsesPKQuery(t *testing.T) {
	db := newMemDB()
	s := newTestStore(db)
	ctx := context.Background()

	_ = s.Put(ctx, &Item{Namespace: "prod", Name: "a", Value: "1"})
	_ = s.Put(ctx, &Item{Namespace: "prod", Name: "b", Value: "2"})
	_ = s.Put(ctx, &Item{Namespace: "staging", Name: "c", Value: "3"})

	items, err := s.ScanNamespace(ctx, "prod")
	if err != nil {
		t.Fatalf("ScanNamespace: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items in prod, got %d", len(items))
	}
	for _, it := range items {
		if it.Namespace != "prod" {
			t.Fatalf("unexpected namespace: %s", it.Namespace)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: Delete
// ---------------------------------------------------------------------------

func TestDeleteRemovesItem(t *testing.T) {
	db := newMemDB()
	s := newTestStore(db)
	ctx := context.Background()

	_ = s.Put(ctx, &Item{Namespace: testNamespaceDefault, Name: "todelete", Value: "x"})
	if err := s.Delete(ctx, testNamespaceDefault, "todelete"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := s.Get(ctx, testNamespaceDefault, "todelete")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDeleteIdempotent(t *testing.T) {
	db := newMemDB()
	s := newTestStore(db)
	ctx := context.Background()

	// Delete an item that does not exist — must not error.
	if err := s.Delete(ctx, testNamespaceDefault, "phantom"); err != nil {
		t.Errorf("Delete non-existent item: want nil, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: ScanAll
// ---------------------------------------------------------------------------

func TestScanAllReturnsAllNamespaces(t *testing.T) {
	db := newMemDB()
	s := newTestStore(db)
	ctx := context.Background()

	_ = s.Put(ctx, &Item{Namespace: "ns1", Name: "a", Value: "1"})
	_ = s.Put(ctx, &Item{Namespace: "ns2", Name: "b", Value: "2"})
	_ = s.Put(ctx, &Item{Namespace: "ns3", Name: "c", Value: "3"})

	items, err := s.ScanAll(ctx)
	if err != nil {
		t.Fatalf("ScanAll: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("want 3 items, got %d", len(items))
	}
}

// ---------------------------------------------------------------------------
// Tests: UpdateEncrypted
// ---------------------------------------------------------------------------

func TestUpdateEncryptedSuccessOnMatchingVersion(t *testing.T) {
	db := newMemDB()
	s := newTestStore(db)
	ctx := context.Background()

	_ = s.Put(ctx, &Item{Namespace: testNamespaceDefault, Name: "k", Value: "old", Encrypted: true})
	item, _ := s.Get(ctx, testNamespaceDefault, "k")

	err := s.UpdateEncrypted(ctx, testNamespaceDefault, "k", "new-ciphertext", item.Version)
	if err != nil {
		t.Fatalf("UpdateEncrypted: %v", err)
	}

	updated, _ := s.Get(ctx, testNamespaceDefault, "k")
	if updated.Value != "new-ciphertext" {
		t.Errorf("value not updated: got %q", updated.Value)
	}
	if updated.Version != item.Version+1 {
		t.Errorf("version not incremented: want %d, got %d", item.Version+1, updated.Version)
	}
}

func TestUpdateEncryptedFailsOnVersionMismatch(t *testing.T) {
	db := newMemDB()
	s := newTestStore(db)
	ctx := context.Background()

	_ = s.Put(ctx, &Item{Namespace: testNamespaceDefault, Name: "k", Value: "old", Encrypted: true})

	// Use a stale version (0 instead of 1).
	err := s.UpdateEncrypted(ctx, testNamespaceDefault, "k", "new", 0)
	if !errors.Is(err, ErrConflict) {
		t.Errorf("want ErrConflict on version mismatch, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: Restore
// ---------------------------------------------------------------------------

func TestRestorePreservesMetadata(t *testing.T) {
	db := newMemDB()
	s := newTestStore(db)
	ctx := context.Background()

	createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	original := &Item{
		Namespace: "prod",
		Name:      "api-key",
		Value:     "ciphertext",
		Encrypted: true,
		Version:   7,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	if err := s.Restore(ctx, original); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	got, err := s.Get(ctx, original.Namespace, original.Name)
	if err != nil {
		t.Fatalf("Get after Restore: %v", err)
	}
	if got.Version != original.Version {
		t.Fatalf("version: want %d, got %d", original.Version, got.Version)
	}
	if !got.CreatedAt.Equal(original.CreatedAt) {
		t.Fatalf("CreatedAt: want %v, got %v", original.CreatedAt, got.CreatedAt)
	}
	if !got.UpdatedAt.Equal(original.UpdatedAt) {
		t.Fatalf("UpdatedAt: want %v, got %v", original.UpdatedAt, got.UpdatedAt)
	}
}

// ---------------------------------------------------------------------------
// Tests: record conversion
// ---------------------------------------------------------------------------

func TestRecordToItemRoundtrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := Item{
		Namespace: "myns",
		Name:      "mykey",
		Value:     "myval",
		Encrypted: true,
		Version:   7,
		CreatedAt: now,
		UpdatedAt: now,
	}

	rec := itemToRecord(&original)
	if rec.PK != "NS#myns" {
		t.Errorf("PK: want NS#myns, got %s", rec.PK)
	}
	if rec.SK != "mykey" {
		t.Errorf("SK: want mykey, got %s", rec.SK)
	}

	back := recordToItem(&rec)
	if back.Namespace != original.Namespace {
		t.Errorf("Namespace: want %q, got %q", original.Namespace, back.Namespace)
	}
	if back.Name != original.Name {
		t.Errorf("Name: want %q, got %q", original.Name, back.Name)
	}
	if back.Version != original.Version {
		t.Errorf("Version: want %d, got %d", original.Version, back.Version)
	}
}

// ---------------------------------------------------------------------------
// Tests: Put error propagation
// ---------------------------------------------------------------------------

func TestPutPropagatesDynamoError(t *testing.T) {
	db := newMemDB()
	db.putErr = errors.New("dynamo unavailable")
	s := newTestStore(db)

	err := s.Put(context.Background(), &Item{Namespace: testNamespaceDefault, Name: "k", Value: "v"})
	if err == nil {
		t.Error("expected error propagated from DynamoDB, got nil")
	}
}

// ---------------------------------------------------------------------------
// Ensure *memDB satisfies DynamoDBClient at compile time.
// ---------------------------------------------------------------------------

var _ DynamoDBClient = (*memDB)(nil)

// Satisfy the interface — marshalling helpers needed by UpdateItem mock.
var _ = sdkaws.String
