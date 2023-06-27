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
	"fmt"
	"time"

	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/controllerutils"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

// Reconciler fetches the shoot access token for gardener-node-agent and writes the token to disk.
// If token changes it will restart itself.
type Reconciler struct {
	Client     client.Client
	Config     *nodeagentv1alpha1.NodeAgentConfiguration
	SyncPeriod time.Duration
	Fs         afero.Fs
}

// Reconcile fetches the shoot access token for gardener-node-agent and writes the token to disk.
// If token changes it will restart itself.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, request.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	token, ok := secret.Data[nodeagentv1alpha1.NodeAgentTokenSecretKey]
	if !ok {
		return reconcile.Result{}, fmt.Errorf("no token found in secret")
	}

	// only update token file if content changed
	currentToken, err := afero.ReadFile(r.Fs, nodeagentv1alpha1.NodeAgentTokenFilePath)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error reading token file %s: %w", nodeagentv1alpha1.NodeAgentTokenFilePath, err)
	}

	if !bytes.Equal(token, currentToken) {
		log.Info("Updating token in file", "file", nodeagentv1alpha1.NodeAgentTokenFilePath)
		if err := afero.WriteFile(r.Fs, nodeagentv1alpha1.NodeAgentTokenFilePath, token, 0600); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed writing token: %w", err)
		}
	}

	log.V(1).Info("Requeuing", "requeueAfter", r.SyncPeriod)
	return reconcile.Result{RequeueAfter: r.SyncPeriod}, nil
}
