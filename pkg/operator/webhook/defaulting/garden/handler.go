// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	operatorv1alpha1helper "github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
)

// Handler performs defaulting.
type Handler struct{}

// Default performs the defaulting.
func (h *Handler) Default(_ context.Context, obj runtime.Object) error {
	garden, ok := obj.(*operatorv1alpha1.Garden)
	if !ok {
		return fmt.Errorf("expected *operatorv1alpha1.Garden but got %T", obj)
	}

	if len(garden.Spec.RuntimeCluster.Networking.IPFamilies) == 0 {
		garden.Spec.RuntimeCluster.Networking.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4}
	}

	if garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer == nil {
		garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer = &operatorv1alpha1.KubeAPIServerConfig{}
	}
	if garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.KubeAPIServerConfig == nil {
		garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.KubeAPIServerConfig = &gardencorev1beta1.KubeAPIServerConfig{}
	}

	gardencorev1beta1.SetDefaults_KubeAPIServerConfig(garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.KubeAPIServerConfig)

	if garden.Spec.VirtualCluster.Kubernetes.KubeControllerManager == nil {
		garden.Spec.VirtualCluster.Kubernetes.KubeControllerManager = &operatorv1alpha1.KubeControllerManagerConfig{}
	}
	if garden.Spec.VirtualCluster.Kubernetes.KubeControllerManager.KubeControllerManagerConfig == nil {
		garden.Spec.VirtualCluster.Kubernetes.KubeControllerManager.KubeControllerManagerConfig = &gardencorev1beta1.KubeControllerManagerConfig{}
	}

	if garden.Spec.VirtualCluster.Gardener.APIServer == nil {
		garden.Spec.VirtualCluster.Gardener.APIServer = &operatorv1alpha1.GardenerAPIServerConfig{}
	}
	if garden.Spec.VirtualCluster.Gardener.APIServer.EncryptionConfig == nil {
		garden.Spec.VirtualCluster.Gardener.APIServer.EncryptionConfig = &gardencorev1beta1.EncryptionConfig{}
	}
	if garden.Spec.VirtualCluster.Gardener.APIServer.EncryptionConfig.Provider.Type == nil {
		garden.Spec.VirtualCluster.Gardener.APIServer.EncryptionConfig.Provider.Type = garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.EncryptionConfig.Provider.Type
	}

	if garden.Status.Credentials == nil {
		garden.Status.Credentials = &operatorv1alpha1.Credentials{}
	}
	if garden.Status.Credentials.EncryptionAtRest == nil {
		garden.Status.Credentials.EncryptionAtRest = &operatorv1alpha1.EncryptionAtRest{}
	}

	if len(garden.Status.Credentials.EncryptionAtRest.ProviderType) == 0 {
		garden.Status.Credentials.EncryptionAtRest.ProviderType = operatorv1alpha1helper.GetEncryptionProviderType(garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer)
	}

	// Defaulting used for migration from `.status.encryptedResources` to `status.credentials.encryptionAtRest.resources`.
	// TODO(AleksandarSavchev): Remove this block after v1.135 has been released.
	if len(garden.Status.EncryptedResources) > 0 && len(garden.Status.Credentials.EncryptionAtRest.Resources) == 0 {
		garden.Status.Credentials.EncryptionAtRest.Resources = garden.Status.EncryptedResources
	}

	return nil
}
