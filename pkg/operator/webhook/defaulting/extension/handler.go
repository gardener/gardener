// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// Handler performs defaulting.
type Handler struct{}

// Default performs the defaulting.
func (h *Handler) Default(_ context.Context, obj runtime.Object) error {
	extension, ok := obj.(*operatorv1alpha1.Extension)
	if !ok {
		return fmt.Errorf("expected *operatorv1alpha1.Extension but got %T", obj)
	}

	if slices.ContainsFunc(extension.Spec.Resources, func(resource gardencorev1beta1.ControllerResource) bool {
		return resource.Kind == extensionsv1alpha1.WorkerResource
	}) && extension.Spec.Deployment != nil && extension.Spec.Deployment.ExtensionDeployment != nil && extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig == nil {
		extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig = ptr.To(true)
	}

	for i, resource := range extension.Spec.Resources {
		if resource.Primary == nil {
			extension.Spec.Resources[i].Primary = ptr.To(true)
		}
	}

	return nil
}
