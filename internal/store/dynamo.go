package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ErrNotFound is returned when an item does not exist in the table.
var ErrNotFound = errors.New("item not found")

// DynamoStore implements Store backed by a DynamoDB table.
//
// Table schema:
//
//	PK (S): "NS#{namespace}"  — partition key
//	SK (S): "{name}"          — sort key
//
// All other attributes are non-key.
type DynamoStore struct {
	client DynamoDBClient
	table  string
}

// New returns a DynamoStore using the given client and table name.
func New(client DynamoDBClient, table string) *DynamoStore {
	return &DynamoStore{client: client, table: table}
}

// Put creates or updates an item.
// Version starts at 1 and increments on every update.
func (s *DynamoStore) Put(ctx context.Context, item Item) error {
	// Read current version so we can increment it.
	existing, err := s.Get(ctx, item.Namespace, item.Name)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("reading item before put: %w", err)
	}

	now := time.Now().UTC()
	if existing == nil {
		item.Version = 1
		item.CreatedAt = now
	} else {
		item.Version = existing.Version + 1
		item.CreatedAt = existing.CreatedAt
	}
	item.UpdatedAt = now

	rec := itemToRecord(item)
	av, err := attributevalue.MarshalMap(rec)
	if err != nil {
		return fmt.Errorf("marshalling item: %w", err)
	}

	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: sdkaws.String(s.table),
		Item:      av,
	})
	if err != nil {
		return fmt.Errorf("putting item %q/%q: %w", item.Namespace, item.Name, err)
	}

	slog.Debug("item stored", "namespace", item.Namespace, "name", item.Name, "version", item.Version)
	return nil
}

// Get retrieves a single item by namespace and name.
func (s *DynamoStore) Get(ctx context.Context, namespace, name string) (*Item, error) {
	key, err := attributevalue.MarshalMap(map[string]string{
		"PK": pkPrefix + namespace,
		"SK": name,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling key: %w", err)
	}

	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: sdkaws.String(s.table),
		Key:       key,
	})
	if err != nil {
		return nil, fmt.Errorf("getting item %q/%q: %w", namespace, name, err)
	}

	if len(out.Item) == 0 {
		return nil, ErrNotFound
	}

	var rec record
	if err := attributevalue.UnmarshalMap(out.Item, &rec); err != nil {
		return nil, fmt.Errorf("unmarshalling item: %w", err)
	}

	it := recordToItem(rec)
	return &it, nil
}

// List returns all items in the namespace via a Query on the PK.
func (s *DynamoStore) List(ctx context.Context, namespace string) ([]Item, error) {
	return s.queryNamespace(ctx, namespace)
}

// ScanNamespace returns all items in the namespace.
// Identical to List; exists to satisfy the Store interface semantics.
func (s *DynamoStore) ScanNamespace(ctx context.Context, namespace string) ([]Item, error) {
	return s.queryNamespace(ctx, namespace)
}

// ScanAll returns every item in the table via a full table scan.
// Use sparingly; prefer namespace-scoped operations in production.
func (s *DynamoStore) ScanAll(ctx context.Context) ([]Item, error) {
	var items []Item
	var lastKey map[string]dbtypes.AttributeValue

	for {
		out, err := s.client.Scan(ctx, &dynamodb.ScanInput{
			TableName:         sdkaws.String(s.table),
			ExclusiveStartKey: lastKey,
		})
		if err != nil {
			return nil, fmt.Errorf("scanning table %s: %w", s.table, err)
		}

		for _, raw := range out.Items {
			var rec record
			if err := attributevalue.UnmarshalMap(raw, &rec); err != nil {
				return nil, fmt.Errorf("unmarshalling scan result: %w", err)
			}
			items = append(items, recordToItem(rec))
		}

		if out.LastEvaluatedKey == nil {
			break
		}
		lastKey = out.LastEvaluatedKey
	}

	return items, nil
}

// Delete removes an item. No-op if it does not exist.
func (s *DynamoStore) Delete(ctx context.Context, namespace, name string) error {
	key, err := attributevalue.MarshalMap(map[string]string{
		"PK": pkPrefix + namespace,
		"SK": name,
	})
	if err != nil {
		return fmt.Errorf("marshalling key: %w", err)
	}

	_, err = s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: sdkaws.String(s.table),
		Key:       key,
	})
	if err != nil {
		return fmt.Errorf("deleting item %q/%q: %w", namespace, name, err)
	}

	slog.Debug("item deleted", "namespace", namespace, "name", name)
	return nil
}

// UpdateEncrypted replaces the value of an existing item only when its
// version equals expectedVersion. Prevents lost updates during key rotation.
// Returns ErrConflict when the condition fails.
func (s *DynamoStore) UpdateEncrypted(ctx context.Context, namespace, name, newValue string, expectedVersion int) error {
	key, err := attributevalue.MarshalMap(map[string]string{
		"PK": pkPrefix + namespace,
		"SK": name,
	})
	if err != nil {
		return fmt.Errorf("marshalling key: %w", err)
	}

	now, err := attributevalue.Marshal(time.Now().UTC())
	if err != nil {
		return fmt.Errorf("marshalling timestamp: %w", err)
	}
	val, err := attributevalue.Marshal(newValue)
	if err != nil {
		return fmt.Errorf("marshalling value: %w", err)
	}
	ev, err := attributevalue.Marshal(expectedVersion)
	if err != nil {
		return fmt.Errorf("marshalling version: %w", err)
	}
	newVer, err := attributevalue.Marshal(expectedVersion + 1)
	if err != nil {
		return fmt.Errorf("marshalling new version: %w", err)
	}

	_, err = s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: sdkaws.String(s.table),
		Key:       key,
		UpdateExpression: sdkaws.String(
			"SET #val = :val, #ver = :new_ver, updated_at = :now",
		),
		ConditionExpression: sdkaws.String("#ver = :expected_ver"),
		ExpressionAttributeNames: map[string]string{
			"#val": "value",
			"#ver": "version",
		},
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":val":          val,
			":now":          now,
			":expected_ver": ev,
			":new_ver":      newVer,
		},
	})
	if err != nil {
		var cce *dbtypes.ConditionalCheckFailedException
		if errors.As(err, &cce) {
			return ErrConflict
		}
		return fmt.Errorf("updating item %q/%q: %w", namespace, name, err)
	}

	slog.Debug("item re-encrypted", "namespace", namespace, "name", name)
	return nil
}

// queryNamespace queries all items for a given namespace using the PK index.
func (s *DynamoStore) queryNamespace(ctx context.Context, namespace string) ([]Item, error) {
	pk, err := attributevalue.Marshal(pkPrefix + namespace)
	if err != nil {
		return nil, fmt.Errorf("marshalling PK: %w", err)
	}

	var items []Item
	var lastKey map[string]dbtypes.AttributeValue

	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              sdkaws.String(s.table),
			KeyConditionExpression: sdkaws.String("PK = :pk"),
			ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
				":pk": pk,
			},
			ExclusiveStartKey: lastKey,
		})
		if err != nil {
			return nil, fmt.Errorf("querying namespace %q: %w", namespace, err)
		}

		for _, raw := range out.Items {
			var rec record
			if err := attributevalue.UnmarshalMap(raw, &rec); err != nil {
				return nil, fmt.Errorf("unmarshalling query result: %w", err)
			}
			items = append(items, recordToItem(rec))
		}

		if out.LastEvaluatedKey == nil {
			break
		}
		lastKey = out.LastEvaluatedKey
	}

	return items, nil
}

// ErrConflict is returned by UpdateEncrypted when the conditional check fails,
// indicating the item was modified concurrently.
var ErrConflict = errors.New("item was modified concurrently; re-run rotate")
