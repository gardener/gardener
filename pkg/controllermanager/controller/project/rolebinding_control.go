// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package project

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/go-logr/logr"
)

func newRoleBindingEventHandler(ctx context.Context, c client.Client, logger logr.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		name := obj.GetName()

		if name == "gardener.cloud:system:project-member" ||
			name == "gardener.cloud:system:project-viewer" ||
			strings.HasPrefix(name, "gardener.cloud:extension:project:") {

			namespace := obj.GetNamespace()

			project, err := gutil.ProjectForNamespaceFromReader(ctx, c, namespace)
			if err != nil {
				logger.WithValues("namespace", namespace).Error(err, "Failed to get project for RoleBinding")
				return nil
			}

			if project.DeletionTimestamp == nil {
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Namespace: "",
						Name:      project.Name,
					},
				}}
			}
		}

		return nil
	})
}
