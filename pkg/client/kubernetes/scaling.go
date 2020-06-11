// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ScaleStatefulSet scales a StatefulSet.
func ScaleStatefulSet(ctx context.Context, c client.Client, key client.ObjectKey, replicas int32) error {
	// TODO: replace this with call to scale subresource once controller-runtime supports it
	// see: https://github.com/kubernetes-sigs/controller-runtime/issues/172
	statefulset := &appsv1.StatefulSet{}
	if err := c.Get(ctx, key, statefulset); err != nil {
		return err
	}

	statefulset.Spec.Replicas = &replicas
	return c.Update(ctx, statefulset)
}

// ScaleEtcd scales a Etcd resource.
func ScaleEtcd(ctx context.Context, c client.Client, key client.ObjectKey, replicas int) error {
	// TODO: replace this with call to scale subresource once controller-runtime supports it
	// see: https://github.com/kubernetes-sigs/controller-runtime/issues/172
	etcd := &druidv1alpha1.Etcd{}
	if err := c.Get(ctx, key, etcd); err != nil {
		return err
	}

	etcd.Spec.Replicas = replicas
	return c.Update(ctx, etcd)
}

// ScaleDeployment scales a Deployment.
func ScaleDeployment(ctx context.Context, c client.Client, key client.ObjectKey, replicas int32) error {
	// TODO: replace this with call to scale subresource once controller-runtime supports it
	// see: https://github.com/kubernetes-sigs/controller-runtime/issues/172
	deployment := &appsv1.Deployment{}
	if err := c.Get(ctx, key, deployment); err != nil {
		return err
	}

	deployment.Spec.Replicas = &replicas
	return c.Update(ctx, deployment)
}
