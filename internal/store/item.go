// Package store provides DynamoDB-backed key-value storage with optional
// per-item encryption metadata.
package store

import "time"

const pkPrefix = "NS#"

// Item is the domain model for a stored value.
// The Namespace and Name together uniquely identify an item.
type Item struct {
	Namespace string
	Name      string
	// Value is the raw stored string: base64(nonce‖ciphertext) when Encrypted,
	// otherwise the plaintext value.
	Value     string
	Encrypted bool
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// record is the DynamoDB wire representation of an Item.
// The PK prefix keeps all items for a namespace co-located on the same
// partition so a Query (not Scan) can retrieve them efficiently.
type record struct {
	PK        string    `dynamodbav:"PK"`
	SK        string    `dynamodbav:"SK"`
	Value     string    `dynamodbav:"value"`
	Encrypted bool      `dynamodbav:"encrypted"`
	Version   int       `dynamodbav:"version"`
	CreatedAt time.Time `dynamodbav:"created_at"`
	UpdatedAt time.Time `dynamodbav:"updated_at"`
}

func itemToRecord(it *Item) record {
	return record{
		PK:        pkPrefix + it.Namespace,
		SK:        it.Name,
		Value:     it.Value,
		Encrypted: it.Encrypted,
		Version:   it.Version,
		CreatedAt: it.CreatedAt,
		UpdatedAt: it.UpdatedAt,
	}
}

func recordToItem(r *record) Item {
	return Item{
		Namespace: r.PK[len(pkPrefix):], // strip "NS#" prefix
		Name:      r.SK,
		Value:     r.Value,
		Encrypted: r.Encrypted,
		Version:   r.Version,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}
