package idempotency

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahuljain/txn-processing-service/internal/config"
)

// CachedResponse holds a previously computed API response.
type CachedResponse struct {
	StatusCode int    `json:"status_code"`
	Body       []byte `json:"body"`
}

// Store defines the contract for idempotency key persistence.
type Store interface {
	Get(ctx context.Context, key string) (*CachedResponse, error)
	Set(ctx context.Context, key string, response *CachedResponse) error
}

// DynamoDBStore implements Store using a DynamoDB table with TTL.
type DynamoDBStore struct {
	client    *dynamodb.Client
	tableName string
	ttl       time.Duration
}

// NewDynamoDBStore creates a new idempotency store.
func NewDynamoDBStore(cfg *config.Config) (*DynamoDBStore, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.AWSRegion),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &DynamoDBStore{
		client:    dynamodb.NewFromConfig(awsCfg),
		tableName: fmt.Sprintf("%s-idempotency", cfg.DynamoDBTable),
		ttl:       cfg.IdempotencyTTL,
	}, nil
}

// Get retrieves a cached response by idempotency key.
func (s *DynamoDBStore) Get(ctx context.Context, key string) (*CachedResponse, error) {
	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: key},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get idempotency key: %w", err)
	}

	if result.Item == nil {
		return nil, nil
	}

	statusAttr, ok := result.Item["status_code"].(*types.AttributeValueMemberN)
	if !ok {
		return nil, fmt.Errorf("invalid cached response format")
	}

	bodyAttr, ok := result.Item["body"].(*types.AttributeValueMemberB)
	if !ok {
		return nil, fmt.Errorf("invalid cached response body")
	}

	var statusCode int
	fmt.Sscanf(statusAttr.Value, "%d", &statusCode)

	return &CachedResponse{
		StatusCode: statusCode,
		Body:       bodyAttr.Value,
	}, nil
}

// Set stores a response with a TTL for automatic expiration.
func (s *DynamoDBStore) Set(ctx context.Context, key string, response *CachedResponse) error {
	ttlEpoch := time.Now().Add(s.ttl).Unix()

	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item: map[string]types.AttributeValue{
			"PK":          &types.AttributeValueMemberS{Value: key},
			"status_code": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", response.StatusCode)},
			"body":        &types.AttributeValueMemberB{Value: response.Body},
			"ttl":         &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", ttlEpoch)},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to cache idempotent response: %w", err)
	}

	return nil
}
