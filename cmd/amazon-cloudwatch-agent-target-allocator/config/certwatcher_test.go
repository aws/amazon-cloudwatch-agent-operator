// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// certwatcher_test.go
package config

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateSelfSignedCertAndKey returns PEM-encoded cert and key suitable for testing.
func generateSelfSignedCertAndKey(commonName string) (certPEM, keyPEM []byte, err error) {
	// Generate RSA key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// Create a minimal self-signed certificate template
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<63-1))
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour), // short validity is fine for tests

		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		IsCA:                  true, // mark true so it can be used as a CA
		BasicConstraintsValid: true,
	}

	// Self-sign the certificate
	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	// Encode cert + key to PEM
	var certBuf, keyBuf bytes.Buffer
	pem.Encode(&certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	pem.Encode(&keyBuf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return certBuf.Bytes(), keyBuf.Bytes(), nil
}

func TestCertAndCAWatcher_UpdatesCA(t *testing.T) {
	t.Parallel()

	// Generate a server cert/key for certwatcher
	certPEM, keyPEM, err := generateSelfSignedCertAndKey("test-server")
	if err != nil {
		t.Fatalf("failed to generate server cert/key: %v", err)
	}

	// Generate two distinct self-signed certs to represent old CA vs new CA
	oldCAPEM, _, err := generateSelfSignedCertAndKey("old-ca")
	if err != nil {
		t.Fatalf("failed to generate old CA: %v", err)
	}
	newCAPEM, _, err := generateSelfSignedCertAndKey("new-ca")
	if err != nil {
		t.Fatalf("failed to generate new CA: %v", err)
	}

	// Write all these PEM files into a temp dir
	tmpDir := t.TempDir()

	certPath := filepath.Join(tmpDir, "tls.crt")
	keyPath := filepath.Join(tmpDir, "tls.key")
	caPath := filepath.Join(tmpDir, "ca.crt")

	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatalf("failed to write cert file: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}
	if err := os.WriteFile(caPath, oldCAPEM, 0600); err != nil {
		t.Fatalf("failed to write initial CA file: %v", err)
	}

	// Create the CertAndCAWatcher using our files
	watcher, err := NewCertAndCAWatcher(certPath, keyPath, caPath)
	if err != nil {
		t.Fatalf("failed to create CertAndCAWatcher: %v", err)
	}

	// Start the watcher in the background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = watcher.Start(ctx)
	}()

	// Record the initial CA pool pointer
	oldPool := watcher.GetCAPool()
	if oldPool == nil {
		t.Fatal("expected non-nil initial CA pool")
	}

	// Overwrite the CA file with newCAPEM, triggering a reload
	if err := os.WriteFile(caPath, newCAPEM, 0600); err != nil {
		t.Fatalf("failed to write new CA file: %v", err)
	}

	// Loop until the watcher updates the CA pool (or times out)
	deadline := time.Now().Add(2 * time.Second)
	for {
		newPool := watcher.GetCAPool()
		if newPool != oldPool {
			t.Log("CA pool successfully updated.")
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for CA pool to be updated")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
