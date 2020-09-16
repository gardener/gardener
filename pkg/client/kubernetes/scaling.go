// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
