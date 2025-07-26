// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	"k8s.io/component-base/version"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

// GetDefaultConfigFilePath returns the default file path on the worker node that contains the configuration of the gardener-node-agent.
func GetDefaultConfigFilePath() string {
	return GetConfigFilePath(nodeagentconfigv1alpha1.BaseDir)
}

// GetConfigFilePath generates the file path on the worker node that contains the configuration of the gardener-node-agent with a baseDir.
func GetConfigFilePath(baseDir string) string {
	return fmt.Sprintf("%s/config-%s.yaml", baseDir, version.Get().GitVersion)
}
