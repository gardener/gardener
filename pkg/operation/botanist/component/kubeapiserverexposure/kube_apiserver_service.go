// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeapiserverexposure

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kubeAPIServerChartName = "kube-apiserver-service"
)

// ServiceValues configure the kube-apiserver service.
type ServiceValues struct {
	Annotations               map[string]string
	KonnectivityTunnelEnabled bool
	SNIPhase                  component.Phase
}

// NewService creates a new instance of DeployWaiter for a specific DNS entry.
// <waiter> is optional and it's defaulted to github.com/gardener/gardener/pkg/utils/retry.DefaultOps().
func NewService(
	values *ServiceValues,
	serviceKey client.ObjectKey,
	sniServiceKey client.ObjectKey,
	applier kubernetes.ChartApplier,
	chartsRootPath string,
	logger logrus.FieldLogger,
	crclient client.Client,
	waiter retry.Ops,
	clusterIPFunc func(clusterIP string),
	ingressFunc func(ingressIP string),
) component.DeployWaiter {
	var loadBalancerServiceKey client.ObjectKey

	if waiter == nil {
		waiter = retry.DefaultOps()
	}

	if clusterIPFunc == nil {
		clusterIPFunc = func(_ string) {}
	}

	if ingressFunc == nil {
		ingressFunc = func(_ string) {}
	}

	internalValues := &serviceValues{
		Name: serviceKey.Name,
	}

	if values != nil {
		switch values.SNIPhase {
		case component.PhaseEnabled:
			internalValues.ServiceType = corev1.ServiceTypeClusterIP
			internalValues.EnableSNI = true
			internalValues.GardenerManaged = true
			loadBalancerServiceKey = sniServiceKey
		case component.PhaseEnabling:
			// existing traffic must still access the old loadbalancer
			// IP (due to DNS cache).
			internalValues.ServiceType = corev1.ServiceTypeLoadBalancer
			internalValues.EnableSNI = true
			internalValues.GardenerManaged = false
			loadBalancerServiceKey = sniServiceKey
		case component.PhaseDisabling:
			internalValues.ServiceType = corev1.ServiceTypeLoadBalancer
			internalValues.EnableSNI = true
			internalValues.GardenerManaged = true
			loadBalancerServiceKey = serviceKey
		default:
			internalValues.ServiceType = corev1.ServiceTypeLoadBalancer
			internalValues.EnableSNI = false
			internalValues.GardenerManaged = false
			loadBalancerServiceKey = serviceKey
		}

		internalValues.Annotations = values.Annotations
		internalValues.EnableKonnectivityTunnel = values.KonnectivityTunnelEnabled
	}

	return &service{
		ChartApplier:           applier,
		chartPath:              filepath.Join(chartsRootPath, "seed-controlplane", "charts", kubeAPIServerChartName),
		client:                 crclient,
		logger:                 logger,
		values:                 internalValues,
		service:                serviceKey,
		loadBalancerServiceKey: loadBalancerServiceKey,
		waiter:                 waiter,
		clusterIPFunc:          clusterIPFunc,
		ingressFunc:            ingressFunc,
	}
}

func (s *service) Deploy(ctx context.Context) error {
	if err := s.Apply(
		ctx,
		s.chartPath,
		s.service.Namespace,
		kubeAPIServerChartName,
		kubernetes.Values(s.values),
	); err != nil {
		return err
	}

	service := &corev1.Service{}
	if err := s.client.Get(ctx, s.service, service); err != nil {
		return err
	}

	s.clusterIPFunc(service.Spec.ClusterIP)
	return nil
}

func (s *service) Destroy(ctx context.Context) error {
	return client.IgnoreNotFound(s.client.Delete(ctx, s.emptyService()))
}

func (s *service) Wait(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	return s.waiter.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		// this ingress can be either the kube-apiserver's service or istio's IGW loadbalancer.
		loadBalancerIngress, err := kutil.GetLoadBalancerIngress(
			ctx,
			s.client,
			&corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: s.loadBalancerServiceKey.Name, Namespace: s.loadBalancerServiceKey.Namespace,
				},
			},
		)
		if err != nil {
			s.logger.Info("Waiting until the KubeAPI Server ingress LoadBalancer deployed in the Seed cluster is ready...")
			// TODO(AC): This is a quite optimistic check / we should differentiate here
			return retry.MinorError(fmt.Errorf("KubeAPI Server ingress LoadBalancer deployed in the Seed cluster is ready: %v", err))
		}
		s.ingressFunc(loadBalancerIngress)
		return retry.Ok()
	})
}

func (s *service) WaitCleanup(ctx context.Context) error {
	return kutil.WaitUntilResourceDeleted(ctx, s.client, s.emptyService(), 5*time.Second)
}

func (s *service) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: s.service.Name, Namespace: s.service.Namespace}}
}

// serviceValues configure the kube-apiserver service.
// this one is not exposed as not all values should be configured
// from the outside.
type serviceValues struct {
	EnableSNI                bool               `json:"enableSNI,omitempty"`
	EnableKonnectivityTunnel bool               `json:"enableKonnectivityTunnel,omitempty"`
	GardenerManaged          bool               `json:"gardenerManaged,omitempty"`
	Name                     string             `json:"name,omitempty"`
	Annotations              map[string]string  `json:"annotations,omitempty"`
	ServiceType              corev1.ServiceType `json:"serviceType,omitempty"`
}

type service struct {
	values                 *serviceValues
	service                client.ObjectKey
	loadBalancerServiceKey client.ObjectKey
	kubernetes.ChartApplier
	chartPath     string
	logger        logrus.FieldLogger
	client        client.Client
	waiter        retry.Ops
	clusterIPFunc func(clusterIP string)
	ingressFunc   func(ingressIP string)
}
