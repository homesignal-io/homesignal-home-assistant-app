package secrets

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type SecretsManagerClient interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

func NewSecretsManagerClient(ctx context.Context, region string) (SecretsManagerClient, error) {
	options := []func(*awsconfig.LoadOptions) error{}
	if strings.TrimSpace(region) != "" {
		options = append(options, awsconfig.WithRegion(region))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config for Secrets Manager: %w", err)
	}
	return secretsmanager.NewFromConfig(cfg), nil
}

func ReadString(ctx context.Context, client SecretsManagerClient, secretID string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("Secrets Manager client is required")
	}
	secretID = strings.TrimSpace(secretID)
	if secretID == "" {
		return "", fmt.Errorf("secret id is required")
	}
	out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretID),
	})
	if err != nil {
		return "", fmt.Errorf("get secret value: %w", err)
	}
	if out.SecretString == nil || strings.TrimSpace(*out.SecretString) == "" {
		return "", fmt.Errorf("secret value is empty")
	}
	return *out.SecretString, nil
}
