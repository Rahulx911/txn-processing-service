package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahuljain/txn-processing-service/internal/config"
	"github.com/rahuljain/txn-processing-service/internal/model"
)

var (
	ErrNotFound        = errors.New("transaction not found")
	ErrConflict        = errors.New("optimistic locking conflict: transaction was modified concurrently")
	ErrInvalidTransition = errors.New("invalid state transition")
)

// TransactionRepository defines the contract for transaction persistence.
type TransactionRepository interface {
	Create(ctx context.Context, txn *model.Transaction) error
	GetByID(ctx context.Context, id string) (*model.Transaction, error)
	List(ctx context.Context, limit int, lastKey string) ([]*model.Transaction, string, error)
	UpdateStatus(ctx context.Context, id string, newStatus model.TransactionStatus, expectedVersion int) error
	Ping(ctx context.Context) error
}

// DynamoDBRepository implements TransactionRepository using DynamoDB.
type DynamoDBRepository struct {
	client    *dynamodb.Client
	tableName string
}

// NewDynamoDBRepository creates a new DynamoDB-backed repository.
func NewDynamoDBRepository(cfg *config.Config) (*DynamoDBRepository, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.AWSRegion),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS config: %w", err)
	}

	client := dynamodb.NewFromConfig(awsCfg)

	return &DynamoDBRepository{
		client:    client,
		tableName: cfg.DynamoDBTable,
	}, nil
}

// Create inserts a new transaction into DynamoDB.
// Uses a condition expression to prevent overwriting existing records.
func (r *DynamoDBRepository) Create(ctx context.Context, txn *model.Transaction) error {
	item, err := attributevalue.MarshalMap(txn)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %w", err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	if err != nil {
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			return fmt.Errorf("transaction %s already exists: %w", txn.ID, ErrConflict)
		}
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	return nil
}

// GetByID retrieves a transaction by its primary key.
func (r *DynamoDBRepository) GetByID(ctx context.Context, id string) (*model.Transaction, error) {
	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: id},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	if result.Item == nil {
		return nil, ErrNotFound
	}

	var txn model.Transaction
	if err := attributevalue.UnmarshalMap(result.Item, &txn); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
	}

	return &txn, nil
}

// List retrieves transactions with pagination support.
func (r *DynamoDBRepository) List(ctx context.Context, limit int, lastKey string) ([]*model.Transaction, string, error) {
	input := &dynamodb.ScanInput{
		TableName: aws.String(r.tableName),
		Limit:     aws.Int32(int32(limit)),
	}

	if lastKey != "" {
		input.ExclusiveStartKey = map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: lastKey},
		}
	}

	result, err := r.client.Scan(ctx, input)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list transactions: %w", err)
	}

	transactions := make([]*model.Transaction, 0, len(result.Items))
	for _, item := range result.Items {
		var txn model.Transaction
		if err := attributevalue.UnmarshalMap(item, &txn); err != nil {
			return nil, "", fmt.Errorf("failed to unmarshal transaction: %w", err)
		}
		transactions = append(transactions, &txn)
	}

	var nextKey string
	if result.LastEvaluatedKey != nil {
		if pk, ok := result.LastEvaluatedKey["PK"].(*types.AttributeValueMemberS); ok {
			nextKey = pk.Value
		}
	}

	return transactions, nextKey, nil
}

// UpdateStatus transitions a transaction to a new status using optimistic locking.
// The expectedVersion parameter prevents race conditions by ensuring the record
// hasn't been modified since it was last read.
func (r *DynamoDBRepository) UpdateStatus(ctx context.Context, id string, newStatus model.TransactionStatus, expectedVersion int) error {
	now := time.Now().UTC()

	updateExpr := "SET #status = :newStatus, #version = :newVersion, #updated = :now"
	exprValues := map[string]types.AttributeValue{
		":newStatus":       &types.AttributeValueMemberS{Value: string(newStatus)},
		":newVersion":      &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expectedVersion+1)},
		":expectedVersion": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expectedVersion)},
		":now":             &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
	}

	// Add settled_at timestamp for settled transactions
	if newStatus == model.StatusSettled {
		updateExpr += ", #settled = :settledAt"
		exprValues[":settledAt"] = &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)}
	}

	exprNames := map[string]string{
		"#status":  "status",
		"#version": "version",
		"#updated": "updated_at",
	}
	if newStatus == model.StatusSettled {
		exprNames["#settled"] = "settled_at"
	}

	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: id},
		},
		UpdateExpression:          aws.String(updateExpr),
		ConditionExpression:       aws.String("#version = :expectedVersion"),
		ExpressionAttributeValues: exprValues,
		ExpressionAttributeNames:  exprNames,
	})
	if err != nil {
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			return ErrConflict
		}
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	return nil
}

// Ping verifies DynamoDB connectivity.
func (r *DynamoDBRepository) Ping(ctx context.Context) error {
	_, err := r.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(r.tableName),
	})
	return err
}
