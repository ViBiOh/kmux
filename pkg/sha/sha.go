package sha

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// New get sha256 value of given interface
func New[T any](o T) string {
	hasher := sha256.New()

	// no err check https://golang.org/pkg/hash/#Hash
	_, _ = fmt.Fprintf(hasher, "%#v", o)

	return hex.EncodeToString(hasher.Sum(nil))
}
