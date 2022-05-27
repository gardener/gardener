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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconciler struct {
	client client.Client
	hostIP string
}

func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

var (
	keyIstioIngressGateway = client.ObjectKey{Namespace: "istio-ingress", Name: "istio-ingressgateway"}
	keyNginxIngress        = client.ObjectKey{Namespace: "garden", Name: "nginx-ingress-controller"}
)

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	key := req.NamespacedName
	service := &corev1.Service{}
	if err := r.client.Get(ctx, key, service); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return reconcile.Result{}, nil
	}

	log.Info("Reconciling service")

	if key == keyIstioIngressGateway || key == keyNginxIngress {
		patch := client.MergeFrom(service.DeepCopy())

		for i, servicePort := range service.Spec.Ports {
			switch {
			case key == keyIstioIngressGateway && servicePort.Name == "tcp":
				service.Spec.Ports[i].NodePort = 30443
			case key == keyIstioIngressGateway && servicePort.Name == "proxy":
				service.Spec.Ports[i].NodePort = 31443
			case key == keyNginxIngress && servicePort.Name == "https":
				service.Spec.Ports[i].NodePort = 30448
			}
		}

		if err := r.client.Patch(ctx, service, patch); err != nil {
			return reconcile.Result{}, err
		}
	}

	patch := client.MergeFrom(service.DeepCopy())
	service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: r.hostIP}}
	return reconcile.Result{}, r.client.Status().Patch(ctx, service, patch)
}
