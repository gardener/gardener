// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
