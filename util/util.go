package util

import (
	"math/rand"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

func generateRandomString(length int) string {
	rand.Seed(time.Now().UnixNano())

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}

	return string(b)
}

// GenerateK8sCompliantName returns a string containing letters and numbers
func GenerateK8sCompliantName(prefix string, length int) string {
	return prefix + "-" + generateRandomString(length)
}
