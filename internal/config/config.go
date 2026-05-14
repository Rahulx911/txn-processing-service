package config

import (
	"os"
	"time"
)

// Config holds all application configuration.
type Config struct {
	Port           string
	Environment    string
	AWSRegion      string
	DynamoDBTable  string
	IdempotencyTTL time.Duration
	SQSQueueURL    string
	SNSTopicARN    string
	LogLevel       string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8080"),
		Environment:    getEnv("ENVIRONMENT", "development"),
		AWSRegion:      getEnv("AWS_REGION", "ap-south-1"),
		DynamoDBTable:  getEnv("DYNAMODB_TABLE", "transactions"),
		IdempotencyTTL: parseDuration(getEnv("IDEMPOTENCY_TTL", "24h")),
		SQSQueueURL:    getEnv("SQS_QUEUE_URL", ""),
		SNSTopicARN:    getEnv("SNS_TOPIC_ARN", ""),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 24 * time.Hour
	}
	return d
}
