// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificates

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/webhook"
)

// DefaultSyncPeriod is the default sync period for the certificate reconciler and reloader.
var DefaultSyncPeriod = 5 * time.Minute

// AddCertificateManagementToManager adds reconcilers to the given manager that manage the webhook certificates, namely
// - generate and auto-rotate the webhook CA and server cert using a secrets manager (in leader only)
// - fetch current webhook server cert and write it to disk for the webhook server to pick up (in all replicas)
func AddCertificateManagementToManager(
	ctx context.Context,
	mgr manager.Manager,
	sourceCluster cluster.Cluster,
	clock clock.Clock,
	sourceWebhookConfigs webhook.Configs,
	shootWebhookConfigs *webhook.Configs,
	atomicShootWebhookConfigs *atomic.Value,
	shootNamespaceSelector map[string]string,
	shootWebhookManagedResourceName string,
	componentName string,
	namespace string,
	mode string,
	url string,
) error {
	var (
		identity         = webhook.PrefixedName(componentName) + "-webhook"
		caSecretName     = "ca-" + componentName + "-webhook"
		serverSecretName = componentName + "-webhook-server"
	)

	// first, add reconciler that manages the certificates and injects them into webhook configs
	// (only running in the leader or once if no secrets have been generated yet)
	if err := (&reconciler{
		Clock:                           clock,
		SyncPeriod:                      DefaultSyncPeriod,
		SourceWebhookConfigs:            sourceWebhookConfigs,
		ShootWebhookConfigs:             shootWebhookConfigs,
		AtomicShootWebhookConfigs:       atomicShootWebhookConfigs,
		CASecretName:                    caSecretName,
		ServerSecretName:                serverSecretName,
		Namespace:                       namespace,
		Identity:                        identity,
		ComponentName:                   componentName,
		ShootWebhookManagedResourceName: shootWebhookManagedResourceName,
		ShootNamespaceSelector:          shootNamespaceSelector,
		Mode:                            mode,
		URL:                             url,
	}).AddToManager(ctx, mgr, sourceCluster); err != nil {
		return fmt.Errorf("failed to add webhook server certificate reconciler: %w", err)
	}

	// secondly, add reloader that fetches the managed certificates and writes it to the webhook server's cert dir
	// (running in all replicas)
	if err := (&reloader{
		SyncPeriod:       DefaultSyncPeriod,
		ServerSecretName: serverSecretName,
		Namespace:        namespace,
		Identity:         identity,
	}).AddToManager(ctx, mgr, sourceCluster); err != nil {
		return fmt.Errorf("failed to add webhook server certificate reloader: %w", err)
	}

	return nil
}
