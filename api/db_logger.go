package main

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/google/uuid"
)

// RequestLogger handles logging to DynamoDB
type RequestLogger struct {
	svc       *dynamodb.DynamoDB
	tableName string
}

// NewRequestLogger creates a new instance of RequestLogger
func NewRequestLogger(svc *dynamodb.DynamoDB, tableName string) *RequestLogger {
	return &RequestLogger{
		svc:       svc,
		tableName: tableName,
	}
}

// LogRequest saves the request details to DynamoDB
func (rl *RequestLogger) LogRequest(method, path, remoteIP, body string, status int) {
	if rl.tableName == "" {
		return
	}

	sanitizedBody := rl.sanitizeBody(body)

	requestLog := RequestLog{
		RequestID: uuid.New().String(),
		Timestamp: time.Now().Unix(),
		Method:    method,
		Path:      path,
		RemoteIP:  remoteIP,
		Body:      sanitizedBody,
		Status:    status,
	}

	av, err := dynamodbattribute.MarshalMap(requestLog)
	if err != nil {
		log.Printf("Error marshalling request log: %v", err)
		return
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(rl.tableName),
	}

	_, err = rl.svc.PutItem(input)
	if err != nil {
		log.Printf("Error putting request log to DynamoDB: %v", err)
	}
}

// sanitizeBody removes sensitive information and cleans the body string
func (rl *RequestLogger) sanitizeBody(body string) string {
	if body == "" {
		return ""
	}

	// Try to parse as JSON to sanitize specific keys
	var data map[string]any
	err := json.Unmarshal([]byte(body), &data)
	if err == nil {
		// List of sensitive keys to redact
		sensitiveKeys := []string{"password", "token", "auth", "secret", "key", "salt"}
		rl.redactMap(data, sensitiveKeys)

		sanitizedJSON, err := json.Marshal(data)
		if err == nil {
			return string(sanitizedJSON)
		}
	}

	// If not JSON or failed to re-marshal, perform simple string-based redaction
	// This is a backup for non-JSON bodies or malformed JSON
	lowerBody := strings.ToLower(body)
	sensitivePatterns := []string{"password", "token", "secret"}
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerBody, pattern) {
			return "[REDACTED - Sensitive content detected]"
		}
	}

	// Basic string cleaning: limit length and remove potentially harmful chars
	const maxLogLength = 2000
	if len(body) > maxLogLength {
		body = body[:maxLogLength] + "... [TRUNCATED]"
	}

	return body
}

// redactMap recursively redacts sensitive keys in a map
func (rl *RequestLogger) redactMap(data map[string]any, sensitiveKeys []string) {
	for k, v := range data {
		lowerK := strings.ToLower(k)
		isSensitive := false
		for _, sk := range sensitiveKeys {
			if strings.Contains(lowerK, sk) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			data[k] = "[REDACTED]"
			continue
		}

		// Handle nested maps
		if nextMap, ok := v.(map[string]any); ok {
			rl.redactMap(nextMap, sensitiveKeys)
		}

		// Handle slices of maps
		if nextSlice, ok := v.([]any); ok {
			for _, item := range nextSlice {
				if itemMap, ok := item.(map[string]any); ok {
					rl.redactMap(itemMap, sensitiveKeys)
				}
			}
		}
	}
}
