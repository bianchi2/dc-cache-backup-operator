package util

import (
	cachev1beta1 "bianchi2/dc-cache-backup-operator/api/v1beta1"
	"math/rand"
	"time"
)

const (
	charset          = "abcdefghijklmnopqrstuvwxyz0123456789"
	dateFormatLayout = "2006-01-02 15:04:05 -0700"
)

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

func IsBackupOutdated(cr *cachev1beta1.CacheBackupRequest) (outdated bool, err error) {

	layout := dateFormatLayout
	lastTransactionTimeStr := cr.Status.LastTransactionTime
	lastTransactionTime, err := time.Parse(layout, lastTransactionTimeStr)
	if err != nil {
		return false, err
	}
	interval := time.Duration(cr.Spec.BackupIntervalMinutes) * time.Minute
	currentTime := time.Now()

	if cr.Status.Status == "Succeeded" && currentTime.Sub(lastTransactionTime) < (interval) {
		return false, nil
	}
	return true, nil
}
