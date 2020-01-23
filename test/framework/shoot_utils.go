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
	"encoding/json"
	"fmt"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/test/integration/framework"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"strings"
	"time"
)

// ShootSeedNamespace gets the shoot namespace in the seed
func (f *ShootFramework) ShootSeedNamespace() string {
	return computeTechnicalID(f.Project.Name, f.Shoot)
}

// ShootKubeconfigSecretName gets the name of the secret with the kubeconfig of the shoot
func (f *ShootFramework) ShootKubeconfigSecretName() string {
	return fmt.Sprintf("%s.kubeconfig", f.Shoot.GetName())
}

// GetLoggingPassword returns the passwort to access the elasticseerach logging instance
func (f *ShootFramework) GetLoggingPassword(ctx context.Context) (string, error) {
	return framework.GetObjectFromSecret(ctx, f.SeedClient, f.ShootSeedNamespace(), loggingIngressCredentials, "password")
}

// GetElasticsearchLogs gets logs for <podName> from the elasticsearch instance in <elasticsearchNamespace>
func (f *ShootFramework) GetElasticsearchLogs(ctx context.Context, elasticsearchNamespace, podName string, client kubernetes.Interface) (*SearchResponse, error) {
	elasticsearchLabels := labels.SelectorFromSet(labels.Set(map[string]string{
		"app":  elasticsearchLogging,
		"role": "logging",
	}))

	now := time.Now()
	index := fmt.Sprintf("logstash-admin-%d.%02d.%02d", now.Year(), now.Month(), now.Day())
	loggingPassword, err := f.GetLoggingPassword(ctx)

	if err != nil {
		return nil, err
	}

	command := fmt.Sprintf("curl http://localhost:%d/%s/_search?q=kubernetes.pod_name:%s --user %s:%s", elasticsearchPort, index, podName, LoggingUserName, loggingPassword)
	var reader io.Reader
	err = retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (bool, error) {
		reader, err = PodExecByLabel(ctx, elasticsearchLabels, elasticsearchLogging, command, elasticsearchNamespace, client)
		if err != nil {
			return retry.MinorError(err)
		}
		return retry.Ok()
	})
	if err != nil {
		return nil, err
	}

	search := &SearchResponse{}
	if err = json.NewDecoder(reader).Decode(search); err != nil {
		return nil, err
	}

	return search, nil
}

// DumpState dumps the state of a shoot
// The state includes all k8s components running in the shoot itself as well as the controlplane
func (f *ShootFramework) DumpState(ctx context.Context) {
	if f.DisableStateDump {
		return
	}
	if f.Shoot != nil && f.ShootClient != nil {
		ctxIdentifier := fmt.Sprintf("[SHOOT %s]", f.Shoot.Name)
		f.Logger.Info(ctxIdentifier)
		if err := f.dumpDefaultResourcesInAllNamespaces(ctx, ctxIdentifier, f.ShootClient); err != nil {
			f.Logger.Errorf("unable to dump resources from all namespaces in shoot %s: %s", f.Shoot.Name, err.Error())
		}
		if err := f.dumpNodes(ctx, ctxIdentifier, f.ShootClient); err != nil {
			f.Logger.Errorf("unable to dump information of nodes from shoot %s: %s", f.Shoot.Name, err.Error())
		}
	}

	//dump controlplane in the shoot namespace
	if f.Seed != nil && f.SeedClient != nil {
		if err := f.dumpControlplaneInSeed(ctx, f.Seed, f.ShootSeedNamespace()); err != nil {
			f.Logger.Errorf("unable to dump controlplane of %s in seed %s: %v", f.Shoot.Name, f.Seed.Name, err)
		}
	}

	ctxIdentifier := "[GARDENER]"
	f.Logger.Info(ctxIdentifier)
	if f.Shoot != nil {

		project, err := f.GetShootProject(ctx, f.Shoot.GetNamespace())
		if err != nil {
			f.Logger.Errorf("unable to get project namespace of shoot %s: %s", f.Shoot.GetNamespace(), err.Error())
			return
		}

		err = f.dumpEventsInNamespace(ctx, ctxIdentifier, f.GardenClient, *project.Spec.Namespace, func(event corev1.Event) bool {
			return event.InvolvedObject.Name == f.Shoot.Name
		})
		if err != nil {
			f.Logger.Errorf("unable to dump Events from project namespace %s in gardener: %s", *project.Spec.Namespace, err.Error())
		}
	}
}

// CreateShootTestArtifacts creates a shoot object from the given path and sets common attributes (test-individual settings like workers have to be handled by each test).
func CreateShootTestArtifacts(shootTestYamlPath string, prefix *string, projectNamespace, shootRegion, cloudProfile, secretbinding, providerType, k8sVersion, externalDomain *string, clearDNS bool, clearExtensions bool) (string, *gardencorev1beta1.Shoot, error) {
	shoot := &gardencorev1beta1.Shoot{}
	if shootTestYamlPath != "" {
		if err := ReadObject(shootTestYamlPath, shoot); err != nil {
			return "", nil, err
		}
	}

	if shootRegion != nil && len(*shootRegion) > 0 {
		shoot.Spec.Region = *shootRegion
	}

	if externalDomain != nil && len(*externalDomain) > 0 {
		shoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: externalDomain}
		clearDNS = false
	}

	if projectNamespace != nil && len(*projectNamespace) > 0 {
		shoot.Namespace = *projectNamespace
	}

	if prefix != nil && len(*prefix) != 0 {
		integrationTestName, err := generateRandomShootName(*prefix, 8)
		if err != nil {
			return "", nil, err
		}
		shoot.Name = integrationTestName
	}

	if cloudProfile != nil && len(*cloudProfile) > 0 {
		shoot.Spec.CloudProfileName = *cloudProfile
	}

	if secretbinding != nil && len(*secretbinding) > 0 {
		shoot.Spec.SecretBindingName = *secretbinding
	}

	if providerType != nil && len(*providerType) > 0 {
		shoot.Spec.Provider.Type = *providerType
	}

	if k8sVersion != nil && len(*k8sVersion) > 0 {
		shoot.Spec.Kubernetes.Version = *k8sVersion
	}

	if clearDNS {
		shoot.Spec.DNS = &gardencorev1beta1.DNS{}
	}

	if clearExtensions {
		shoot.Spec.Extensions = nil
	}

	if shoot.Annotations == nil {
		shoot.Annotations = map[string]string{}
	}
	shoot.Annotations[v1beta1constants.AnnotationShootIgnoreAlerts] = "true"

	return shoot.Name, shoot, nil
}

// SetProviderConfigsFromFilepath parses the infrastructure, controlPlane and networking provider-configs and sets them on the shoot
func SetProviderConfigsFromFilepath(shoot *gardencorev1beta1.Shoot, infrastructureConfigPath, controlPlaneConfigPath, networkingConfigPath, workersConfigPath *string) error {
	// clear provider configs first
	shoot.Spec.Provider.InfrastructureConfig = nil
	shoot.Spec.Provider.ControlPlaneConfig = nil
	shoot.Spec.Networking.ProviderConfig = nil

	if infrastructureConfigPath != nil && len(*infrastructureConfigPath) != 0 {
		infrastructureProviderConfig, err := ParseFileAsProviderConfig(*infrastructureConfigPath)
		if err != nil {
			return err
		}
		shoot.Spec.Provider.InfrastructureConfig = infrastructureProviderConfig
	}

	if len(*controlPlaneConfigPath) != 0 {
		controlPlaneProviderConfig, err := ParseFileAsProviderConfig(*controlPlaneConfigPath)
		if err != nil {
			return err
		}
		shoot.Spec.Provider.ControlPlaneConfig = controlPlaneProviderConfig
	}

	if len(*networkingConfigPath) != 0 {
		networkingProviderConfig, err := ParseFileAsProviderConfig(*networkingConfigPath)
		if err != nil {
			return err
		}
		shoot.Spec.Networking.ProviderConfig = networkingProviderConfig
	}

	if len(*workersConfigPath) != 0 {
		workers, err := ParseFileAsWorkers(*workersConfigPath)
		if err != nil {
			return err
		}
		shoot.Spec.Provider.Workers = workers
	}

	return nil
}

func generateRandomShootName(prefix string, length int) (string, error) {
	randomString, err := utils.GenerateRandomString(length)
	if err != nil {
		return "", err
	}

	if len(prefix) > 0 {
		return prefix + strings.ToLower(randomString), nil
	}

	return IntegrationTestPrefix + strings.ToLower(randomString), nil
}
