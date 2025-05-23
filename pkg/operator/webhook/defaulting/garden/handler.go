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
)

// Handler performs defaulting.
type Handler struct{}

// Default performs the defaulting.
func (h *Handler) Default(_ context.Context, obj runtime.Object) error {
	garden, ok := obj.(*operatorv1alpha1.Garden)
	if !ok {
		return fmt.Errorf("expected *operatorv1alpha1.Garden but got %T", obj)
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

	return nil
}
