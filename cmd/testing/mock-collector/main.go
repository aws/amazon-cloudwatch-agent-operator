// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	port               = "80"
	taServiceName      = "target-allocator-service"
	mainEndpoint       = "https://" + taServiceName + ":" + port // Update this with your main endpoint
	interval           = 30 * time.Second                        // Update this with your desired interval
	WAIT_BETWEEN_TESTS = 5 * time.Second
	MAX_FAILURE_RATE   = 0.1 //10%
)

// Function to create a custom HTTP client with a CA certificate
func createTLSClient(caCertFile string) (*http.Client, error) {
	// Load CA cert
	caCert, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read CA cert file: %v", err)
	}

	// Create a CA cert pool and add the CA cert to it
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
		return nil, fmt.Errorf("failed to append CA cert to pool")
	}

	// Configure the TLS transport
	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}
	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	client := &http.Client{
		Transport: tr,
	}
	return client, nil
}

// Function to run tests on endpoints
func testEndpoints(client *http.Client) float64 {
	endpoints := []string{
		"/scrape_configs",
		"/jobs/kubernetes-service-endpoints/targets",
		"/jobs",
		"/livez",
		"/readyz",
	}
	failCount := 0
	totalCount := len(endpoints)
	for _, endpoint := range endpoints {
		url := fmt.Sprintf("%s%s", mainEndpoint, endpoint)
		resp, err := client.Get(url)
		if err != nil {
			log.Printf("Error testing %s: %v", url, err)
			failCount++
			continue
		}
		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		log.Printf("Endpoint: %s, Status: %d, Response: %s", url, resp.StatusCode, string(body))
		time.Sleep(WAIT_BETWEEN_TESTS)
	}
	log.Printf("%d/%d Endpoint Test have succeded", totalCount-failCount, totalCount)
	failureRate := float64(failCount) / float64(totalCount)
	return failureRate
}

func main() {
	// Path to your CA certificate file
	caCertFile := "/etc/amazon-cloudwatch-observability-agent-cert/tls-ca.crt"
	client, err := createTLSClient(caCertFile)
	if err != nil {
		log.Fatalf("Failed to create TLS client: %v", err)
	}
	log.Println("Starting mock collector")
	var failureRate float64
	if failureRate = testEndpoints(client); failureRate > MAX_FAILURE_RATE {
		time.Sleep(1 * time.Minute) // Wait a minute for service to boot up
	}
	// Run tests at regular intervals
	for range time.Tick(interval) {
		log.Println("Running endpoint tests...")
		if failureRate = testEndpoints(client); failureRate > MAX_FAILURE_RATE {
			os.Exit(1)
		}
	}
}
