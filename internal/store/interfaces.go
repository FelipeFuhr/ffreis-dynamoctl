package store

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Store is the interface for all key-value operations.
type Store interface {
	// Put creates or updates an item. The Version field is managed automatically:
	// 1 for new items, existing_version+1 for updates.
	Put(ctx context.Context, item Item) error

	// Get returns the item with the given namespace and name.
	// Returns ErrNotFound when the item does not exist.
	Get(ctx context.Context, namespace, name string) (*Item, error)

	// List returns all items in the namespace (metadata only — Value is included).
	List(ctx context.Context, namespace string) ([]Item, error)

	// Delete removes the item. A no-op if the item does not exist.
	Delete(ctx context.Context, namespace, name string) error

	// ScanNamespace returns all items in the namespace. Used by backup.
	ScanNamespace(ctx context.Context, namespace string) ([]Item, error)

	// ScanAll returns every item in the table. Used by full-table backup.
	ScanAll(ctx context.Context) ([]Item, error)

	// UpdateEncrypted atomically replaces the stored value of an existing item
	// only if its current version equals expectedVersion.
	// Used by rotate to prevent lost updates.
	UpdateEncrypted(ctx context.Context, namespace, name, newValue string, expectedVersion int) error

	// Restore writes an item preserving its original metadata (Version, CreatedAt,
	// UpdatedAt). Unlike Put, it does not auto-assign version or timestamps.
	// Intended for use during backup restoration.
	Restore(ctx context.Context, item Item) error
}

// DynamoDBClient is the subset of *dynamodb.Client used by DynamoStore.
// Declare it here so tests can inject a mock without importing the real SDK.
type DynamoDBClient interface {
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}
