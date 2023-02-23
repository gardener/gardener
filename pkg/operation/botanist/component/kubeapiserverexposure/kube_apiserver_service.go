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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second
	// DefaultTimeout is the default timeout and defines how long Gardener should wait
	// for a successful reconciliation of the service resource.
	DefaultTimeout = 10 * time.Minute
)

// ServiceValues configure the kube-apiserver service.
type ServiceValues struct {
	// AnnotationsFunc is a function that returns annotations that should be added to the service.
	AnnotationsFunc func() map[string]string
	// IsShootService declares the service as being used for shoot clusters.
	IsShootService bool
	// SNIPhase is the current status of the SNI configuration.
	SNIPhase component.Phase
}

// serviceValues configure the kube-apiserver service.
// this one is not exposed as not all values should be configured
// from the outside.
type serviceValues struct {
	annotationsFunc func() map[string]string
	isShootService  bool
	serviceType     corev1.ServiceType
	enableSNI       bool
	gardenerManaged bool
}

// NewService creates a new instance of DeployWaiter for the Service used to expose the kube-apiserver.
// <waiter> is optional and defaulted to github.com/gardener/gardener/pkg/utils/retry.DefaultOps().
func NewService(
	log logr.Logger,
	cl client.Client,
	values *ServiceValues,
	serviceKeyFunc func() client.ObjectKey,
	sniServiceKeyFunc func() client.ObjectKey,
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
		internalValues = &serviceValues{
			annotationsFunc: func() map[string]string { return map[string]string{} },
		}
		loadBalancerServiceKeyFunc func() client.ObjectKey
	)

	if values != nil {
		switch values.SNIPhase {
		case component.PhaseEnabled:
			internalValues.serviceType = corev1.ServiceTypeClusterIP
			internalValues.enableSNI = true
			internalValues.gardenerManaged = true
			loadBalancerServiceKeyFunc = sniServiceKeyFunc
		case component.PhaseEnabling:
			// existing traffic must still access the old loadbalancer
			// IP (due to DNS cache).
			internalValues.serviceType = corev1.ServiceTypeLoadBalancer
			internalValues.enableSNI = true
			internalValues.gardenerManaged = false
			loadBalancerServiceKeyFunc = sniServiceKeyFunc
		case component.PhaseDisabling:
			internalValues.serviceType = corev1.ServiceTypeLoadBalancer
			internalValues.enableSNI = true
			internalValues.gardenerManaged = true
			loadBalancerServiceKeyFunc = serviceKeyFunc
		default:
			internalValues.serviceType = corev1.ServiceTypeLoadBalancer
			internalValues.enableSNI = false
			internalValues.gardenerManaged = false
			loadBalancerServiceKeyFunc = serviceKeyFunc
		}

		internalValues.annotationsFunc = values.AnnotationsFunc
		internalValues.isShootService = values.IsShootService
	}

	return &service{
		log:                        log,
		client:                     cl,
		values:                     internalValues,
		serviceKeyFunc:             serviceKeyFunc,
		loadBalancerServiceKeyFunc: loadBalancerServiceKeyFunc,
		waiter:                     waiter,
		clusterIPFunc:              clusterIPFunc,
		ingressFunc:                ingressFunc,
	}
}

type service struct {
	log                        logr.Logger
	client                     client.Client
	values                     *serviceValues
	serviceKeyFunc             func() client.ObjectKey
	loadBalancerServiceKeyFunc func() client.ObjectKey
	waiter                     retry.Ops
	clusterIPFunc              func(clusterIP string)
	ingressFunc                func(ingressIP string)
}

func (s *service) Deploy(ctx context.Context) error {
	obj := s.emptyService()

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, s.client, obj, func() error {
		obj.Annotations = utils.MergeStringMaps(obj.Annotations, s.values.annotationsFunc())
		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForScrapeTargets(obj, networkingv1.NetworkPolicyPort{Port: utils.IntStrPtrFromInt(kubeapiserverconstants.Port), Protocol: utils.ProtocolPtr(corev1.ProtocolTCP)}))
		utilruntime.Must(gardenerutils.InjectNetworkPolicyNamespaceSelectors(obj, metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress}}))
		// TODO(rfranzke): Drop this annotation once the APIServerSNI feature gate is dropped (then API servers are only
		//  exposed indirectly via Istio) and the NetworkPolicy controller in gardener-resource-manager is enabled for
		//  all relevant namespaces in the seed cluster.
		metav1.SetMetaDataAnnotation(&obj.ObjectMeta, resourcesv1alpha1.NetworkingFromWorldToPorts, fmt.Sprintf(`[{"protocol":"TCP","port":%d}]`, kubeapiserverconstants.Port))

		if s.values.enableSNI {
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "networking.istio.io/exportTo", "*")
		}
		if s.values.isShootService {
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias, v1beta1constants.LabelNetworkPolicyShootNamespaceAlias)
			utilruntime.Must(gardenerutils.InjectNetworkPolicyNamespaceSelectors(obj,
				metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress}},
				metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExposureClassHandler}}))
		}

		obj.Labels = getLabels()
		if s.values.gardenerManaged {
			metav1.SetMetaDataLabel(&obj.ObjectMeta, v1beta1constants.LabelAPIServerExposure, v1beta1constants.LabelAPIServerExposureGardenerManaged)
		}

		obj.Spec.Type = s.values.serviceType
		obj.Spec.Selector = getLabels()
		obj.Spec.Ports = kubernetesutils.ReconcileServicePorts(obj.Spec.Ports, []corev1.ServicePort{
			{
				Name:       kubeapiserver.ServicePortName,
				Protocol:   corev1.ProtocolTCP,
				Port:       kubeapiserverconstants.Port,
				TargetPort: intstr.FromInt(kubeapiserverconstants.Port),
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
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	return s.waiter.Until(ctx, DefaultInterval, func(ctx context.Context) (done bool, err error) {
		// this ingress can be either the kube-apiserver's service or istio's IGW loadbalancer.
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.loadBalancerServiceKeyFunc().Name,
				Namespace: s.loadBalancerServiceKeyFunc().Namespace,
			},
		}

		loadBalancerIngress, err := kubernetesutils.GetLoadBalancerIngress(ctx, s.client, svc)
		if err != nil {
			s.log.Info("Waiting until the kube-apiserver ingress LoadBalancer deployed in the Seed cluster is ready", "service", client.ObjectKeyFromObject(svc))
			return retry.MinorError(fmt.Errorf("KubeAPI Server ingress LoadBalancer deployed in the Seed cluster is ready: %v", err))
		}
		s.ingressFunc(loadBalancerIngress)

		return retry.Ok()
	})
}

func (s *service) WaitCleanup(ctx context.Context) error {
	return kubernetesutils.WaitUntilResourceDeleted(ctx, s.client, s.emptyService(), 2*time.Second)
}

func (s *service) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: s.serviceKeyFunc().Name, Namespace: s.serviceKeyFunc().Namespace}}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer,
	}
}
