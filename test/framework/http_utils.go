// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

// HTTPGet performs an HTTP GET request with context
func HTTPGet(ctx context.Context, url string) (*http.Response, error) {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	httpClient := http.Client{
		Transport: transport,
	}
	httpRequest, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	httpRequest = httpRequest.WithContext(ctx)
	return httpClient.Do(httpRequest)
}

// TestHTTPEndpointWithBasicAuth validates that a http endpoint can be accessed using basic authentication
func TestHTTPEndpointWithBasicAuth(ctx context.Context, url, userName, password string) error {
	return testHTTPEndpointWith(ctx, url, func(r *http.Request) {
		r.SetBasicAuth(userName, password)
	})
}

// TestHTTPEndpointWithToken validates that a http endpoint can be accessed using a bearer token
func TestHTTPEndpointWithToken(ctx context.Context, url, token string) error {
	return testHTTPEndpointWith(ctx, url, func(r *http.Request) {
		bearerToken := fmt.Sprintf("Bearer %s", token)
		r.Header.Set("Authorization", bearerToken)
	})
}

func testHTTPEndpointWith(ctx context.Context, url string, mutator func(*http.Request)) error {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec: G402 -- Test only.
		Proxy:           http.ProxyFromEnvironment,
	}

	httpClient := http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	httpRequest, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	mutator(httpRequest)
	httpRequest = httpRequest.WithContext(ctx)

	r, err := httpClient.Do(httpRequest)
	if err != nil {
		return err
	}
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("http request should return %d but returned %d instead", http.StatusOK, r.StatusCode)
	}
	return nil
}
