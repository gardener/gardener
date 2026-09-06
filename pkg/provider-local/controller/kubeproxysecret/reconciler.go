// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubeproxysecret

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kubeproxy "github.com/gardener/gardener/pkg/component/kubernetes/proxy"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const dataKeyConfig = "config.yaml"

// Reconciler deletes stale kube-proxy ManagedResource secrets whose rendered kube-proxy configuration
// still enables conntrack maxPerCore, so that gardenlet recreates them via a CREATE request which is
// intercepted by the kube-proxy-config seed webhook (which sets maxPerCore to 0).
//
// This is required on upgrade/wake-up: the kube-proxy ManagedResource secret is immutable and
// content-addressed. When it was created by a previous Gardener release without the webhook, the new
// gardenlet recomputes byte-for-byte identical content, so its controllerutil.CreateOrUpdate is a
// no-op - no admission request is generated and the mutating webhook is never invoked. Deleting the
// stale secret forces the next reconcile to issue a CREATE, which the webhook does intercept.
//
// TODO: Remove this controller one release after the kube-proxy-config seed webhook was introduced.
// From then on the "previous" Gardener release under test already contains the webhook, so the
// kube-proxy ManagedResource secret is always created with conntrack maxPerCore set to 0 and this
// migration shim is no longer needed.
type Reconciler struct {
	// Client is the client for making requests to the seed cluster.
	Client client.Client
	// Decoder is used to extract the objects encoded in the ManagedResource secret.
	Decoder runtime.Decoder
	// Codec is used to decode the kube-proxy component configuration.
	Codec kubeproxy.ConfigCodec
}

// Reconcile deletes the kube-proxy ManagedResource secret if its kube-proxy configuration still has a
// non-zero conntrack maxPerCore value.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, request.NamespacedName, secret); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if secret.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	objects, err := managedresources.ExtractObjectsFromSecret(r.Decoder, secret)
	if err != nil {
		// The secret might not (yet) contain renderable manifests; nothing we can act on.
		log.V(1).Info("Could not extract objects from kube-proxy managed resource secret, skipping", "reason", err.Error())
		return reconcile.Result{}, nil
	}

	for _, obj := range objects {
		configMap, ok := obj.(*corev1.ConfigMap)
		if !ok || !strings.HasPrefix(configMap.Name, kubeproxy.ConfigNamePrefix) {
			continue
		}

		raw, ok := configMap.Data[dataKeyConfig]
		if !ok {
			continue
		}

		config, err := r.Codec.Decode(raw)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("could not decode kube-proxy configuration from ConfigMap %q: %w", configMap.Name, err)
		}

		if ptr.Deref(config.Conntrack.MaxPerCore, 0) == 0 {
			// The webhook already disabled conntrack maxPerCore (or there is nothing to do), leave the secret as is.
			return reconcile.Result{}, nil
		}

		log.Info("Deleting stale kube-proxy managed resource secret to trigger recreation and webhook mutation of conntrack maxPerCore", "secret", client.ObjectKeyFromObject(secret))
		if err := r.Client.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("could not delete stale kube-proxy managed resource secret %q: %w", client.ObjectKeyFromObject(secret), err)
		}
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, nil
}
