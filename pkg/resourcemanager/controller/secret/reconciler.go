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

package secret

import (
	"context"
	"fmt"
	"time"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/predicate"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler adds/removes finalizers to/from secrets referenced by ManagedResources.
type Reconciler struct {
	log    logr.Logger
	client client.Client

	ClassFilter *predicate.ClassFilter
}

// InjectClient injects a client into the reconciler.
func (r *Reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

// InjectLogger injects a logger into the reconciler.
func (r *Reconciler) InjectLogger(l logr.Logger) error {
	r.log = l.WithName(ControllerName)
	return nil
}

// Reconcile implements reconcile.Reconciler.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	log := r.log.WithValues("secret", req)

	secret := &corev1.Secret{}
	if err := r.client.Get(ctx, req.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Stopping reconciliation of Secret, as it has been deleted")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("could not fetch Secret: %+v", err)
	}

	resourceList := &resourcesv1alpha1.ManagedResourceList{}
	if err := r.client.List(ctx, resourceList, client.InNamespace(secret.Namespace)); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not fetch ManagedResources in namespace of Secret: %+v", err)
	}

	// check if there is at least one ManagedResource this controller is responsible for and which references this secret
	secretIsReferenced := false
	for _, resource := range resourceList.Items {
		for _, ref := range resource.Spec.SecretRefs {
			// check if we are responsible for this MR, class might have changed, then we need to remove our finalizer
			if ref.Name == secret.Name && r.ClassFilter.Responsible(&resource) {
				secretIsReferenced = true
				break
			}
		}
	}

	controllerFinalizer := r.ClassFilter.FinalizerName()
	hasFinalizer := controllerutil.ContainsFinalizer(secret, controllerFinalizer)
	if secretIsReferenced && !hasFinalizer {
		log.Info("Adding finalizer to Secret because it is referenced by a ManagedResource",
			"finalizer", controllerFinalizer)

		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, r.client, secret, controllerFinalizer); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not add finalizer to Secret: %w", err)
		}
	} else if !secretIsReferenced && hasFinalizer {
		log.Info("Removing finalizer from Secret because it is not referenced by a ManagedResource of this class",
			"finalizer", controllerFinalizer)

		if err := controllerutils.PatchRemoveFinalizers(ctx, r.client, secret, controllerFinalizer); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer from Secret: %w", err)
		}
	}

	return reconcile.Result{}, nil
}
