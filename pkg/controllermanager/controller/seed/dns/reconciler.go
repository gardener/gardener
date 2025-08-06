// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dns

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Reconciler reconciles Seeds and updates their internal DNS settings if necessary.
// If the seed does not have spec.dns.internal set, it fetches the internal domain secret
// and configures the seed explicitly.
type Reconciler struct {
	Client          client.Client
	GardenNamespace string
}

// Reconcile reconciles Seeds and sets their internal DNS settings if necessary.
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

	// If the Seed does not have spec.dns.internal set, set it automatically from the internal domain.
	if seed.Spec.DNS.Internal == nil {
		// TODO(dimityrmirchev): This logic should eventually be deprecated and removed
		secret, err := gardenerutils.ReadInternalDomainSecret(ctx, r.Client, r.GardenNamespace, true)
		if err != nil {
			return reconcile.Result{}, err
		}

		providerType, domain, zone, err := gardenerutils.GetDomainInfoFromAnnotations(secret.Annotations)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Patch the Seed with the internal domain info
		patch := client.MergeFrom(seed.DeepCopy())
		seed.Spec.DNS.Internal = &gardencorev1beta1.SeedDNSProviderConfig{
			Type:   providerType,
			Domain: domain,
		}
		if len(zone) > 0 {
			seed.Spec.DNS.Internal.Zone = &zone
		}

		seed.Spec.DNS.Internal.CredentialsRef = corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  secret.Namespace,
			Name:       secret.Name,
		}

		if err := r.Client.Patch(ctx, seed, patch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to patch Seed with internal DNS: %w", err)
		}
	}

	return reconcile.Result{}, nil
}
