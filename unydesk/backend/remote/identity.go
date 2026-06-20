package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func StableID(kind, seed string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(kind) + ":" + strings.TrimSpace(seed)))
	return hex.EncodeToString(sum[:8])
}

func StablePublicID(kind, seed string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(kind) + ":" + strings.TrimSpace(seed)))
	digits := make([]byte, 9)
	for i := range digits {
		digits[i] = '0' + (sum[i] % 10)
	}
	return string(digits[0:3]) + " " + string(digits[3:6]) + " " + string(digits[6:9])
}
