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

package dnsprovider

import (
	"context"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconciler struct {
	logger logr.Logger
	client client.Client
}

// NewReconciler creates a new reconcile.Reconciler that reconciles DNSProviders.
func NewReconciler() reconcile.Reconciler {
	return &reconciler{
		logger: log.Log.WithName(ControllerName),
	}
}

func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	dnsProvider := &dnsv1alpha1.DNSProvider{}
	if err := r.client.Get(ctx, request.NamespacedName, dnsProvider); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	patch := client.MergeFrom(dnsProvider.DeepCopy())
	dnsProvider.Status.ObservedGeneration = dnsProvider.Generation
	dnsProvider.Status.State = dnsv1alpha1.STATE_READY

	return ctrl.Result{}, r.client.Status().Patch(ctx, dnsProvider, patch)
}
