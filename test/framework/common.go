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

import "time"

const (
	k8sClientInitPollInterval = 20 * time.Second
	k8sClientInitTimeout      = 5 * time.Minute
	defaultPollInterval       = 5 * time.Second

	// KubeconfigSecretKeyName ist the name of the key in a secret that holds the kubeconfig of a shoot
	KubeconfigSecretKeyName = "kubeconfig"

	// LoggingUserName is the admin user name for the elasticserach logging instance of a shoot
	LoggingUserName           = "admin"
	loggingIngressCredentials = "logging-ingress-credentials"
	elasticsearchLogging      = "elasticsearch-logging"
	elasticsearchPort         = 9200

	// IntegrationTestPrefix is the default prefix that will be used for test shoots if none other is specified
	IntegrationTestPrefix = "itest-"

	// WorkerNamePrefix is the default prefix that will be used for Shoot workers
	WorkerNamePrefix = "worker-"
)

// SearchResponse represents the response from a search query to elasticsearch
type SearchResponse struct {
	Hits struct {
		Total uint64 `json:"total"`
	} `json:"hits"`
}
