package id

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// New generates a random 16-byte hex-encoded identifier.
func New() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes[:])
}
