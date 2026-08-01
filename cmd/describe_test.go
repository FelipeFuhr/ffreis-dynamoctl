package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// fakeDescribeTableAPI implements describeTableAPI with a canned response, so
// the success path (schema/GSI construction, PrintDescribeResult) can be
// exercised without a real DynamoDB endpoint.
type fakeDescribeTableAPI struct {
	out *awsdynamodb.DescribeTableOutput
	err error
}

func (f *fakeDescribeTableAPI) DescribeTable(context.Context, *awsdynamodb.DescribeTableInput, ...func(*awsdynamodb.Options)) (*awsdynamodb.DescribeTableOutput, error) {
	return f.out, f.err
}

func TestDescribeCmdFactoryErrorPropagates(t *testing.T) {
	orig := ddbClientFactory
	defer func() { ddbClientFactory = orig }()

	flagTable = "my-table"
	flagJSON = false

	wantErr := errors.New("no credentials")
	ddbClientFactory = func(_ context.Context) (*awsdynamodb.Client, error) {
		return nil, wantErr
	}

	var out bytes.Buffer
	cmd := newDescribeCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error from factory failure, got nil")
	}
	if !strings.Contains(err.Error(), "no credentials") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDescribeCmdUsesGlobalFlagTable(t *testing.T) {
	orig := ddbClientFactory
	defer func() { ddbClientFactory = orig }()

	// flagTable is a persistent flag on root; set it directly as all other
	// cmd tests do when testing subcommands in isolation.
	flagTable = "target-table"
	flagJSON = false

	var capturedTable string
	ddbClientFactory = func(_ context.Context) (*awsdynamodb.Client, error) {
		capturedTable = flagTable
		return nil, errors.New("stop early")
	}

	var out bytes.Buffer
	cmd := newDescribeCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	_ = cmd.Execute()

	if capturedTable != "target-table" {
		t.Fatalf("flagTable: want %q, got %q", "target-table", capturedTable)
	}
}

// TestDescribeCmdSuccessPathBuildsSchemaAndGSIs covers the bulk of the
// command that the two tests above never reach: cross-referencing
// AttributeDefinitions against KeySchema/GSI KeySchema to fill in AttrType,
// and rendering the result as JSON. A wrong lookup here would silently print
// an empty attr_type rather than fail loudly, so this pins the actual values.
func TestDescribeCmdSuccessPathBuildsSchemaAndGSIs(t *testing.T) {
	origFactory := describeClientFactory
	defer func() { describeClientFactory = origFactory }()

	flagTable = "orders"
	flagJSON = true

	fake := &fakeDescribeTableAPI{
		out: &awsdynamodb.DescribeTableOutput{
			Table: &ddbtypes.TableDescription{
				TableName:   aws.String("orders"),
				TableStatus: ddbtypes.TableStatusActive,
				BillingModeSummary: &ddbtypes.BillingModeSummary{
					BillingMode: ddbtypes.BillingModePayPerRequest,
				},
				ItemCount:      aws.Int64(42),
				TableSizeBytes: aws.Int64(1024),
				AttributeDefinitions: []ddbtypes.AttributeDefinition{
					{AttributeName: aws.String("pk"), AttributeType: ddbtypes.ScalarAttributeTypeS},
					{AttributeName: aws.String("sk"), AttributeType: ddbtypes.ScalarAttributeTypeS},
					{AttributeName: aws.String("gsi1pk"), AttributeType: ddbtypes.ScalarAttributeTypeN},
				},
				KeySchema: []ddbtypes.KeySchemaElement{
					{AttributeName: aws.String("pk"), KeyType: ddbtypes.KeyTypeHash},
					{AttributeName: aws.String("sk"), KeyType: ddbtypes.KeyTypeRange},
				},
				GlobalSecondaryIndexes: []ddbtypes.GlobalSecondaryIndexDescription{
					{
						IndexName:   aws.String("gsi1"),
						IndexStatus: ddbtypes.IndexStatusActive,
						Projection:  &ddbtypes.Projection{ProjectionType: ddbtypes.ProjectionTypeAll},
						KeySchema: []ddbtypes.KeySchemaElement{
							{AttributeName: aws.String("gsi1pk"), KeyType: ddbtypes.KeyTypeHash},
						},
					},
				},
			},
		},
	}
	describeClientFactory = func(context.Context) (describeTableAPI, error) {
		return fake, nil
	}

	var out bytes.Buffer
	cmd := newDescribeCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var info describeOutput
	if err := json.Unmarshal(out.Bytes(), &info); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}

	if info.TableName != "orders" || info.BillingMode != "PAY_PER_REQUEST" ||
		info.ItemCount != 42 || info.SizeBytes != 1024 {
		t.Errorf("top-level fields mismatch: %+v", info)
	}
	if len(info.KeySchema) != 2 ||
		info.KeySchema[0] != (keyAttr{"pk", "HASH", "S"}) ||
		info.KeySchema[1] != (keyAttr{"sk", "RANGE", "S"}) {
		t.Errorf("key_schema mismatch, want [pk/HASH/S sk/RANGE/S], got: %+v", info.KeySchema)
	}
	if len(info.GSIs) != 1 {
		t.Fatalf("want 1 GSI, got: %+v", info.GSIs)
	}
	gsi := info.GSIs[0]
	if gsi.Name != "gsi1" || gsi.Status != "ACTIVE" || gsi.Projection != "ALL" {
		t.Errorf("gsi top-level fields mismatch: %+v", gsi)
	}
	// The actual point of this test: gsi1pk's attr_type ("N") is only
	// resolvable by cross-referencing AttributeDefinitions — a broken lookup
	// would silently leave this blank rather than fail.
	if len(gsi.KeySchema) != 1 || gsi.KeySchema[0] != (keyAttr{"gsi1pk", "HASH", "N"}) {
		t.Errorf("gsi key_schema mismatch, want [gsi1pk/HASH/N], got: %+v", gsi.KeySchema)
	}
}

// keyAttr and describeOutput mirror internal/output.KeyAttr/GSIView/TableInfo's
// JSON shape, kept local to the test so it decodes the command's actual
// stdout rather than asserting against the internal struct directly.
type keyAttr struct {
	Name     string `json:"name"`
	KeyType  string `json:"key_type"`
	AttrType string `json:"attr_type"`
}

type gsiView struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Projection string    `json:"projection"`
	KeySchema  []keyAttr `json:"key_schema"`
}

type describeOutput struct {
	TableName   string    `json:"table_name"`
	BillingMode string    `json:"billing_mode"`
	ItemCount   int64     `json:"item_count"`
	SizeBytes   int64     `json:"size_bytes"`
	KeySchema   []keyAttr `json:"key_schema"`
	GSIs        []gsiView `json:"gsis"`
}

// TestDescribeCmdNilBillingModeSummaryMeansProvisioned covers the legacy-mode
// branch: DynamoDB omits BillingModeSummary entirely for tables created under
// PROVISIONED billing before the summary field existed, and that nil must be
// interpreted as PROVISIONED rather than left blank.
func TestDescribeCmdNilBillingModeSummaryMeansProvisioned(t *testing.T) {
	origFactory := describeClientFactory
	defer func() { describeClientFactory = origFactory }()

	flagTable = "legacy-table"
	flagJSON = true

	fake := &fakeDescribeTableAPI{
		out: &awsdynamodb.DescribeTableOutput{
			Table: &ddbtypes.TableDescription{
				TableName:          aws.String("legacy-table"),
				TableStatus:        ddbtypes.TableStatusActive,
				BillingModeSummary: nil,
			},
		},
	}
	describeClientFactory = func(context.Context) (describeTableAPI, error) {
		return fake, nil
	}

	var out bytes.Buffer
	cmd := newDescribeCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var info describeOutput
	if err := json.Unmarshal(out.Bytes(), &info); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if info.BillingMode != "PROVISIONED" {
		t.Fatalf("nil BillingModeSummary should render as PROVISIONED, got: %q", info.BillingMode)
	}
}

// TestDescribeCmdAPIErrorIsWrapped covers the DescribeTable-call failure
// path, distinct from the factory-failure path the two tests above already
// cover — this is the error returned once a client exists but AWS rejects
// the call (e.g. table not found).
func TestDescribeCmdAPIErrorIsWrapped(t *testing.T) {
	origFactory := describeClientFactory
	defer func() { describeClientFactory = origFactory }()

	flagTable = "missing-table"
	flagJSON = false

	wantErr := errors.New("ResourceNotFoundException")
	fake := &fakeDescribeTableAPI{err: wantErr}
	describeClientFactory = func(context.Context) (describeTableAPI, error) {
		return fake, nil
	}

	var out bytes.Buffer
	cmd := newDescribeCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error from DescribeTable failure, got nil")
	}
	if !strings.Contains(err.Error(), "missing-table") || !strings.Contains(err.Error(), "ResourceNotFoundException") {
		t.Fatalf("error should name the table and wrap the cause, got: %v", err)
	}
}
