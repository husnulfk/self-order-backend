package util

import (
	"crypto/rand"
	"math/big"
)

const (
	alphaNum = "ABCDEFGHJKLMNPQRSTUVWXYZ0123456789"
	urlSafe  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
)

// RandomString returns a cryptographically-secure random string of length n.
func RandomString(n int, charset string) string {
	b := make([]byte, n)
	max := big.NewInt(int64(len(charset)))
	for i := range b {
		x, err := rand.Int(rand.Reader, max)
		if err != nil {
			b[i] = charset[i%len(charset)]
			continue
		}
		b[i] = charset[x.Int64()]
	}
	return string(b)
}

// OrderCode returns a human-friendly order code, e.g. "ORD-7K3M9Q".
func OrderCode() string {
	return "ORD-" + RandomString(6, alphaNum)
}

// TableToken returns an unguessable table token used in the QR URL /t/:token.
func TableToken() string {
	return RandomString(20, urlSafe)
}

// RandomID returns a prefixed random id, e.g. "MOCK-XXXXXXXXXXXX".
func RandomID(prefix string) string {
	return prefix + "-" + RandomString(12, alphaNum)
}
