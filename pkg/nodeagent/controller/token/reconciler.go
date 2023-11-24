// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package token

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/nodeagent/apis/config"
)

// Reconciler fetches the shoot access token for gardener-node-agent and writes it to disk.
type Reconciler struct {
	APIReader client.Reader
	Config    config.TokenControllerConfig
	FS        afero.Afero

	secretNameToPath map[string]string
}

// Reconcile fetches the shoot access token for gardener-node-agent and writes it to disk.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	secret := &corev1.Secret{}
	if err := r.APIReader.Get(ctx, request.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object not found")
			return reconcile.Result{}, fmt.Errorf("secret %s not found: %w", request.NamespacedName, err)
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	path, ok := r.secretNameToPath[secret.Name]
	if !ok {
		return reconcile.Result{}, fmt.Errorf("failed determining the path where to sync the token to (unknown secret name %q)", secret.Name)
	}

	token := secret.Data[resourcesv1alpha1.DataKeyToken]
	if len(token) == 0 {
		return reconcile.Result{}, fmt.Errorf("secret key %q does not exist or is empty", resourcesv1alpha1.DataKeyToken)
	}

	currentToken, err := r.FS.ReadFile(path)
	if err != nil && !errors.Is(err, afero.ErrFileNotFound) {
		return reconcile.Result{}, fmt.Errorf("failed reading token file %s: %w", path, err)
	}

	if !bytes.Equal(currentToken, token) {
		log.Info("Access token differs from the one currently stored on the disk, updating it", "path", path)
		if err := r.FS.WriteFile(path, token, 0600); err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to write access token to %s: %w", path, err)
		}
		log.Info("Updated token written to disk")
	}

	log.Info("Token sync completed, requeuing for next sync", "requeueAfter", r.Config.SyncPeriod.Duration)
	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}
