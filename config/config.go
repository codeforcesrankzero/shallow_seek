package config

import (
	"os"
	"time"
)

var (
	StartTime = time.Now()
	APIKeys   = map[string]bool{
		"test-key": true,
	}
)

func GetPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		return "8080"
	}
	return port
}

func GetElasticsearchURL() string {
	url := os.Getenv("ELASTICSEARCH_URL")
	if url == "" {
		return "http://localhost:9200"
	}
	return url
}

func GetRedisURL() string {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		return "localhost:6379"
	}
	return url
} 