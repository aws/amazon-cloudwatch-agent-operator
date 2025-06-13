// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
)

type CertAndCAWatcher struct {
	certWatcher *certwatcher.CertWatcher

	caFilePath string
	caPool     *x509.CertPool
	caWatcher  *fsnotify.Watcher

	mu sync.RWMutex
}

func NewCertAndCAWatcher(certPath, keyPath, caPath string) (*CertAndCAWatcher, error) {
	certWatcher, err := certwatcher.New(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("error creating cert watcher: %w", err)
	}

	caPool, err := loadCAPool(caPath)
	if err != nil {
		return nil, fmt.Errorf("error loading CA pool: %w", err)
	}

	caWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("error creating CA file watcher: %w", err)
	}
	if err := caWatcher.Add(caPath); err != nil {
		return nil, fmt.Errorf("error adding CA file to watcher: %w", err)
	}

	return &CertAndCAWatcher{
		certWatcher: certWatcher,
		caFilePath:  caPath,
		caPool:      caPool,
		caWatcher:   caWatcher,
	}, nil
}

func loadCAPool(caPath string) (*x509.CertPool, error) {
	caCert, err := os.ReadFile(caPath)
	caCertPool := x509.NewCertPool()
	if err != nil {
		return nil, fmt.Errorf("error reading CA file: %w", err)
	}
	caCertPool.AppendCertsFromPEM(caCert)
	return caCertPool, nil
}

func (w *CertAndCAWatcher) Start(ctx context.Context) error {
	go func() {
		_ = w.certWatcher.Start(ctx)
	}()

	go w.watchCA(ctx)

	<-ctx.Done()
	return nil
}

func (w *CertAndCAWatcher) watchCA(ctx context.Context) {
	for {
		select {
		case event, ok := <-w.caWatcher.Events:
			if !ok {
				return
			}
			if event.Op.Has(fsnotify.Write) || event.Op.Has(fsnotify.Create) || event.Op.Has(fsnotify.Remove) {
				newPool, err := loadCAPool(w.caFilePath)
				if err != nil {
					continue
				}
				w.mu.Lock()
				w.caPool = newPool
				w.mu.Unlock()

				// needed incase file removed
				if event.Op.Has(fsnotify.Remove) {
					time.Sleep(100 * time.Millisecond)
					_ = w.caWatcher.Add(w.caFilePath)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (w *CertAndCAWatcher) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return w.certWatcher.GetCertificate(clientHello)
}

func (w *CertAndCAWatcher) GetCAPool() *x509.CertPool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.caPool
}
