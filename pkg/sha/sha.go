package sha

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func JSON(content any) string {
	value, _ := json.Marshal(content)

	hasher := sha256.New()

	// no err check https://golang.org/pkg/hash/#Hash
	_, _ = hasher.Write(value)

	return hex.EncodeToString(hasher.Sum(nil))
}
