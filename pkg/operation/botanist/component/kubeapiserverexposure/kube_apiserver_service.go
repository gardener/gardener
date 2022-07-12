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
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceValues configure the kube-apiserver service.
type ServiceValues struct {
	Annotations map[string]string
	SNIPhase    component.Phase
}

// serviceValues configure the kube-apiserver service.
// this one is not exposed as not all values should be configured
// from the outside.
type serviceValues struct {
	annotations     map[string]string
	serviceType     corev1.ServiceType
	enableSNI       bool
	gardenerManaged bool
}

// NewService creates a new instance of DeployWaiter for the Service used to expose the kube-apiserver.
// <waiter> is optional and it's defaulted to github.com/gardener/gardener/pkg/utils/retry.DefaultOps().
func NewService(
	log logr.Logger,
	crclient client.Client,
	values *ServiceValues,
	serviceKey client.ObjectKey,
	sniServiceKey client.ObjectKey,
	waiter retry.Ops,
	clusterIPFunc func(clusterIP string),
	ingressFunc func(ingressIP string),
) component.DeployWaiter {
	if waiter == nil {
		waiter = retry.DefaultOps()
	}

	if clusterIPFunc == nil {
		clusterIPFunc = func(_ string) {}
	}

	if ingressFunc == nil {
		ingressFunc = func(_ string) {}
	}

	var (
		internalValues         = &serviceValues{}
		loadBalancerServiceKey client.ObjectKey
	)

	if values != nil {
		switch values.SNIPhase {
		case component.PhaseEnabled:
			internalValues.serviceType = corev1.ServiceTypeClusterIP
			internalValues.enableSNI = true
			internalValues.gardenerManaged = true
			loadBalancerServiceKey = sniServiceKey
		case component.PhaseEnabling:
			// existing traffic must still access the old loadbalancer
			// IP (due to DNS cache).
			internalValues.serviceType = corev1.ServiceTypeLoadBalancer
			internalValues.enableSNI = true
			internalValues.gardenerManaged = false
			loadBalancerServiceKey = sniServiceKey
		case component.PhaseDisabling:
			internalValues.serviceType = corev1.ServiceTypeLoadBalancer
			internalValues.enableSNI = true
			internalValues.gardenerManaged = true
			loadBalancerServiceKey = serviceKey
		default:
			internalValues.serviceType = corev1.ServiceTypeLoadBalancer
			internalValues.enableSNI = false
			internalValues.gardenerManaged = false
			loadBalancerServiceKey = serviceKey
		}

		internalValues.annotations = values.Annotations
	}

	return &service{
		log:                    log,
		client:                 crclient,
		values:                 internalValues,
		serviceKey:             serviceKey,
		loadBalancerServiceKey: loadBalancerServiceKey,
		waiter:                 waiter,
		clusterIPFunc:          clusterIPFunc,
		ingressFunc:            ingressFunc,
	}
}

type service struct {
	log                    logr.Logger
	client                 client.Client
	values                 *serviceValues
	serviceKey             client.ObjectKey
	loadBalancerServiceKey client.ObjectKey
	waiter                 retry.Ops
	clusterIPFunc          func(clusterIP string)
	ingressFunc            func(ingressIP string)
}

func (s *service) Deploy(ctx context.Context) error {
	obj := s.emptyService()

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, s.client, obj, func() error {
		obj.Annotations = s.values.annotations
		if s.values.enableSNI {
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "networking.istio.io/exportTo", "*")
		}

		obj.Labels = getLabels()
		if s.values.gardenerManaged {
			metav1.SetMetaDataLabel(&obj.ObjectMeta, v1beta1constants.LabelAPIServerExposure, v1beta1constants.LabelAPIServerExposureGardenerManaged)
		}

		obj.Spec.Type = s.values.serviceType
		obj.Spec.Selector = getLabels()
		obj.Spec.Ports = kutil.ReconcileServicePorts(obj.Spec.Ports, []corev1.ServicePort{
			{
				Name:       kubeapiserver.ServicePortName,
				Protocol:   corev1.ProtocolTCP,
				Port:       kubeapiserver.Port,
				TargetPort: intstr.FromInt(kubeapiserver.Port),
			},
		}, s.values.serviceType)

		return nil
	}); err != nil {
		return err
	}

	s.clusterIPFunc(obj.Spec.ClusterIP)
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
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.loadBalancerServiceKey.Name,
				Namespace: s.loadBalancerServiceKey.Namespace,
			},
		}

		loadBalancerIngress, err := kutil.GetLoadBalancerIngress(ctx, s.client, svc)
		if err != nil {
			s.log.Info("Waiting until the kube-apiserver ingress LoadBalancer deployed in the Seed cluster is ready", "service", client.ObjectKeyFromObject(svc))
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
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: s.serviceKey.Name, Namespace: s.serviceKey.Namespace}}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer,
	}
}
