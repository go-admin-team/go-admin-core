package utils

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

const (
	// RequestIDKey requestID key
	RequestIDKey = "x-request-id"
	// UsernameKey username key
	UsernameKey = "x-username"
)

// GetRequestID request id from header
func GetRequestID(ctx context.Context) string {
	id := GetHeaderFirst(ctx, RequestIDKey)
	if id == "" {
		id = NewRequestID()
	}
	return id
}

// GetUsername get username from header
func GetUsername(ctx context.Context) string {
	return GetHeaderFirst(ctx, UsernameKey)
}

// GetHeaderFirst get header first value
func GetHeaderFirst(ctx context.Context, key string) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(key); len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

// NewRequestID generate a RequestId
func NewRequestID() string {
	return uuid.New().String()
}
