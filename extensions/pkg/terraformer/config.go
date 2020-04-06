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
	"time"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
func (t *terraformer) SetVariablesEnvironment(tfvarsEnvironment map[string]string) Terraformer {
	t.variablesEnvironment = tfvarsEnvironment
	return t
}

// SetTerminationGracePeriodSeconds configures the .spec.terminationGracePeriodSeconds for the Terraformer pod.
func (t *terraformer) SetTerminationGracePeriodSeconds(terminationGracePeriodSeconds int64) Terraformer {
	t.terminationGracePeriodSeconds = terminationGracePeriodSeconds
	return t
}

// SetDeadlineCleaning configures the deadline while waiting for a clean environment.
func (t *terraformer) SetDeadlineCleaning(d time.Duration) Terraformer {
	t.deadlineCleaning = d
	return t
}

// SetDeadlinePod configures the deadline while waiting for the Terraformer apply/destroy pod.
func (t *terraformer) SetDeadlinePod(d time.Duration) Terraformer {
	t.deadlinePod = d
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

func (t *terraformer) initializerConfig() *InitializerConfig {
	return &InitializerConfig{
		Namespace:         t.namespace,
		ConfigurationName: t.configName,
		VariablesName:     t.variablesName,
		StateName:         t.stateName,
		InitializeState:   t.IsStateEmpty(),
	}
}

// InitializeWith initializes the Terraformer with the given Initializer. It is expected from the
// Initializer to correctly create all the resources as specified in the given InitializerConfig.
// A default implementation can be found in DefaultInitializer.
func (t *terraformer) InitializeWith(initializer Initializer) Terraformer {
	if err := initializer.Initialize(t.initializerConfig()); err != nil {
		t.logger.Errorf("Could not create the Terraform ConfigMaps/Secrets: %s", err.Error())
		return t
	}
	t.configurationDefined = true
	return t
}

func createOrUpdateConfigMap(ctx context.Context, c client.Client, namespace, name string, values map[string]string) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
	_, err := controllerutil.CreateOrUpdate(ctx, c, configMap, func() error {
		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}
		for key, value := range values {
			configMap.Data[key] = value
		}
		return nil
	})
	return configMap, err
}

// CreateOrUpdateConfigurationConfigMap creates or updates the Terraform configuration ConfigMap
// with the given main and variables content.
func CreateOrUpdateConfigurationConfigMap(ctx context.Context, c client.Client, namespace, name, main, variables string) (*corev1.ConfigMap, error) {
	return createOrUpdateConfigMap(ctx, c, namespace, name, map[string]string{
		MainKey:      main,
		VariablesKey: variables,
	})
}

// CreateStateConfigMap creates the Terraformer state ConfigMap with the given state.
func CreateStateConfigMap(ctx context.Context, c client.Client, namespace, name, state string) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Data: map[string]string{
			StateKey: state,
		},
	}

	return c.Create(ctx, configMap)
}

// CreateOrUpdateTFVarsSecret creates or updates the Terraformer variables Secret with the given tfvars.
func CreateOrUpdateTFVarsSecret(ctx context.Context, c client.Client, namespace, name string, tfvars []byte) (*corev1.Secret, error) {
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
	_, err := controllerutil.CreateOrUpdate(ctx, c, secret, func() error {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		secret.Data[TFVarsKey] = tfvars
		return nil
	})
	return secret, err
}

// initializerFunc implements Initializer.
type initializerFunc func(config *InitializerConfig) error

// Initialize implements Initializer.
func (f initializerFunc) Initialize(config *InitializerConfig) error {
	return f(config)
}

// DefaultInitializer is an Initializer that initializes the configuration, variables and state resources
// based on the given main, variables, tfvars and state content and on the given InitializerConfig.
func DefaultInitializer(c client.Client, main, variables string, tfvars []byte, state string) Initializer {
	return initializerFunc(func(config *InitializerConfig) error {
		ctx := context.TODO()
		if _, err := CreateOrUpdateConfigurationConfigMap(ctx, c, config.Namespace, config.ConfigurationName, main, variables); err != nil {
			return err
		}

		if _, err := CreateOrUpdateTFVarsSecret(ctx, c, config.Namespace, config.VariablesName, tfvars); err != nil {
			return err
		}

		if config.InitializeState {
			if err := CreateStateConfigMap(ctx, c, config.Namespace, config.StateName, state); err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
		}
		return nil
	})
}

// prepare checks whether all required ConfigMaps and Secrets exist. It returns the number of
// existing ConfigMaps/Secrets, or the error in case something unexpected happens.
func (t *terraformer) prepare(ctx context.Context) (int, error) {
	numberOfExistingResources, err := t.NumberOfResources(ctx)
	if err != nil {
		return -1, err
	}

	if t.variablesEnvironment == nil {
		return -1, errors.New("no Terraform variables environment provided")
	}

	// Clean up possible existing pod artifacts from previous runs
	if err := t.ensureCleanedUp(ctx); err != nil {
		return -1, err
	}

	return numberOfExistingResources, nil
}

// NumberOfResources returns the number of existing Terraform resources or an error in case something went wrong.
func (t *terraformer) NumberOfResources(ctx context.Context) (int, error) {
	numberOfExistingResources := 0

	if err := t.client.Get(ctx, kutil.Key(t.namespace, t.stateName), &corev1.ConfigMap{}); err == nil {
		numberOfExistingResources++
	} else if !apierrors.IsNotFound(err) {
		return -1, err
	}

	if err := t.client.Get(ctx, kutil.Key(t.namespace, t.variablesName), &corev1.Secret{}); err == nil {
		numberOfExistingResources++
	} else if !apierrors.IsNotFound(err) {
		return -1, err
	}

	if err := t.client.Get(ctx, kutil.Key(t.namespace, t.configName), &corev1.ConfigMap{}); err == nil {
		numberOfExistingResources++
	} else if !apierrors.IsNotFound(err) {
		return -1, err
	}

	return numberOfExistingResources, nil
}

// ConfigExists returns true if all three Terraform configuration secrets/configmaps exist, and false otherwise.
func (t *terraformer) ConfigExists() (bool, error) {
	numberOfExistingResources, err := t.NumberOfResources(context.TODO())
	return numberOfExistingResources == numberOfConfigResources, err
}

// CleanupConfiguration deletes the two ConfigMaps which store the Terraform configuration and state. It also deletes
// the Secret which stores the Terraform variables.
func (t *terraformer) CleanupConfiguration(ctx context.Context) error {
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

// ensureCleanedUp deletes the Terraformer pods, and waits until everything has been cleaned up.
func (t *terraformer) ensureCleanedUp(ctx context.Context) error {
	podList, err := t.listTerraformerPods(ctx)
	if err != nil {
		return err
	}
	if err := t.deleteTerraformerPods(ctx, podList); err != nil {
		return err
	}

	return t.waitForCleanEnvironment(ctx)
}

// GenerateVariablesEnvironment takes a <secret> and a <keyValueMap> and builds an environment which
// can be injected into the Terraformer pod manifest. The keys of the <keyValueMap> will be prefixed with
// 'TF_VAR_' and the value will be used to extract the respective data from the <secret>.
func GenerateVariablesEnvironment(secret *corev1.Secret, keyValueMap map[string]string) map[string]string {
	out := make(map[string]string, len(keyValueMap))
	for key, value := range keyValueMap {
		out[fmt.Sprintf("TF_VAR_%s", key)] = strings.TrimSpace(string(secret.Data[value]))
	}
	return out
}
