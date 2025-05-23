// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terraformer

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
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

// SetDeadlinePodCreation configures the deadline while waiting for the creation of the Terraformer apply/destroy pod.
func (t *terraformer) SetDeadlinePodCreation(d time.Duration) Terraformer {
	t.deadlinePodCreation = d
	return t
}

// SetOwnerRef configures the resource that will be used as owner of the secrets and configmaps
func (t *terraformer) SetOwnerRef(owner *metav1.OwnerReference) Terraformer {
	t.ownerRef = owner
	return t
}

// UseProjectedTokenMount configures the useProjectedTokenMount field (defaults to true).
// Should only be disabled for testing in a cluster without gardener-resource-manager.
func (t *terraformer) UseProjectedTokenMount(useProjectedTokenMount bool) Terraformer {
	t.useProjectedTokenMount = useProjectedTokenMount
	return t
}

// SetLogLevel sets the log level of the Terraformer pod.
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
	config := t.initializerConfig(ctx)

	if err := initializer.Initialize(ctx, config, t.ownerRef); err != nil {
		t.logger.Error(err, "Could not create Terraformer ConfigMaps/Secrets")
		return t
	}

	t.configurationInitialized = true
	t.stateInitialized = config.InitializeState

	return t
}

func createOrUpdateConfigMap(ctx context.Context, c client.Client, namespace, name string, values map[string]string, ownerRef *metav1.OwnerReference) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	_, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, c, configMap, func() error {
		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}
		for key, value := range values {
			configMap.Data[key] = value
		}
		if ownerRef != nil {
			configMap.SetOwnerReferences(kubernetesutils.MergeOwnerReferences(configMap.OwnerReferences, *ownerRef))
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

	_, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, c, secret, func() error {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		secret.Data[TFVarsKey] = tfvars
		if ownerRef != nil {
			secret.SetOwnerReferences(kubernetesutils.MergeOwnerReferences(secret.OwnerReferences, *ownerRef))
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

// NumberOfResources returns the number of existing Terraform resources or an error in case something went wrong.
func (t *terraformer) NumberOfResources(ctx context.Context) (int, error) {
	numberOfExistingResources := 0

	if err := t.client.Get(ctx, client.ObjectKey{Namespace: t.namespace, Name: t.stateName}, &corev1.ConfigMap{}); err == nil {
		numberOfExistingResources++
	} else if !apierrors.IsNotFound(err) {
		return -1, err
	}

	if err := t.client.Get(ctx, client.ObjectKey{Namespace: t.namespace, Name: t.variablesName}, &corev1.Secret{}); err == nil {
		numberOfExistingResources++
	} else if !apierrors.IsNotFound(err) {
		return -1, err
	}

	if err := t.client.Get(ctx, client.ObjectKey{Namespace: t.namespace, Name: t.configName}, &corev1.ConfigMap{}); err == nil {
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

	t.logger.V(1).Info("Deleting Terraform state ConfigMap", "name", t.stateName)
	if err := t.client.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.stateName}}); client.IgnoreNotFound(err) != nil {
		return err
	}

	t.logger.V(1).Info("Deleting Terraform variables Secret", "name", t.variablesName)
	if err := t.client.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.variablesName}}); client.IgnoreNotFound(err) != nil {
		return err
	}

	t.logger.V(1).Info("Deleting Terraform configuration ConfigMap", "name", t.configName)
	if err := t.client.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.configName}}); client.IgnoreNotFound(err) != nil {
		return err
	}

	return nil
}

// RemoveTerraformerFinalizerFromConfig deletes the terraformer finalizer from the two ConfigMaps and the Secret which store the Terraform configuration and state.
func (t *terraformer) RemoveTerraformerFinalizerFromConfig(ctx context.Context) error {
	for _, obj := range []client.Object{
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.variablesName}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.stateName}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.configName}},
	} {
		if err := t.client.Get(ctx, client.ObjectKey{Namespace: t.namespace, Name: obj.GetName()}, obj); client.IgnoreNotFound(err) != nil {
			return err
		}

		if controllerutil.ContainsFinalizer(obj, TerraformerFinalizer) {
			t.logger.Info("Removing finalizer", "obj", client.ObjectKeyFromObject(obj))
			if err := controllerutils.RemoveFinalizers(ctx, t.client, obj, TerraformerFinalizer); err != nil {
				return fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}
	}
	return nil
}

// EnsureCleanedUp deletes the Terraformer pods, and waits until everything has been cleaned up.
func (t *terraformer) EnsureCleanedUp(ctx context.Context) error {
	t.logger.Info("Ensuring all Terraformer pods have been deleted")

	podList, err := t.listPods(ctx)
	if err != nil {
		return err
	}

	if err := t.deleteTerraformerPods(ctx, podList); err != nil {
		return err
	}

	return t.WaitForCleanEnvironment(ctx)
}
