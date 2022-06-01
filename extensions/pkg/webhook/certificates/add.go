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

package certificates

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// DefaultSyncPeriod is the default sync period for the certificate reconciler and reloader.
var DefaultSyncPeriod = 5 * time.Minute

// AddCertificateManagementToManager adds reconcilers to the given manager that manage the webhook certificates, namely
// - generate and auto-rotate the webhook CA and server cert using a secrets manager (in leader only)
// - fetch current webhook server cert and write it to disk for the webhook server to pick up (in all replicas)
func AddCertificateManagementToManager(
	ctx context.Context,
	mgr manager.Manager,
	clock clock.Clock,
	seedWebhookConfig, shootWebhookConfig *admissionregistrationv1.MutatingWebhookConfiguration,
	atomicShootWebhookConfig *atomic.Value,
	extensionName string,
	shootWebhookManagedResourceName string,
	shootNamespaceSelector map[string]string,
	namespace, mode, url string,
) error {
	var (
		identity         = "gardener-extension-" + extensionName + "-webhook"
		caSecretName     = "ca-" + extensionName + "-webhook"
		serverSecretName = extensionName + "-webhook-server"
	)

	// first, add reconciler that manages the certificates and injects them into webhook configs
	// (only running in the leader or once if no secrets have been generated yet)
	if err := (&reconciler{
		Clock:                           clock,
		SyncPeriod:                      DefaultSyncPeriod,
		SeedWebhookConfig:               seedWebhookConfig,
		ShootWebhookConfig:              shootWebhookConfig,
		AtomicShootWebhookConfig:        atomicShootWebhookConfig,
		CASecretName:                    caSecretName,
		ServerSecretName:                serverSecretName,
		Namespace:                       namespace,
		Identity:                        identity,
		ExtensionName:                   extensionName,
		ShootWebhookManagedResourceName: shootWebhookManagedResourceName,
		ShootNamespaceSelector:          shootNamespaceSelector,
		Mode:                            mode,
		URL:                             url,
	}).AddToManager(ctx, mgr); err != nil {
		return fmt.Errorf("failed to add webhook server certificate reconciler: %w", err)
	}

	// secondly, add reloader that fetches the managed certificates and writes it to the webhook server's cert dir
	// (running in all replicas)
	if err := (&reloader{
		SyncPeriod:       DefaultSyncPeriod,
		ServerSecretName: serverSecretName,
		Namespace:        namespace,
		Identity:         identity,
	}).AddToManager(ctx, mgr); err != nil {
		return fmt.Errorf("failed to add webhook server certificate reloader: %w", err)
	}

	return nil
}
