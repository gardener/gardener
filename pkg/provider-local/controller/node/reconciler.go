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

package node

import (
	"context"

	"github.com/gardener/gardener/pkg/provider-local/local"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconciler struct {
	logger logr.Logger
	client client.Client
}

// NewReconciler creates a new reconcile.Reconciler that reconciles Nodes.
func NewReconciler() reconcile.Reconciler {
	return &reconciler{
		logger: log.Log.WithName(ControllerName),
	}
}

func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	node := &corev1.Node{}
	if err := r.client.Get(ctx, request.NamespacedName, node); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	for _, resourceList := range []corev1.ResourceList{node.Status.Allocatable, node.Status.Capacity} {
		if !resourceList[corev1.ResourceCPU].Equal(local.NodeResourceCPU) || !resourceList[corev1.ResourceMemory].Equal(local.NodeResourceMemory) {
			// submit empty patch to trigger 'node' webhook which will update these values
			return reconcile.Result{}, r.client.Status().Patch(ctx, node, client.RawPatch(types.StrategicMergePatchType, []byte("{}")))
		}
	}

	return reconcile.Result{}, nil
}
