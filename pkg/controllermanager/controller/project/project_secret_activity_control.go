// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const secretActivityReconcilerName = "secretActivity"

// NewSecretActivityReconciler creates a new instance of a reconciler which reconciles the LastActivityTimestamp of a project, then it calls the stale reconciler.
func NewSecretActivityReconciler(gardenClient client.Client) reconcile.Reconciler {
	return &projectSecretActivityReconciler{
		gardenClient: gardenClient,
	}
}

type projectSecretActivityReconciler struct {
	gardenClient client.Client
}

func (r *projectSecretActivityReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	secret := &corev1.Secret{}
	if err := r.gardenClient.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: request.Name}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	return r.reconcile(ctx, log, secret)
}

func (r *projectSecretActivityReconciler) reconcile(ctx context.Context, log logr.Logger, obj client.Object) (reconcile.Result, error) {
	project, err := gutil.ProjectForNamespaceFromReader(ctx, r.gardenClient, obj.GetNamespace())
	if err != nil {
		if apierrors.IsNotFound(err) {
			// project is gone, nothing to do here
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to get project for namespace %q: %w", obj.GetNamespace(), err)
	}

	if project.Status.LastActivityTimestamp != nil && obj.GetCreationTimestamp().UTC().Before(project.Status.LastActivityTimestamp.UTC()) {
		// not the newest object in this project, nothing to do
		return reconcile.Result{}, nil
	}

	timestamp := obj.GetCreationTimestamp()
	if project.Status.LastActivityTimestamp == nil {
		secrets := &corev1.SecretList{}
		if err := r.gardenClient.List(
			ctx,
			secrets,
			client.InNamespace(*project.Spec.Namespace),
			client.MatchingLabels{v1beta1constants.LabelSecretBindingReference: "true"},
		); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to list secrets in project namespace %q: %w", *project.Spec.Namespace, err)
		}
		for _, secret := range secrets.Items {
			if secret.GetCreationTimestamp().UTC().After(timestamp.UTC()) {
				timestamp = secret.GetCreationTimestamp()
			}
		}
	}

	log.Info("Updating Project's lastActivityTimestamp", "lastActivityTimestamp", timestamp)
	if err := updateStatus(ctx, r.gardenClient, project, func() {
		project.Status.LastActivityTimestamp = &timestamp
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed updating Project's lastActivityTimestamp: %w", err)
	}
	return reconcile.Result{}, nil
}

func (c *Controller) updateSecretActivity(oldObj, newObj interface{}) {
	newSecret, ok := newObj.(*corev1.Secret)
	if !ok {
		return
	}
	oldSecret, ok := oldObj.(*corev1.Secret)
	if !ok {
		return
	}

	// skip queueing if oldSecret has the label, which implies it already caused a LastActivityTimestamp update
	// or the newSecret doesn't have the label, which means it is not referred anymore and shouldn't be considered an "activity".
	if metav1.HasLabel(oldSecret.ObjectMeta, v1beta1constants.LabelSecretBindingReference) ||
		!metav1.HasLabel(newSecret.ObjectMeta, v1beta1constants.LabelSecretBindingReference) {
		return
	}

	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		c.log.Error(err, "Couldn't get key for object", "object", newObj)
		return
	}
	c.projectSecretActivityQueue.Add(key)
}
