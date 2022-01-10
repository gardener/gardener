// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller

import (
	"context"
	"fmt"

	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	labelSelector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			gardencorev1beta1constants.LabelApp: "virtual-garden",
			"component":                         gardencorev1beta1constants.DeploymentNameKubeAPIServer,
		},
	}
	selector, _ = metav1.LabelSelectorAsSelector(&labelSelector)
)

// GetVirtualGardenClusterEndpoint gets the virtual garden service from the runtime cluster and determines the virtual garden cluster endpoint
func (o *operation) GetVirtualGardenClusterEndpoint(ctx context.Context) error {
	serviceList := &corev1.ServiceList{}
	if err := o.runtimeClient.Client().List(ctx, serviceList,
		client.InNamespace(gardencorev1beta1constants.GardenNamespace),
		client.MatchingLabelsSelector{Selector: selector}); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to retrieve virtual garden service from the runtime cluster: %v", err)
		}
		return fmt.Errorf("missing virtual garden service in the runtime cluster: %w", err)
	}

	if len(serviceList.Items) != 1 {
		return fmt.Errorf("expected exactly one virtual garden service in the runtime cluster")
	}

	virtualGardenService := serviceList.Items[0]
	if len(virtualGardenService.Spec.Ports) == 0 {
		return fmt.Errorf("expected the virtual garden service in the runtime cluster to have at least one port")
	}

	o.VirtualGardenClusterEndpoint = pointer.String(fmt.Sprintf("%s:%d", virtualGardenService.Name, virtualGardenService.Spec.Ports[0].Port))
	return nil
}
