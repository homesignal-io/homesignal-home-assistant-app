package secrets

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func TestReadString(t *testing.T) {
	client := &fakeSecretsManagerClient{value: "secret-value"}
	got, err := ReadString(context.Background(), client, "secret-id")
	if err != nil {
		t.Fatalf("ReadString returned error: %v", err)
	}
	if got != "secret-value" {
		t.Fatalf("secret = %q, want secret-value", got)
	}
	if client.secretID != "secret-id" {
		t.Fatalf("secret id = %q, want secret-id", client.secretID)
	}
}

func TestReadStringRejectsEmptySecret(t *testing.T) {
	_, err := ReadString(context.Background(), &fakeSecretsManagerClient{}, "secret-id")
	if err == nil {
		t.Fatal("expected empty secret error")
	}
}

type fakeSecretsManagerClient struct {
	value    string
	secretID string
}

func (c *fakeSecretsManagerClient) GetSecretValue(_ context.Context, params *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	if params.SecretId != nil {
		c.secretID = *params.SecretId
	}
	return &secretsmanager.GetSecretValueOutput{
		SecretString: aws.String(c.value),
	}, nil
}
