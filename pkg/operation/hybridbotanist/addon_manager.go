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

package hybridbotanist

import (
	"path/filepath"

	"github.com/gardener/gardener/pkg/operation/common"
)

// DeployKubeAddonManager deploys the Kubernetes Addon Manager which will use labeled Kubernetes resources in order
// to ensure that they exist in a cluster/reconcile them in case somebody changed something.
func (b *HybridBotanist) DeployKubeAddonManager() error {
	var name = "kube-addon-manager"

	cloudConfig, err := b.generateCloudConfigExecutionChart()
	if err != nil {
		return err
	}
	storageClasses, err := b.generateStorageClassesChart()
	if err != nil {
		return err
	}
	coreAddons, err := b.generateCoreAddonsChart()
	if err != nil {
		return err
	}
	optionalAddons, err := b.generateOptionalAddonsChart()
	if err != nil {
		return err
	}

	defaultValues := map[string]interface{}{
		"cloudConfigContent":    cloudConfig.Files,
		"storageClassesContent": storageClasses.Files,
		"coreAddonsContent":     coreAddons.Files,
		"optionalAddonsContent": optionalAddons.Files,
		"podAnnotations": map[string]interface{}{
			"checksum/secret-kube-addon-manager": b.CheckSums[name],
		},
		"replicas": b.Shoot.GetReplicas(1),
		// we need to add capabilities for Shoot's API server.
		"shootAPIServer": map[string]interface{}{
			"Capabilities": map[string]interface{}{
				"KubeVersion": map[string]interface{}{
					"GitVersion": b.Shoot.Info.Spec.Kubernetes.Version,
				},
			},
		},
	}

	values, err := b.Botanist.ImageVector.InjectImages(defaultValues, b.SeedVersion(), b.ShootVersion(), common.KubeAddonManagerImageName)
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-controlplane", "charts", name), b.Shoot.SeedNamespace, name, values, nil)
}
