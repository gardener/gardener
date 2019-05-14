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

package localbotanist

import (
	"fmt"
	"strings"

	"github.com/gardener/gardener/pkg/operation/common"
)

// GenerateCloudProviderConfig returns a cloud provider config for the Local cloud provider.
// Not needed on Local.
func (b *LocalBotanist) GenerateCloudProviderConfig() (string, error) {
	return "", nil
}

// RefreshCloudProviderConfig refreshes the cloud provider credentials in the existing cloud
// provider config.
// Not needed on Local, hence, the original is returned back.
func (b *LocalBotanist) RefreshCloudProviderConfig(currentConfig map[string]string) map[string]string {
	return currentConfig
}

// GenerateKubeAPIServerServiceConfig generates the cloud provider specific values which are required to render the
// Service manifest of the kube-apiserver-service properly.
func (b *LocalBotanist) GenerateKubeAPIServerServiceConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"type":       "NodePort",
		"targetPort": 31443,
		"nodePort":   31443,
	}, nil
}

// GenerateKubeAPIServerExposeConfig defines the cloud provider specific values which configure how the kube-apiserver
// is exposed to the public.
func (b *LocalBotanist) GenerateKubeAPIServerExposeConfig() (map[string]interface{}, error) {
	if !strings.HasSuffix(*b.Shoot.Info.Spec.DNS.Domain, ".nip.io") {
		return nil, fmt.Errorf("missing `.nip.io` TLD")
	}
	return map[string]interface{}{
		"advertiseAddress": strings.Replace(strings.TrimSuffix(*b.Shoot.Info.Spec.DNS.Domain, ".nip.io"), "-", ".", -1),
		"securePort":       31443,
	}, nil
}

// GenerateKubeAPIServerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-apiserver properly.
func (b *LocalBotanist) GenerateKubeAPIServerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateCloudControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the cloud-controller-manager properly.
func (b *LocalBotanist) GenerateCloudControllerManagerConfig() (map[string]interface{}, string, error) {
	return nil, common.CloudControllerManagerDeploymentName, nil
}

// GenerateCSIConfig generates the configuration for CSI charts
func (b *LocalBotanist) GenerateCSIConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateKubeControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the kube-controller-manager properly.
func (b *LocalBotanist) GenerateKubeControllerManagerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateKubeSchedulerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-scheduler properly.
func (b *LocalBotanist) GenerateKubeSchedulerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateETCDStorageClassConfig generates values which are required to create etcd volume storageclass properly.
func (b *LocalBotanist) GenerateETCDStorageClassConfig() map[string]interface{} {
	return map[string]interface{}{
		"name":        "gardener.cloud-fast",
		"capacity":    "25Gi",
		"provisioner": "k8s.io/minikube-hostpath",
		"parameters":  map[string]interface{}{},
	}
}

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *LocalBotanist) GenerateEtcdBackupConfig() (map[string][]byte, map[string]interface{}, error) {
	backupConfigData := map[string]interface{}{
		"schedule":         b.Operation.ShootBackup.Schedule,
		"storageProvider":  "",
		"storageContainer": "/var/etcd/default.bkp",
		"env":              []map[string]interface{}{},
		"volumeMount":      []map[string]interface{}{},
	}

	return nil, backupConfigData, nil
}

// DeployCloudSpecificControlPlane does currently nothing for Local.
func (b *LocalBotanist) DeployCloudSpecificControlPlane() error {
	return nil
}
