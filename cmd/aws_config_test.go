package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

func TestNewAWSConfigWrapsLoaderErrors(t *testing.T) {
	origLoader := loadDefaultConfig
	defer func() { loadDefaultConfig = origLoader }()

	loadDefaultConfig = func(context.Context, ...func(*config.LoadOptions) error) (sdkaws.Config, error) {
		return sdkaws.Config{}, errors.New("boom")
	}

	_, err := newAWSConfig(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "loading AWS config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewAWSStoreAndS3ClientConstructWithStubConfig(t *testing.T) {
	origLoader := loadDefaultConfig
	defer func() { loadDefaultConfig = origLoader }()

	loadDefaultConfig = func(context.Context, ...func(*config.LoadOptions) error) (sdkaws.Config, error) {
		return sdkaws.Config{Region: "us-east-1"}, nil
	}

	flagRegion = "us-east-1"
	flagTable = "tbl"

	st, err := newAWSStore(context.Background())
	if err != nil {
		t.Fatalf("newAWSStore: %v", err)
	}
	if st == nil {
		t.Fatal("expected store")
	}

	s3c, err := newAWSS3Client(context.Background())
	if err != nil {
		t.Fatalf("newAWSS3Client: %v", err)
	}
	if s3c == nil {
		t.Fatal("expected s3 client")
	}
}
