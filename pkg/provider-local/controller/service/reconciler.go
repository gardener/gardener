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

package service

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconciler struct {
	logger logr.Logger
	client client.Client
	hostIP string
}

// NewReconciler creates a new reconcile.Reconciler that reconciles Services.
func NewReconciler(hostIP string) reconcile.Reconciler {
	return &reconciler{
		logger: log.Log.WithName(ControllerName),
		hostIP: hostIP,
	}
}

func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	service := &corev1.Service{}
	if err := r.client.Get(ctx, request.NamespacedName, service); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return reconcile.Result{}, nil
	}

	if service.Name == "istio-ingressgateway" && service.Namespace == "istio-ingress" {
		patch := client.MergeFrom(service.DeepCopy())

		for i, servicePort := range service.Spec.Ports {
			if servicePort.Name == "tcp" {
				service.Spec.Ports[i].NodePort = 30443
				break
			}
		}

		if err := r.client.Patch(ctx, service, patch); err != nil {
			return reconcile.Result{}, nil
		}
	}

	patch := client.MergeFrom(service.DeepCopy())
	service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: r.hostIP}}
	return reconcile.Result{}, r.client.Status().Patch(ctx, service, patch)
}
