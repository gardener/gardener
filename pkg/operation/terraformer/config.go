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

package terraformer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// MainKey is the key of the main.tf file inside the configuration ConfigMap.
	MainKey = "main.tf"
	// VariablesKey is the key of the variables.tf file inside the configuration ConfigMap.
	VariablesKey = "variables.tf"
	// TFVarsKey is the key of the terraform.tfvars file inside the variables Secret.
	TFVarsKey = "terraform.tfvars"
	// StateKey is the key of the terraform.tfstate file inside the state ConfigMap.
	StateKey = "terraform.tfstate"
)

// SetVariablesEnvironment sets the provided <tfvarsEnvironment> on the Terraformer object.
func (t *Terraformer) SetVariablesEnvironment(tfvarsEnvironment map[string]string) *Terraformer {
	t.variablesEnvironment = tfvarsEnvironment
	return t
}

// InitializerConfig is the configuration about the location and naming of the resources the
// Terraformer expects.
type InitializerConfig struct {
	// Namespace is the namespace where all the resources required for the Terraformer shall be
	// deployed.
	Namespace string
	// ConfigurationName is the desired name of the configuration ConfigMap.
	ConfigurationName string
	// VariablesName is the desired name of the variables Secret.
	VariablesName string
	// StateName is the desired name of the state ConfigMap.
	StateName string
	// InitializeState specifies whether an empty state should be initialized or not.
	InitializeState bool
}

// Initializer is a function that is called from the Terraformer to initialize its configuration.
type Initializer func(config *InitializerConfig) error

func (t *Terraformer) initializerConfig() *InitializerConfig {
	return &InitializerConfig{
		Namespace:         t.namespace,
		ConfigurationName: t.configName,
		VariablesName:     t.variablesName,
		StateName:         t.stateName,
		InitializeState:   t.isStateEmpty(),
	}
}

// InitializeWith initializes the Terraformer with the given Initializer. It is expected from the
// Initializer to correctly create all the resources as specified in the given InitializerConfig.
// A default implementation can be found in DefaultInitializer.
func (t *Terraformer) InitializeWith(initializer Initializer) *Terraformer {
	if err := initializer(t.initializerConfig()); err != nil {
		t.logger.Errorf("Could not create the Terraform ConfigMaps/Secrets: %s", err.Error())
		return t
	}
	t.configurationDefined = true
	return t
}

func createOrUpdateConfigMap(ctx context.Context, c client.Client, namespace, name string, values map[string]string) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
	return configMap, kutil.CreateOrUpdate(ctx, c, configMap, func() error {
		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}
		for key, value := range values {
			configMap.Data[key] = value
		}
		return nil
	})
}

// CreateOrUpdateConfigurationConfigMap creates or updates the Terraform configuration ConfigMap
// with the given main and variables content.
func CreateOrUpdateConfigurationConfigMap(ctx context.Context, c client.Client, namespace, name, main, variables string) (*corev1.ConfigMap, error) {
	return createOrUpdateConfigMap(ctx, c, namespace, name, map[string]string{
		MainKey:      main,
		VariablesKey: variables,
	})
}

// CreateOrUpdateStateConfigMap creates or updates the Terraformer state ConfigMap with the given state.
func CreateOrUpdateStateConfigMap(ctx context.Context, c client.Client, namespace, name, state string) (*corev1.ConfigMap, error) {
	return createOrUpdateConfigMap(ctx, c, namespace, name, map[string]string{
		StateKey: state,
	})
}

// CreateOrUpdateTFVarsSecret creates or updates the Terraformer variables Secret with the given tfvars.
func CreateOrUpdateTFVarsSecret(ctx context.Context, c client.Client, namespace, name string, tfvars []byte) (*corev1.Secret, error) {
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
	return secret, kutil.CreateOrUpdate(ctx, c, secret, func() error {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		secret.Data[TFVarsKey] = tfvars
		return nil
	})
}

// DefaultInitializer is an Initializer that initializes the configuration, variables and state resources
// based on the given main, variables and tfvars content and on the given InitializerConfig.
func DefaultInitializer(c client.Client, main, variables string, tfvars []byte) Initializer {
	return func(config *InitializerConfig) error {
		ctx := context.TODO()
		if _, err := CreateOrUpdateConfigurationConfigMap(ctx, c, config.Namespace, config.ConfigurationName, main, variables); err != nil {
			return err
		}

		if _, err := CreateOrUpdateTFVarsSecret(ctx, c, config.Namespace, config.VariablesName, tfvars); err != nil {
			return err
		}

		if config.InitializeState {
			if _, err := CreateOrUpdateStateConfigMap(ctx, c, config.Namespace, config.StateName, ""); err != nil {
				return err
			}
		}
		return nil
	}
}

// prepare checks whether all required ConfigMaps and Secrets exist. It returns the number of
// existing ConfigMaps/Secrets, or the error in case something unexpected happens.
func (t *Terraformer) prepare(ctx context.Context) (int, error) {
	numberOfExistingResources, err := t.verifyConfigExists(ctx)
	if err != nil {
		return -1, err
	}

	if t.variablesEnvironment == nil {
		return -1, errors.New("no Terraform variables environment provided")
	}

	// Clean up possible existing job/pod artifacts from previous runs
	if err := t.ensureCleanedUp(); err != nil {
		return -1, err
	}

	return numberOfExistingResources, nil
}

func (t *Terraformer) verifyConfigExists(ctx context.Context) (int, error) {
	numberOfExistingResources := 0

	if err := t.client.Get(ctx, kutil.Key(t.namespace, t.stateName), &corev1.ConfigMap{}); err == nil {
		numberOfExistingResources++
	} else if err != nil && !apierrors.IsNotFound(err) {
		return -1, err
	}

	if err := t.client.Get(ctx, kutil.Key(t.namespace, t.variablesName), &corev1.Secret{}); err == nil {
		numberOfExistingResources++
	} else if err != nil && !apierrors.IsNotFound(err) {
		return -1, err
	}

	if err := t.client.Get(ctx, kutil.Key(t.namespace, t.configName), &corev1.ConfigMap{}); err == nil {
		numberOfExistingResources++
	} else if err != nil && !apierrors.IsNotFound(err) {
		return -1, err
	}

	return numberOfExistingResources, nil
}

// ConfigExists returns true if all three Terraform configuration secrets/configmaps exist, and false otherwise.
func (t *Terraformer) ConfigExists() (bool, error) {
	numberOfExistingResources, err := t.verifyConfigExists(context.TODO())
	return numberOfExistingResources == numberOfConfigResources, err
}

// cleanupConfiguration deletes the two ConfigMaps which store the Terraform configuration and state. It also deletes
// the Secret which stores the Terraform variables.
func (t *Terraformer) cleanupConfiguration(ctx context.Context) error {
	t.logger.Debugf("Deleting Terraform variables Secret '%s'", t.variablesName)
	if err := t.client.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.variablesName}}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	t.logger.Debugf("Deleting Terraform configuration ConfigMap '%s'", t.configName)
	if err := t.client.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.configName}}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	t.logger.Debugf("Deleting Terraform state ConfigMap '%s'", t.stateName)
	if err := t.client.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.stateName}}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

// ensureCleanedUp deletes the job, pods, and waits until everything has been cleaned up.
func (t *Terraformer) ensureCleanedUp() error {
	ctx := context.TODO()
	jobPodList, err := t.listJobPods(ctx)
	if err != nil {
		return err
	}
	if err := t.cleanupJob(ctx, jobPodList); err != nil {
		return err
	}
	return t.waitForCleanEnvironment(ctx)
}

// GenerateVariablesEnvironment takes a <secret> and a <keyValueMap> and builds an environment which
// can be injected into the Terraformer job/pod manifest. The keys of the <keyValueMap> will be prefixed with
// 'TF_VAR_' and the value will be used to extract the respective data from the <secret>.
func GenerateVariablesEnvironment(secret *corev1.Secret, keyValueMap map[string]string) map[string]string {
	out := make(map[string]string, len(keyValueMap))
	for key, value := range keyValueMap {
		out[fmt.Sprintf("TF_VAR_%s", key)] = strings.TrimSpace(string(secret.Data[value]))
	}
	return out
}
