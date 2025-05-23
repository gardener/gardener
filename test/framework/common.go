// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"time"
)

const (
	k8sClientInitPollInterval = 20 * time.Second
	k8sClientInitTimeout      = 5 * time.Minute
	defaultPollInterval       = 5 * time.Second

	// KubeconfigSecretKeyName is the name of the key in a secret that holds the kubeconfig of a shoot
	KubeconfigSecretKeyName = "kubeconfig"

	// LoggingUserName is the admin user name for the vali instance of a shoot
	valiLogging = "vali"
	valiPort    = 3100

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

// SearchResponse represents the response from a search query to vali
type SearchResponse struct {
	Data struct {
		Result []struct {
			Value []any `json:"value"`
		} `json:"result"`
	} `json:"data"`
}
