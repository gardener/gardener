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
