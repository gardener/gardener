// Copyright 2019 Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package framework

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/pkg/errors"
	"net/http"
	"time"
)

// HTTPGet performs an HTTP GET request with context
func HTTPGet(ctx context.Context, url string) (*http.Response, error) {
	httpClient := http.Client{}
	httpRequest, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	httpRequest = httpRequest.WithContext(ctx)
	return httpClient.Do(httpRequest)
}

// TestHTTPEndpointWithBasicAuth validates that a http endpoint can be accessed using basic authentication
func TestHTTPEndpointWithBasicAuth(ctx context.Context, url, userName, password string) error {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	httpClient := http.Client{
		Transport: transport,
		Timeout:   time.Duration(5 * time.Second),
	}

	httpRequest, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	httpRequest.SetBasicAuth(userName, password)
	httpRequest = httpRequest.WithContext(ctx)

	r, err := httpClient.Do(httpRequest)
	if err != nil {
		return err
	}

	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("dashboard unavailable")
	}

	return nil
}

// TestHTTPEndpointWithToken validates that a http endpoint can be accessed using a bearer token
func TestHTTPEndpointWithToken(ctx context.Context, url, token string) error {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	httpClient := http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	httpRequest, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	bearerToken := fmt.Sprintf("Bearer %s", token)
	httpRequest.Header.Set("Authorization", bearerToken)
	httpRequest = httpRequest.WithContext(ctx)

	r, err := httpClient.Do(httpRequest)
	if err != nil {
		return err
	}
	if r.StatusCode != http.StatusOK {
		return errors.Errorf("http request should return %d but returned %d instead", http.StatusOK, r.StatusCode)
	}
	return nil
}
