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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
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

// SetEnvVars sets the provided environment variables for the Terraformer Pod.
func (t *terraformer) SetEnvVars(envVars ...corev1.EnvVar) Terraformer {
	t.envVars = append(t.envVars, envVars...)
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

// SetOwnerRef configures the resource that will be used as owner of the secrets and configmaps
func (t *terraformer) SetOwnerRef(owner *metav1.OwnerReference) Terraformer {
	t.ownerRef = owner
	return t
}

// UseV2 configures if it should use flags compatible with terraformer@v2.
func (t *terraformer) UseV2(v2 bool) Terraformer {
	t.useV2 = v2
	return t
}

// SetLogLevel sets the log level of the Terraformer pod. It only takes effect when UseV2 is set to true.
func (t *terraformer) SetLogLevel(level string) Terraformer {
	t.logLevel = level
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

func (t *terraformer) initializerConfig(ctx context.Context) *InitializerConfig {
	return &InitializerConfig{
		Namespace:         t.namespace,
		ConfigurationName: t.configName,
		VariablesName:     t.variablesName,
		StateName:         t.stateName,
		InitializeState:   t.IsStateEmpty(ctx),
	}
}

// InitializeWith initializes the Terraformer with the given Initializer. It is expected from the
// Initializer to correctly create all the resources as specified in the given InitializerConfig.
// A default implementation can be found in DefaultInitializer.
func (t *terraformer) InitializeWith(ctx context.Context, initializer Initializer) Terraformer {
	if err := initializer.Initialize(ctx, t.initializerConfig(ctx), t.ownerRef); err != nil {
		t.logger.Error(err, "Could not create Terraformer ConfigMaps/Secrets")
		return t
	}
	t.configurationDefined = true
	return t
}

func createOrUpdateConfigMap(ctx context.Context, c client.Client, namespace, name string, values map[string]string, ownerRef *metav1.OwnerReference) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, c, configMap, func() error {
		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}
		for key, value := range values {
			configMap.Data[key] = value
		}
		if ownerRef != nil {
			configMap.SetOwnerReferences(kutil.MergeOwnerReferences(configMap.OwnerReferences, *ownerRef))
		}
		return nil
	})
	return configMap, err
}

// CreateOrUpdateConfigurationConfigMap creates or updates the Terraform configuration ConfigMap
// with the given main and variables content.
func CreateOrUpdateConfigurationConfigMap(ctx context.Context, c client.Client, namespace, name, main, variables string, ownerRef *metav1.OwnerReference) (*corev1.ConfigMap, error) {
	return createOrUpdateConfigMap(
		ctx,
		c,
		namespace,
		name,
		map[string]string{
			MainKey:      main,
			VariablesKey: variables,
		},
		ownerRef,
	)
}

// CreateOrUpdateTFVarsSecret creates or updates the Terraformer variables Secret with the given tfvars.
func CreateOrUpdateTFVarsSecret(ctx context.Context, c client.Client, namespace, name string, tfvars []byte, ownerRef *metav1.OwnerReference) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, c, secret, func() error {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		secret.Data[TFVarsKey] = tfvars
		if ownerRef != nil {
			secret.SetOwnerReferences(kutil.MergeOwnerReferences(secret.OwnerReferences, *ownerRef))
		}
		return nil
	})
	return secret, err
}

// initializerFunc implements Initializer.
type initializerFunc func(ctx context.Context, config *InitializerConfig, ownerRef *metav1.OwnerReference) error

// Initialize implements Initializer.
func (f initializerFunc) Initialize(ctx context.Context, config *InitializerConfig, ownerRef *metav1.OwnerReference) error {
	return f(ctx, config, ownerRef)
}

// DefaultInitializer is an Initializer that initializes the configuration, variables and state resources
// based on the given main, variables and tfvars content and on the given InitializerConfig.
func DefaultInitializer(c client.Client, main, variables string, tfvars []byte, stateInitializer StateConfigMapInitializer) Initializer {
	return initializerFunc(func(ctx context.Context, config *InitializerConfig, ownerRef *metav1.OwnerReference) error {
		if _, err := CreateOrUpdateConfigurationConfigMap(ctx, c, config.Namespace, config.ConfigurationName, main, variables, ownerRef); err != nil {
			return err
		}

		if _, err := CreateOrUpdateTFVarsSecret(ctx, c, config.Namespace, config.VariablesName, tfvars, ownerRef); err != nil {
			return err
		}

		if config.InitializeState {
			if err := stateInitializer.Initialize(ctx, c, config.Namespace, config.StateName, ownerRef); err != nil {
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

	// Clean up possible existing pod artifacts from previous runs
	if err := t.EnsureCleanedUp(ctx); err != nil {
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
func (t *terraformer) ConfigExists(ctx context.Context) (bool, error) {
	numberOfExistingResources, err := t.NumberOfResources(ctx)
	return numberOfExistingResources == numberOfConfigResources, err
}

// CleanupConfiguration deletes the two ConfigMaps which store the Terraform configuration and state. It also deletes
// the Secret which stores the Terraform variables.
func (t *terraformer) CleanupConfiguration(ctx context.Context) error {
	t.logger.Info("Cleaning up all terraformer configuration")

	t.logger.V(1).Info("Deleting Terraform variables Secret", "name", t.variablesName)
	if err := t.client.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.variablesName}}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	t.logger.V(1).Info("Deleting Terraform configuration ConfigMap", "name", t.configName)
	if err := t.client.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.configName}}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	t.logger.V(1).Info("Deleting Terraform state ConfigMap", "name", t.stateName)
	if err := t.client.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.stateName}}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

// EnsureCleanedUp deletes the Terraformer pods, and waits until everything has been cleaned up.
func (t *terraformer) EnsureCleanedUp(ctx context.Context) error {
	t.logger.Info("Ensuring all terraformer Pods have been deleted")

	podList, err := t.listTerraformerPods(ctx)
	if err != nil {
		return err
	}
	if err := t.deleteTerraformerPods(ctx, podList); err != nil {
		return err
	}

	return t.WaitForCleanEnvironment(ctx)
}
