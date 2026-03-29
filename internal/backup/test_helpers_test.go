package backup

import "testing"

const (
	testTable     = "t"
	testBucket    = "b"
	testNamespace = "ns"
	testKey       = "k"
)

func requireNoErr(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}
