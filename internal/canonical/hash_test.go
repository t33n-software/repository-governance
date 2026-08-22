package canonical

import (
	"errors"
	"testing"
)

func TestSum256Hex(t *testing.T) {
	// The empty SHA-256 digest is a well-known constant.
	if got := Sum256Hex(nil); got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("Sum256Hex(nil) = %q", got)
	}
	if Sum256Hex([]byte("a")) == Sum256Hex([]byte("b")) {
		t.Fatal("distinct contents must produce distinct digests")
	}
}

func TestSumReader(t *testing.T) {
	read := func(path string) ([]byte, error) {
		if path != "file.txt" {
			t.Fatalf("path = %q", path)
		}
		return []byte("content"), nil
	}
	hash, err := SumReader(read, "file.txt")
	if err != nil {
		t.Fatalf("SumReader: %v", err)
	}
	if hash != Sum256Hex([]byte("content")) {
		t.Fatalf("hash = %q", hash)
	}
}

func TestSumReaderError(t *testing.T) {
	boom := errors.New("boom")
	read := func(string) ([]byte, error) { return nil, boom }
	if _, err := SumReader(read, "file.txt"); !errors.Is(err, boom) {
		t.Fatalf("expected the read error, got %v", err)
	}
}
