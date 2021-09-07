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

	// LoggingUserName is the admin user name for the loki instance of a shoot
	lokiLogging = "loki"
	lokiPort    = 3100

	// IntegrationTestPrefix is the default prefix that will be used for test shoots if none other is specified
	IntegrationTestPrefix = "itest-"

	// WorkerNamePrefix is the default prefix that will be used for Shoot workers
	WorkerNamePrefix = "worker-"

	// TestMachineryKubeconfigsPathEnvVarName is the name of the environment variable that holds the path to the
	// testmachinery provided kubeconfigs.
	TestMachineryKubeconfigsPathEnvVarName = "TM_KUBECONFIG_PATH"

	// TestMachineryTestRunIDEnvVarName is the name of the environment variable that holds the testrun ID.
	TestMachineryTestRunIDEnvVarName = "TM_TESTRUN_ID"

	// SeedTaintTestRun is the taint used to limit shoots that can be scheduled on a seed to shoots created by the same testrun.
	SeedTaintTestRun = "test.gardener.cloud/test-run"
)

// SearchResponse represents the response from a search query to loki
type SearchResponse struct {
	Data struct {
		Result []struct {
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}
