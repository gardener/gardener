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

package seed

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// SeedControllerName is the name of the seed controller.
	SeedControllerName = "seed"
)

func addSeedController(
	ctx context.Context,
	mgr manager.Manager,
	config *config.SeedControllerConfiguration,
) error {
	logger := mgr.GetLogger()
	gardenClient := mgr.GetClient()

	ctrlOptions := controller.Options{
		Reconciler:              NewDefaultBackupBucketControl(logger, gardenClient),
		MaxConcurrentReconciles: config.ConcurrentSyncs,
	}
	c, err := controller.New(SeedControllerName, mgr, ctrlOptions)
	if err != nil {
		return err
	}

	seed := &gardencorev1beta1.Seed{}
	if err := c.Watch(&source.Kind{Type: seed}, &handler.EnqueueRequestForObject{}); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", seed, err)
	}

	// TODO: Filter! filterGardenSecret
	secret := &corev1.Secret{}
	if err := c.Watch(&source.Kind{Type: secret}, newSecretEventHandler(ctx, gardenClient, logger)); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", secret, err)
	}

	return nil
}

// NewDefaultControl returns a new instance of the default implementation that
// implements the documented semantics for seeds.
// You should use an instance returned from NewDefaultControl() for any scenario other than testing.
func NewDefaultControl(logger logr.Logger, gardenClient client.Client) *reconciler {
	return &reconciler{
		logger:       logger,
		gardenClient: gardenClient,
	}
}

type reconciler struct {
	logger       logr.Logger
	gardenClient client.Client
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	seed := &gardencorev1beta1.Seed{}
	logger := r.logger.WithValues("seed", req.Name)

	err := r.gardenClient.Get(ctx, req.NamespacedName, seed)
	if apierrors.IsNotFound(err) {
		logger.Info("[SEED] Stopping operations for Seed since it has been deleted")
		return reconcile.Result{}, nil
	}
	if err != nil {
		logger.Error(err, "[SEED] Unable to retrieve object from store")
		return reconcile.Result{}, err
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: gutil.ComputeGardenNamespace(seed.Name),
		},
	}

	if err := r.gardenClient.Get(ctx, client.ObjectKeyFromObject(namespace), namespace); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}

		// create namespace with controller ref to seed
		namespace.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(seed, gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"))})
		if err := r.gardenClient.Create(ctx, namespace); err != nil {
			return reconcile.Result{}, err
		}
	} else {
		// namespace already exists, check if it has controller ref to seed
		if !metav1.IsControlledBy(namespace, seed) {
			return reconcile.Result{}, fmt.Errorf("namespace %q is not controlled by seed %q", namespace.Name, seed.Name)
		}
	}

	syncedSecrets, err := r.syncGardenSecrets(ctx, r.gardenClient, namespace)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to sync garden secrets: %w", err)
	}

	if err := r.cleanupStaleSecrets(ctx, r.gardenClient, syncedSecrets, namespace.Name); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to clean up secrets in seed namespace: %w", err)
	}

	return reconcile.Result{}, nil
}

var (
	gardenRoleReq      = utils.MustNewRequirement(v1beta1constants.GardenRole, selection.Exists)
	gardenRoleSelector = labels.NewSelector().Add(gardenRoleReq).Add(gutil.NoControlPlaneSecretsReq)
)

func (r *reconciler) cleanupStaleSecrets(ctx context.Context, gardenClient client.Client, existingSecrets []string, namespace string) error {
	var fns []flow.TaskFn
	exclude := sets.NewString(existingSecrets...)

	secretList := &corev1.SecretList{}
	if err := r.gardenClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: gardenRoleSelector}); err != nil {
		return err
	}

	for _, s := range secretList.Items {
		secret := s
		if exclude.Has(secret.Name) {
			continue
		}
		fns = append(fns, func(ctx context.Context) error {
			return client.IgnoreNotFound(gardenClient.Delete(ctx, &secret))
		})
	}

	return flow.Parallel(fns...)(ctx)
}

func (r *reconciler) syncGardenSecrets(ctx context.Context, gardenClient client.Client, namespace *corev1.Namespace) ([]string, error) {
	secretList := &corev1.SecretList{}
	if err := r.gardenClient.List(ctx, secretList, client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabelsSelector{Selector: gardenRoleSelector}); err != nil {
		return nil, err
	}

	var (
		fns         []flow.TaskFn
		secretNames []string
	)

	for _, s := range secretList.Items {
		secret := s
		secretNames = append(secretNames, secret.Name)
		fns = append(fns, func(ctx context.Context) error {
			seedSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secret.Name,
					Namespace: namespace.Name,
				},
			}

			if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, gardenClient, seedSecret, func() error {
				seedSecret.Annotations = secret.Annotations
				seedSecret.Labels = secret.Labels
				seedSecret.Data = secret.Data
				return nil
			}); err != nil {
				return err
			}
			return nil
		})
	}

	return secretNames, flow.Parallel(fns...)(ctx)
}
