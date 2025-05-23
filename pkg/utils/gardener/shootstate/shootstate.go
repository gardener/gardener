// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootstate

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiextensions "github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	unstructuredutils "github.com/gardener/gardener/pkg/utils/kubernetes/unstructured"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// Deploy deploys the ShootState resource with the effective state for the given shoot into the garden
// cluster.
func Deploy(ctx context.Context, clock clock.Clock, gardenClient, seedClient client.Client, shoot *gardencorev1beta1.Shoot, overwriteSpec bool) error {
	shootState := &gardencorev1beta1.ShootState{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shoot.Name,
			Namespace: shoot.Namespace,
		},
	}

	spec, err := computeSpec(ctx, seedClient, shoot.Status.TechnicalID)
	if err != nil {
		return fmt.Errorf("failed computing spec of ShootState for shoot %s: %w", client.ObjectKeyFromObject(shoot), err)
	}

	_, err = controllerutils.GetAndCreateOrStrategicMergePatch(ctx, gardenClient, shootState, func() error {
		metav1.SetMetaDataAnnotation(&shootState.ObjectMeta, v1beta1constants.GardenerTimestamp, clock.Now().UTC().Format(time.RFC3339))

		if overwriteSpec {
			shootState.Spec = *spec
			return nil
		}

		gardenerData := v1beta1helper.GardenerResourceDataList(shootState.Spec.Gardener)
		for _, data := range spec.Gardener {
			gardenerData.Upsert(data.DeepCopy())
		}
		shootState.Spec.Gardener = gardenerData

		extensionsData := v1beta1helper.ExtensionResourceStateList(shootState.Spec.Extensions)
		for _, data := range spec.Extensions {
			extensionsData.Upsert(data.DeepCopy())
		}
		shootState.Spec.Extensions = extensionsData

		resourcesData := v1beta1helper.ResourceDataList(shootState.Spec.Resources)
		for _, data := range spec.Resources {
			resourcesData.Upsert(data.DeepCopy())
		}
		shootState.Spec.Resources = resourcesData

		return nil
	})
	return err
}

// Delete deletes the ShootState resource for the given shoot from the garden cluster.
func Delete(ctx context.Context, gardenClient client.Client, shoot *gardencorev1beta1.Shoot) error {
	shootState := &gardencorev1beta1.ShootState{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shoot.Name,
			Namespace: shoot.Namespace,
		},
	}

	if err := gardenerutils.ConfirmDeletion(ctx, gardenClient, shootState); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return client.IgnoreNotFound(gardenClient.Delete(ctx, shootState))
}

func computeSpec(ctx context.Context, seedClient client.Client, seedNamespace string) (*gardencorev1beta1.ShootStateSpec, error) {
	gardener, err := computeGardenerData(ctx, seedClient, seedNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed computing Gardener data: %w", err)
	}

	extensions, resources, err := computeExtensionsDataAndResources(ctx, seedClient, seedNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed computing extensions data and resources: %w", err)
	}

	return &gardencorev1beta1.ShootStateSpec{
		Gardener:   gardener,
		Extensions: extensions,
		Resources:  resources,
	}, nil
}

func computeGardenerData(
	ctx context.Context,
	seedClient client.Client,
	seedNamespace string,
) (
	[]gardencorev1beta1.GardenerResourceData,
	error,
) {
	secretsToPersist, err := computeSecretsToPersist(ctx, seedClient, seedNamespace)
	if err != nil {
		return nil, err
	}

	machineState, err := computeMachineState(ctx, seedClient, seedNamespace)
	if err != nil {
		return nil, err
	}

	machineStateJSON, err := json.Marshal(machineState)
	if err != nil {
		return nil, fmt.Errorf("failed marshalling machine state to JSON: %w", err)
	}

	machineStateJSONCompressed, err := compressMachineState(machineStateJSON)
	if err != nil {
		return nil, fmt.Errorf("failed compressing machine state data: %w", err)
	}

	if machineStateJSONCompressed != nil {
		secretsToPersist = append(secretsToPersist, gardencorev1beta1.GardenerResourceData{
			Name: v1beta1constants.DataTypeMachineState,
			Type: v1beta1constants.DataTypeMachineState,
			Data: runtime.RawExtension{Raw: machineStateJSONCompressed},
		})
	}

	return secretsToPersist, nil
}

func computeSecretsToPersist(
	ctx context.Context,
	seedClient client.Client,
	seedNamespace string,
) (
	[]gardencorev1beta1.GardenerResourceData,
	error,
) {
	secretList := &corev1.SecretList{}
	if err := seedClient.List(ctx, secretList, client.InNamespace(seedNamespace), client.MatchingLabels{
		secretsmanager.LabelKeyPersist: secretsmanager.LabelValueTrue,
	}); err != nil {
		return nil, fmt.Errorf("failed listing all secrets that must be persisted: %w", err)
	}

	dataList := make([]gardencorev1beta1.GardenerResourceData, 0, len(secretList.Items))

	for _, secret := range secretList.Items {
		dataJSON, err := json.Marshal(secret.Data)
		if err != nil {
			return nil, fmt.Errorf("failed marshalling secret data to JSON for secret %s: %w", client.ObjectKeyFromObject(&secret), err)
		}

		dataList = append(dataList, gardencorev1beta1.GardenerResourceData{
			Name:   secret.Name,
			Labels: secret.Labels,
			Type:   v1beta1constants.DataTypeSecret,
			Data:   runtime.RawExtension{Raw: dataJSON},
		})
	}

	return dataList, nil
}

func computeExtensionsDataAndResources(
	ctx context.Context,
	seedClient client.Client,
	seedNamespace string,
) (
	[]gardencorev1beta1.ExtensionResourceState,
	[]gardencorev1beta1.ResourceData,
	error,
) {
	var (
		dataList  []gardencorev1beta1.ExtensionResourceState
		resources []gardencorev1beta1.ResourceData
	)

	for _, extension := range []struct {
		objKind           string
		newObjectListFunc func() client.ObjectList
	}{
		{extensionsv1alpha1.BackupEntryResource, func() client.ObjectList { return &extensionsv1alpha1.BackupEntryList{} }},
		{extensionsv1alpha1.ContainerRuntimeResource, func() client.ObjectList { return &extensionsv1alpha1.ContainerRuntimeList{} }},
		{extensionsv1alpha1.ControlPlaneResource, func() client.ObjectList { return &extensionsv1alpha1.ControlPlaneList{} }},
		{extensionsv1alpha1.DNSRecordResource, func() client.ObjectList { return &extensionsv1alpha1.DNSRecordList{} }},
		{extensionsv1alpha1.ExtensionResource, func() client.ObjectList { return &extensionsv1alpha1.ExtensionList{} }},
		{extensionsv1alpha1.InfrastructureResource, func() client.ObjectList { return &extensionsv1alpha1.InfrastructureList{} }},
		{extensionsv1alpha1.NetworkResource, func() client.ObjectList { return &extensionsv1alpha1.NetworkList{} }},
		{extensionsv1alpha1.OperatingSystemConfigResource, func() client.ObjectList { return &extensionsv1alpha1.OperatingSystemConfigList{} }},
		{extensionsv1alpha1.WorkerResource, func() client.ObjectList { return &extensionsv1alpha1.WorkerList{} }},
	} {
		objList := extension.newObjectListFunc()
		if err := seedClient.List(ctx, objList, client.InNamespace(seedNamespace)); err != nil {
			return nil, nil, fmt.Errorf("failed to list extension resources of kind %s: %w", extension.objKind, err)
		}

		if err := meta.EachListItem(objList, func(obj runtime.Object) error {
			extensionObj, err := apiextensions.Accessor(obj)
			if err != nil {
				return fmt.Errorf("failed accessing extension object: %w", err)
			}

			if extensionObj.GetDeletionTimestamp() != nil ||
				(extensionObj.GetExtensionStatus().GetState() == nil && len(extensionObj.GetExtensionStatus().GetResources()) == 0) {
				return nil
			}

			dataList = append(dataList, gardencorev1beta1.ExtensionResourceState{
				Kind:      extension.objKind,
				Name:      ptr.To(extensionObj.GetName()),
				Purpose:   extensionObj.GetExtensionSpec().GetExtensionPurpose(),
				State:     extensionObj.GetExtensionStatus().GetState(),
				Resources: extensionObj.GetExtensionStatus().GetResources(),
			})

			for _, newResource := range extensionObj.GetExtensionStatus().GetResources() {
				referencedObj, err := unstructuredutils.GetObjectByRef(ctx, seedClient, &newResource.ResourceRef, seedNamespace)
				if err != nil {
					return fmt.Errorf("failed reading referenced object %s: %w", client.ObjectKey{Name: newResource.ResourceRef.Name, Namespace: seedNamespace}, err)
				}
				if obj == nil {
					return fmt.Errorf("object %v not found", newResource.ResourceRef)
				}

				raw := &runtime.RawExtension{}
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(referencedObj, raw); err != nil {
					return fmt.Errorf("failed converting referenced object %s to raw extension: %w", client.ObjectKey{Name: newResource.ResourceRef.Name, Namespace: seedNamespace}, err)
				}

				resources = append(resources, gardencorev1beta1.ResourceData{
					CrossVersionObjectReference: newResource.ResourceRef,
					Data:                        *raw,
				})
			}

			return nil
		}); err != nil {
			return nil, nil, fmt.Errorf("failed computing extension data for kind %s: %w", extension.objKind, err)
		}
	}

	return dataList, resources, nil
}
