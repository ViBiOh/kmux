package sha

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// New get sha256 value of given interface
func New(content any) string {
	hasher := sha256.New()

	// no err check https://golang.org/pkg/hash/#Hash
	_, _ = fmt.Fprintf(hasher, "%#v", content)

	return hex.EncodeToString(hasher.Sum(nil))
}

func JSON(content any) string {
	value, _ := json.Marshal(content)
	return New(value)
}
