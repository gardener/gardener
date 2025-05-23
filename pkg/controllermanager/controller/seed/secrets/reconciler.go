// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Reconciler reconciles Seeds and creates a dedicated namespace for each seed in the garden cluster. It also syncs
// relevant garden secrets into this namespace.
type Reconciler struct {
	Client          client.Client
	GardenNamespace string
}

// Reconcile reconciles Seeds and creates a dedicated namespace for each seed in the garden cluster. It also syncs
// relevant garden secrets into this namespace.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	seed := &gardencorev1beta1.Seed{}
	if err := r.Client.Get(ctx, req.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   gardenerutils.ComputeGardenNamespace(seed.Name),
			Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSeed},
		},
	}
	log = log.WithValues("gardenNamespace", namespace.Name)

	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(namespace), namespace); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}

		// create namespace with controller ref to seed
		namespace.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(seed, gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"))})
		log.Info("Creating Namespace in garden for Seed")
		if err := r.Client.Create(ctx, namespace); err != nil {
			return reconcile.Result{}, err
		}
	} else {
		// namespace already exists, check if it has controller ref to seed
		if !metav1.IsControlledBy(namespace, seed) {
			return reconcile.Result{}, fmt.Errorf("namespace %q is not controlled by seed %q", namespace.Name, seed.Name)
		}

		if namespace.Labels[v1beta1constants.GardenRole] != v1beta1constants.GardenRoleSeed {
			patch := client.MergeFrom(namespace.DeepCopy())
			metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.GardenRole, v1beta1constants.GardenRoleSeed)
			if err := r.Client.Patch(ctx, namespace, patch); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	syncedSecrets, err := r.syncGardenSecrets(ctx, namespace)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to sync garden secrets: %v", err)
	}

	if err := r.cleanupStaleSecrets(ctx, syncedSecrets, namespace.Name); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to clean up secrets in seed namespace: %v", err)
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) cleanupStaleSecrets(ctx context.Context, existingSecrets []string, namespace string) error {
	var fns []flow.TaskFn
	exclude := sets.New(existingSecrets...)

	secretList := &corev1.SecretList{}
	if err := r.Client.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: gardenRoleSelector}); err != nil {
		return err
	}

	for _, s := range secretList.Items {
		secret := s
		if exclude.Has(secret.Name) {
			continue
		}
		fns = append(fns, func(ctx context.Context) error {
			return client.IgnoreNotFound(r.Client.Delete(ctx, &secret))
		})
	}

	return flow.Parallel(fns...)(ctx)
}

func (r *Reconciler) syncGardenSecrets(ctx context.Context, namespace *corev1.Namespace) ([]string, error) {
	secretList := &corev1.SecretList{}
	if err := r.Client.List(ctx, secretList, client.InNamespace(r.GardenNamespace), client.MatchingLabelsSelector{Selector: gardenRoleSelector}); err != nil {
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

			_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.Client, seedSecret, func() error {
				seedSecret.Annotations = secret.Annotations
				seedSecret.Labels = secret.Labels
				seedSecret.Type = secret.Type
				seedSecret.Data = secret.Data
				return nil
			})
			return err
		})
	}

	return secretNames, flow.Parallel(fns...)(ctx)
}
