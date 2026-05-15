package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func loadDeviceRecord(path string, now time.Time) (DeviceRecord, error) {
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		record := DeviceRecord{
			SchemaVersion:  deviceRecordSchemaVersion,
			InstallationID: newInstallationID(),
			CreatedAt:      now.UTC(),
			ClaimState:     ClaimStateUnclaimed,
		}
		if err := saveDeviceRecord(path, record); err != nil {
			return DeviceRecord{}, err
		}
		return record, nil
	}
	if err != nil {
		return DeviceRecord{}, fmt.Errorf("read device record: %w", err)
	}

	var record DeviceRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return DeviceRecord{}, fmt.Errorf("parse device record: %w", err)
	}
	if record.InstallationID == "" {
		return DeviceRecord{}, fmt.Errorf("read device record: installation_id is empty")
	}

	changed := false
	if record.SchemaVersion == 0 {
		record.SchemaVersion = deviceRecordSchemaVersion
		record.ClaimState = ClaimStateUnclaimed
		if record.CreatedAt.IsZero() {
			record.CreatedAt = now.UTC()
		}
		changed = true
	}
	if record.ClaimState == "" {
		record.ClaimState = ClaimStateUnclaimed
		changed = true
	}
	if record.SchemaVersion != deviceRecordSchemaVersion {
		return DeviceRecord{}, fmt.Errorf("unsupported device record schema_version %d", record.SchemaVersion)
	}
	if changed {
		if err := saveDeviceRecord(path, record); err != nil {
			return DeviceRecord{}, err
		}
	}

	return record, nil
}

func saveDeviceRecord(path string, record DeviceRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create device record directory: %w", err)
	}
	payload, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode device record: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write device record: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("set device record permissions: %w", err)
	}
	return nil
}

func newInstallationID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("generate installation id: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

func writeSecretFile(path string, payload []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create secret directory: %w", err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write secret file: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("set secret file permissions: %w", err)
	}
	return nil
}

func readSecretFile(path string) ([]byte, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read secret file: %w", err)
	}
	return payload, nil
}

func removeIfExists(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func ensureDevicePrivateKey(path string) (*rsa.PrivateKey, error) {
	payload, err := os.ReadFile(path)
	if err == nil {
		block, _ := pem.Decode(payload)
		if block == nil || block.Type != "RSA PRIVATE KEY" {
			return nil, fmt.Errorf("parse device private key: invalid PEM")
		}
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse device private key: %w", err)
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read device private key: %w", err)
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate device private key: %w", err)
	}
	payload = pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err := writeSecretFile(path, payload); err != nil {
		return nil, err
	}
	return key, nil
}

func newCSRPEM(key *rsa.PrivateKey, installationID string) (string, error) {
	template := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: installationID,
		},
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &template, key)
	if err != nil {
		return "", fmt.Errorf("create csr: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})), nil
}
