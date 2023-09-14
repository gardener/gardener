// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package secret

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/apis/config"
	"github.com/gardener/gardener/pkg/resourcemanager/predicate"
)

// Reconciler adds/removes finalizers to/from secrets referenced by ManagedResources.
type Reconciler struct {
	SourceClient client.Client
	Config       config.SecretControllerConfig
	ClassFilter  *predicate.ClassFilter
}

// Reconcile implements reconcile.Reconciler.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	secret := &corev1.Secret{}
	if err := r.SourceClient.Get(ctx, req.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	resourceList := &resourcesv1alpha1.ManagedResourceList{}
	if err := r.SourceClient.List(ctx, resourceList, client.InNamespace(secret.Namespace)); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not fetch ManagedResources in namespace of Secret: %+v", err)
	}

	// check if there is at least one ManagedResource this controller is responsible for and which references this secret
	secretIsReferenced := false
	var referencedMRResource resourcesv1alpha1.ManagedResource
	for _, resource := range resourceList.Items {
		for _, ref := range resource.Spec.SecretRefs {
			// check if we are responsible for this MR, class might have changed, then we need to remove our finalizer
			if ref.Name == secret.Name && r.ClassFilter.Responsible(&resource) {
				secretIsReferenced = true
				referencedMRResource = resource
				break
			}
		}
	}
	if secretIsReferenced && !metav1.HasLabel(secret.ObjectMeta, resourcesv1alpha1.ReferencedBy) {
		patch := client.MergeFromWithOptions(secret.DeepCopy(), client.MergeFromWithOptimisticLock{})
		metav1.SetMetaDataLabel(&secret.ObjectMeta, resourcesv1alpha1.ReferencedBy, referencedMRResource.Name)
		if err := r.SourceClient.Patch(ctx, secret, patch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add the %s label to secret: %w", resourcesv1alpha1.ReferencedBy, err)
		}
	}
	controllerFinalizer := r.ClassFilter.FinalizerName()
	hasFinalizer := controllerutil.ContainsFinalizer(secret, controllerFinalizer)
	if secretIsReferenced && !hasFinalizer {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.SourceClient, secret, controllerFinalizer); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	} else if !secretIsReferenced && hasFinalizer {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.SourceClient, secret, controllerFinalizer); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}
