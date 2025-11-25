package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// GenerateAuthCode generates a 6-digit numeric code
func GenerateAuthCode() (string, error) {
	code := ""
	for i := 0; i < 6; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		code += fmt.Sprintf("%d", n.Int64())
	}
	return code, nil
}
