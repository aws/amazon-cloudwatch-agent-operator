package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

var (
	port               = "80"
	mainEndpoint       = "https://cloudwatch-agent-w-prom-target-allocator:" + port // Update this with your main endpoint
	interval           = 1 * time.Minute                                            // Update this with your desired interval
	WAIT_BETWEEN_TESTS = 5 * time.Second
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
func testEndpoints(client *http.Client) {
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
}

func main() {
	// Path to your CA certificate file
	caCertFile := "/etc/amazon-cloudwatch-observability-agent-cert/tls-ca.crt"
	client, err := createTLSClient(caCertFile)
	if err != nil {
		log.Fatalf("Failed to create TLS client: %v", err)
	}
	log.Println("Starting mock collector")
	testEndpoints(client)
	// Run tests at regular intervals
	for range time.Tick(interval) {
		log.Println("Running endpoint tests...")
		testEndpoints(client)
	}
}
