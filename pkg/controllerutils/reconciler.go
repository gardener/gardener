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

package controllerutils

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type MultiplexReconciler struct {
	reconcilers map[string]reconcile.Reconciler
}

func NewMultiplexReconciler(reconcilers map[string]reconcile.Reconciler) *MultiplexReconciler {
	if reconcilers == nil {
		reconcilers = map[string]reconcile.Reconciler{}
	}

	return &MultiplexReconciler{
		reconcilers: reconcilers,
	}
}

func (r *MultiplexReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reconciler, exists := r.reconcilers[request.Namespace]
	if !exists {
		return reconcile.Result{}, fmt.Errorf("no reconciler registered for type %q", request.Namespace)
	}

	parts := strings.Split(request.Name, "/")
	if len(parts) != 2 {
		return reconcile.Result{}, fmt.Errorf("invalid request name %q", request.Name)
	}

	request.Namespace = parts[0]
	request.Name = parts[1]

	return reconciler.Reconcile(ctx, request)
}

func (r *MultiplexReconciler) NewRequest(kind string, name string, namespace string) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: kind,
			Name:      fmt.Sprintf("%s/%s", name, namespace),
		},
	}
}
