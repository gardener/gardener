// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gcp

import (
	"context"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
)

// Client is a struct containing the client for the GCP service it needs to interact with.
type Client struct {
	computeService *compute.Service
}

// NewClient creates a new Client for the given GCP service account
func NewClient(ctx context.Context, serviceAccount []byte, projectID string) (ClientInterface, error) {
	computeService, err := createComputeService(ctx, serviceAccount)
	if err != nil {
		return nil, err
	}

	return &Client{computeService}, nil
}

// createComputeService initializes a compute service client and returns it
func createComputeService(ctx context.Context, serviceaccount []byte) (*compute.Service, error) {
	jwt, err := google.JWTConfigFromJSON(serviceaccount, compute.CloudPlatformScope)
	if err != nil {
		return nil, err
	}

	oauthClient := oauth2.NewClient(ctx, jwt.TokenSource(ctx))
	return compute.New(oauthClient)
}
