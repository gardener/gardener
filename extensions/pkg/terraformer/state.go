// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terraformer

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	// TerraformerFinalizer is the finalizer key set by the terraformer on the configmaps and secrets
	TerraformerFinalizer = "gardener.cloud/terraformer"
)

type terraformStateV3 struct {
	Modules []struct {
		Outputs map[string]outputState `json:"outputs"`
	} `json:"modules"`
}

type outputState struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type terraformStateV4 struct {
	Outputs map[string]outputState `json:"outputs"`
}

// GetState returns the Terraform state as byte slice.
func (t *terraformer) GetState(ctx context.Context) ([]byte, error) {
	configMap := &corev1.ConfigMap{}
	if err := t.client.Get(ctx, client.ObjectKey{Namespace: t.namespace, Name: t.stateName}, configMap); err != nil {
		return nil, err
	}

	return []byte(configMap.Data[StateKey]), nil
}

// GetStateOutputVariables returns the given <variable> from the given Terraform <stateData>.
// In case the variable was not found, an error is returned.
func (t *terraformer) GetStateOutputVariables(ctx context.Context, variables ...string) (map[string]string, error) {
	var (
		output = make(map[string]string)

		wantedVariables = sets.New(variables...)
		foundVariables  = sets.New[string]()
	)

	stateConfigMap, err := t.GetState(ctx)
	if err != nil {
		return nil, err
	}

	if len(stateConfigMap) == 0 {
		return nil, &variablesNotFoundError{sets.List(wantedVariables)}
	}

	outputVariables, err := getOutputVariables(stateConfigMap)
	if err != nil {
		return nil, err
	}

	for _, variable := range variables {
		if outputVariable, ok := outputVariables[variable]; ok {
			output[variable] = fmt.Sprint(outputVariable.Value)
			foundVariables.Insert(variable)
		}
	}

	if wantedVariables.Len() != foundVariables.Len() {
		return nil, &variablesNotFoundError{sets.List(wantedVariables.Difference(foundVariables))}
	}

	return output, nil
}

// IsStateEmpty returns true if the Terraform state is empty and the terraformer finalizer
// is not present on any of the used configmaps and secrets. Otherwise, it returns false.
func (t *terraformer) IsStateEmpty(ctx context.Context) bool {
	for _, obj := range []client.Object{
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.configName}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.stateName}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.variablesName}},
	} {
		resourceName := obj.GetName()
		if err := t.client.Get(ctx, client.ObjectKey{Namespace: t.namespace, Name: resourceName}, obj); client.IgnoreNotFound(err) != nil {
			t.logger.Error(err, "Failed to get resource", "name", resourceName)
			return false
		}

		if controllerutil.ContainsFinalizer(obj, TerraformerFinalizer) {
			return false
		}
	}

	state, err := t.GetState(ctx)
	if err != nil {
		return apierrors.IsNotFound(err)
	}
	return len(state) == 0
}

type variablesNotFoundError struct {
	variables []string
}

// Error prints the error message of the variablesNotFound error.
func (e *variablesNotFoundError) Error() string {
	return fmt.Sprintf("could not find all requested variables: %+v", e.variables)
}

// IsVariablesNotFoundError returns true if the error indicates that not all variables have been found.
func IsVariablesNotFoundError(err error) bool {
	switch err.(type) {
	case *variablesNotFoundError:
		return true
	}
	return false
}

func getOutputVariables(stateConfigMap []byte) (map[string]outputState, error) {
	version, err := sniffJSONStateVersion(stateConfigMap)
	if err != nil {
		return nil, err
	}

	var outputVariables map[string]outputState
	switch version {
	case 2, 3:
		var state terraformStateV3
		if err := json.Unmarshal(stateConfigMap, &state); err != nil {
			return nil, err
		}

		outputVariables = state.Modules[0].Outputs
	case 4:
		var state terraformStateV4
		if err := json.Unmarshal(stateConfigMap, &state); err != nil {
			return nil, err
		}

		outputVariables = state.Outputs
	default:
		return nil, fmt.Errorf("the state file uses format version %d, which is not supported by Terraformer", version)
	}

	return outputVariables, nil
}

func sniffJSONStateVersion(stateConfigMap []byte) (uint64, error) {
	type VersionSniff struct {
		Version *uint64 `json:"version"`
	}
	var sniff VersionSniff
	if err := json.Unmarshal(stateConfigMap, &sniff); err != nil {
		return 0, fmt.Errorf("the state file could not be parsed as JSON: %w", err)
	}

	if sniff.Version == nil {
		return 0, fmt.Errorf("the state file does not have a \"version\" attribute, which is required to identify the format version")
	}

	return *sniff.Version, nil
}

// Initialize implements StateConfigMapInitializer
func (f StateConfigMapInitializerFunc) Initialize(ctx context.Context, c client.Client, namespace, name string, ownerRef *metav1.OwnerReference) error {
	return f(ctx, c, namespace, name, ownerRef)
}

// CreateState create terraform state config map and use empty state.
// It does not create or update state ConfigMap if already exists,
func CreateState(ctx context.Context, c client.Client, namespace, name string, ownerRef *metav1.OwnerReference) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Data: map[string]string{
			StateKey: "",
		},
	}

	if ownerRef != nil {
		configMap.SetOwnerReferences(kubernetesutils.MergeOwnerReferences(configMap.OwnerReferences, *ownerRef))
	}

	return client.IgnoreAlreadyExists(c.Create(ctx, configMap))
}

// Initialize implements StateConfigMapInitializer
func (cus CreateOrUpdateState) Initialize(ctx context.Context, c client.Client, namespace, name string, ownerRef *metav1.OwnerReference) error {
	if cus.State == nil {
		return fmt.Errorf("missing state when creating or updating terraform state ConfigMap %s/%s", namespace, name)
	}
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}

	_, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, c, configMap, func() error {
		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}
		configMap.Data[StateKey] = *cus.State

		if ownerRef != nil {
			configMap.SetOwnerReferences(kubernetesutils.MergeOwnerReferences(configMap.OwnerReferences, *ownerRef))
		}
		return nil
	})

	return err
}
