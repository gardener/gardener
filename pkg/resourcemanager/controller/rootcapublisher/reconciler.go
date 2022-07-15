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

package rootcapublisher

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// RootCACertConfigMapName is a constant for the name of the ConfigMap which contains the root CA certificate.
	RootCACertConfigMapName = "kube-root-ca.crt"
	// RootCADataKey is a constant for the data key in a ConfigMap containing the root CA certificate.
	RootCADataKey = "ca.crt"

	// DescriptionAnnotation is constant for annotation key of the config map.
	DescriptionAnnotation = "kubernetes.io/description"
)

type reconciler struct {
	targetClient client.Client
	rootCA       string
}

// NewReconciler is constructor only used in tests.
func NewReconciler(cl client.Client, rootCA string) *reconciler {
	return &reconciler{
		targetClient: cl,
		rootCA:       rootCA,
	}
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	namespace := &corev1.Namespace{}
	if err := r.targetClient.Get(ctx, req.NamespacedName, namespace); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	ownerReference := metav1.NewControllerRef(namespace, corev1.SchemeGroupVersion.WithKind("Namespace"))
	ownerReference.BlockOwnerDeletion = pointer.Bool(false)

	configmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RootCACertConfigMapName,
			Namespace: namespace.Name,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.targetClient, configmap, func() error {
		if _, found := configmap.Annotations[DescriptionAnnotation]; !found {
			configmap.Data = map[string]string{RootCADataKey: r.rootCA}
			configmap.OwnerReferences = []metav1.OwnerReference{*ownerReference}
		}

		return nil
	}); client.IgnoreNotFound(err) != nil && !apierrors.HasStatusCause(err, corev1.NamespaceTerminatingCause) {
		// don't retry a create if the namespace doesn't exist or is terminating
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
