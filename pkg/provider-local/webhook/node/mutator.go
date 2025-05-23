// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/provider-local/local"
)

type mutator struct{}

func (m *mutator) Mutate(_ context.Context, newObj, _ client.Object) error {
	node, ok := newObj.(*corev1.Node)
	if !ok {
		return fmt.Errorf("unexpected object, got %T wanted *corev1.Node", newObj)
	}

	for _, resourceList := range []corev1.ResourceList{node.Status.Allocatable, node.Status.Capacity} {
		resourceList[corev1.ResourceCPU] = local.NodeResourceCPU
		resourceList[corev1.ResourceMemory] = local.NodeResourceMemory
	}

	return nil
}
