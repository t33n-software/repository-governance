package canonical

import (
	"crypto/sha256"
	"encoding/hex"
)

// Sum256Hex returns the lowercase hexadecimal SHA-256 digest of contents.
func Sum256Hex(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}

// SumReader hashes the contents read through the injected read seam so the
// error path stays whitebox-testable.
func SumReader(read func(string) ([]byte, error), path string) (string, error) {
	contents, err := read(path)
	if err != nil {
		return "", err
	}
	return Sum256Hex(contents), nil
}
