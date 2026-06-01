// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package imagepullsecret

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Reconciler watches image pull secrets in the garden namespace and copies them into the
// seed-scoped namespaces (seed-<name>) for each seed listed in the secret's
// gardener.cloud/seed-names annotation.
type Reconciler struct {
	Client          client.Client
	GardenNamespace string
}

// Reconcile copies an image pull secret to the seed-<name> namespaces of all seeds listed in
// the secret's annotation, and removes stale copies from seeds that were removed from it.
// Only seeds that are actually registered in this cluster (have a seed-<name> namespace with
// the gardener.cloud/role=seed label) are considered — annotation entries for non-existent
// seeds are silently ignored.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	// List all registered seed namespaces up front. Both sync and cleanup operate over this
	// set so that annotation entries for non-existent seeds are never acted upon.
	seedNamespaces, err := r.listSeedNamespaces(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, req.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Secret is gone, cleaning up copies from all seed namespaces")
			return reconcile.Result{}, r.deleteFromNamespaces(ctx, req.Name, seedNamespaces)
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	desiredSeeds := parseSeedNames(secret.Annotations[v1beta1constants.AnnotationImagePullSecretSeedNames])

	if err := r.reconcileSeedNamespaces(ctx, secret, desiredSeeds, seedNamespaces); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to reconcile image pull secret in seed namespaces: %w", err)
	}

	return reconcile.Result{}, nil
}

// reconcileSeedNamespaces syncs the secret into seed namespaces whose seed name is in desiredSeeds,
// and deletes it from seed namespaces whose seed name is not in desiredSeeds.
func (r *Reconciler) reconcileSeedNamespaces(ctx context.Context, secret *corev1.Secret, desiredSeeds sets.Set[string], seedNamespaces []string) error {
	var fns []flow.TaskFn

	for _, namespace := range seedNamespaces {
		seedName := strings.TrimPrefix(namespace, gardenerutils.SeedNamespaceNamePrefix)
		ns := namespace

		if desiredSeeds.Has(seedName) {
			fns = append(fns, func(ctx context.Context) error {
				copy := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secret.Name,
						Namespace: ns,
					},
				}
				_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.Client, copy, func() error {
					copy.Annotations = secret.Annotations
					copy.Labels = secret.Labels
					copy.Type = secret.Type
					copy.Data = secret.Data
					return nil
				})
				return err
			})
		} else {
			fns = append(fns, func(ctx context.Context) error {
				return client.IgnoreNotFound(r.Client.Delete(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: secret.Name, Namespace: ns},
				}))
			})
		}
	}

	return flow.Parallel(fns...)(ctx)
}

func (r *Reconciler) deleteFromNamespaces(ctx context.Context, secretName string, namespaces []string) error {
	var fns []flow.TaskFn
	for _, namespace := range namespaces {
		ns := namespace
		fns = append(fns, func(ctx context.Context) error {
			return client.IgnoreNotFound(r.Client.Delete(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns},
			}))
		})
	}
	return flow.Parallel(fns...)(ctx)
}

func (r *Reconciler) listSeedNamespaces(ctx context.Context) ([]string, error) {
	namespaceList := &corev1.NamespaceList{}
	if err := r.Client.List(ctx, namespaceList, client.MatchingLabels{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleSeed,
	}); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(namespaceList.Items))
	for _, ns := range namespaceList.Items {
		names = append(names, ns.Name)
	}
	return names, nil
}

func parseSeedNames(annotation string) sets.Set[string] {
	result := sets.New[string]()
	for _, name := range strings.Split(annotation, ",") {
		if name = strings.TrimSpace(name); name != "" {
			result.Insert(name)
		}
	}
	return result
}
