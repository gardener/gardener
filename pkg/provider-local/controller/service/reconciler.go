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
	"fmt"
	"math/rand"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

const (
	nodePortIstioIngressGateway int32 = 30443
	nodePortIngress             int32 = 30448
)

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	key := req.NamespacedName
	service := &corev1.Service{}
	if err := r.client.Get(ctx, key, service); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
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
				service.Spec.Ports[i].NodePort = nodePortIstioIngressGateway
			case key == keyNginxIngress && servicePort.Name == "https":
				service.Spec.Ports[i].NodePort = nodePortIngress
			}
		}

		if err := r.client.Patch(ctx, service, patch); err != nil {
			if apierrors.IsInvalid(err) && strings.Contains(err.Error(), "port is already allocated") {
				log.Info("Patching nodePort failed because it is already allocated, enabling auto-remediation")
				return reconcile.Result{Requeue: true}, r.remediateAllocatedNodePorts(ctx, log)
			}
			return reconcile.Result{}, err
		}
	}

	patch := client.MergeFrom(service.DeepCopy())
	service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: r.hostIP}}
	return reconcile.Result{}, r.client.Status().Patch(ctx, service, patch)
}

func (r *reconciler) remediateAllocatedNodePorts(ctx context.Context, log logr.Logger) error {
	serviceList := &corev1.ServiceList{}
	if err := r.client.List(ctx, serviceList); err != nil {
		return err
	}

	for _, service := range serviceList.Items {
		var (
			mustUpdate bool
			patch      = client.StrategicMergeFrom(service.DeepCopy())
		)

		for i, port := range service.Spec.Ports {
			if port.NodePort == nodePortIstioIngressGateway ||
				port.NodePort == nodePortIngress {
				var (
					min, max    = 30000, 32767
					newNodePort = int32(rand.Intn(max-min) + min)
				)

				log.Info("Assigning new nodePort to service which already allocates the nodePort",
					"service", client.ObjectKeyFromObject(&service),
					"newNodePort", newNodePort,
				)

				service.Spec.Ports[i].NodePort = newNodePort
				mustUpdate = true
			}
		}

		if mustUpdate {
			if err := r.client.Patch(ctx, &service, patch); err != nil {
				return err
			}
		}
	}

	return nil
}
