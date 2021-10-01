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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	secretFinalizers := sets.NewString(secret.Finalizers...)
	addFinalizer, removeFinalizer := false, false
	if secretIsReferenced && !secretFinalizers.Has(controllerFinalizer) {
		addFinalizer = true
		log.Info("adding finalizer to secret because it is referenced by a ManagedResource",
			"finalizer", controllerFinalizer)
	} else if !secretIsReferenced && secretFinalizers.Has(controllerFinalizer) {
		removeFinalizer = true
		log.Info("removing finalizer from secret because it is not referenced by a ManagedResource of this class",
			"finalizer", controllerFinalizer)
	}

	if addFinalizer || removeFinalizer {
		if err := controllerutils.TryUpdate(ctx, retry.DefaultBackoff, r.client, secret, func() error {
			secretFinalizers := sets.NewString(secret.Finalizers...)
			if addFinalizer {
				secretFinalizers.Insert(controllerFinalizer)
			} else if removeFinalizer {
				secretFinalizers.Delete(controllerFinalizer)
			}
			secret.Finalizers = secretFinalizers.UnsortedList()
			return nil
		}); client.IgnoreNotFound(err) != nil {
			r.log.Error(err, "failed to update finalizers of Secret")
			// dont' run into exponential backoff for adding/removing finalizers
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	return reconcile.Result{}, nil
}
