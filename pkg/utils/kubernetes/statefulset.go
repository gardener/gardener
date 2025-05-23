// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetContainerResourcesInStatefulSet returns the containers resources in StatefulSet.
func GetContainerResourcesInStatefulSet(ctx context.Context, k8sClient client.Client, key client.ObjectKey) (map[string]*corev1.ResourceRequirements, error) {
	statefulSet := &appsv1.StatefulSet{}
	if err := k8sClient.Get(ctx, key, statefulSet); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	resourcesPerContainer := make(map[string]*corev1.ResourceRequirements)

	for _, container := range statefulSet.Spec.Template.Spec.Containers {
		resourcesPerContainer[container.Name] = container.Resources.DeepCopy()
	}

	return resourcesPerContainer, nil
}
