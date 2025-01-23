// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserverexposure

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
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
	// NamePrefix is the prefix for the service name.
	NamePrefix string
	// TopologyAwareRoutingEnabled indicates whether topology-aware routing is enabled for the kube-apiserver service.
	TopologyAwareRoutingEnabled bool
	// RuntimeKubernetesVersion is the Kubernetes version of the runtime cluster.
	RuntimeKubernetesVersion *semver.Version
}

// serviceValues configure the kube-apiserver service.
// this one is not exposed as not all values should be configured
// from the outside.
type serviceValues struct {
	annotationsFunc             func() map[string]string
	namePrefix                  string
	topologyAwareRoutingEnabled bool
	runtimeKubernetesVersion    *semver.Version
}

// NewService creates a new instance of DeployWaiter for the Service used to expose the kube-apiserver.
// <waiter> is optional and defaulted to github.com/gardener/gardener/pkg/utils/retry.DefaultOps().
func NewService(
	log logr.Logger,
	cl client.Client,
	namespace string,
	values *ServiceValues,
	sniServiceKeyFunc func() client.ObjectKey,
	waiter retry.Ops,
	clusterIPsFunc func(clusterIPs []string),
	ingressFunc func(ingressIP string),
) component.DeployWaiter {
	if waiter == nil {
		waiter = retry.DefaultOps()
	}

	if clusterIPsFunc == nil {
		clusterIPsFunc = func(_ []string) {}
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
		loadBalancerServiceKeyFunc = sniServiceKeyFunc

		internalValues.annotationsFunc = values.AnnotationsFunc
		internalValues.namePrefix = values.NamePrefix
		internalValues.topologyAwareRoutingEnabled = values.TopologyAwareRoutingEnabled
		internalValues.runtimeKubernetesVersion = values.RuntimeKubernetesVersion
	}

	return &service{
		log:                        log,
		client:                     cl,
		namespace:                  namespace,
		values:                     internalValues,
		loadBalancerServiceKeyFunc: loadBalancerServiceKeyFunc,
		waiter:                     waiter,
		clusterIPsFunc:             clusterIPsFunc,
		ingressFunc:                ingressFunc,
	}
}

type service struct {
	log                        logr.Logger
	client                     client.Client
	namespace                  string
	values                     *serviceValues
	loadBalancerServiceKeyFunc func() client.ObjectKey
	waiter                     retry.Ops
	clusterIPsFunc             func(clusterIPs []string)
	ingressFunc                func(ingressIP string)
}

func (s *service) Deploy(ctx context.Context) error {
	obj := s.emptyService()

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, s.client, obj, func() error {
		obj.Annotations = utils.MergeStringMaps(obj.Annotations, s.values.annotationsFunc())
		metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "networking.istio.io/exportTo", "*")

		namespaceSelectors := []metav1.LabelSelector{
			{MatchLabels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress}},
			{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyAccessTargetAPIServer: v1beta1constants.LabelNetworkPolicyAllowed}},
		}

		// For shoot namespaces the kube-apiserver service needs extra labels and annotations to create required network policies
		// which allow a connection from istio-ingress components to kube-apiserver.
		networkPolicyPort := networkingv1.NetworkPolicyPort{Port: ptr.To(intstr.FromInt32(kubeapiserverconstants.Port)), Protocol: ptr.To(corev1.ProtocolTCP)}
		if gardenerutils.IsShootNamespace(obj.Namespace) {
			utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForScrapeTargets(obj, networkPolicyPort))
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias, v1beta1constants.LabelNetworkPolicyShootNamespaceAlias)

			namespaceSelectors = append(namespaceSelectors,
				metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: v1beta1constants.GardenNamespace}},
				metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: v1beta1constants.LabelExposureClassHandlerName, Operator: metav1.LabelSelectorOpExists}}},
				metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension}},
			)
		} else {
			utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForGardenScrapeTargets(obj, networkPolicyPort))
		}

		utilruntime.Must(gardenerutils.InjectNetworkPolicyNamespaceSelectors(obj, namespaceSelectors...))
		gardenerutils.ReconcileTopologyAwareRoutingMetadata(obj, s.values.topologyAwareRoutingEnabled, s.values.runtimeKubernetesVersion)

		obj.Labels = utils.MergeStringMaps(obj.Labels, getLabels())
		obj.Spec.Type = corev1.ServiceTypeClusterIP
		obj.Spec.Selector = getLabels()
		obj.Spec.Ports = kubernetesutils.ReconcileServicePorts(obj.Spec.Ports, []corev1.ServicePort{
			{
				Name:       kubeapiserver.ServicePortName,
				Protocol:   corev1.ProtocolTCP,
				Port:       kubeapiserverconstants.Port,
				TargetPort: intstr.FromInt32(kubeapiserverconstants.Port),
			},
		}, corev1.ServiceTypeClusterIP)
		obj.Spec.IPFamilyPolicy = ptr.To(corev1.IPFamilyPolicyPreferDualStack)

		return nil
	}); err != nil {
		return err
	}

	s.clusterIPsFunc(obj.Spec.ClusterIPs)
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
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: s.values.namePrefix + v1beta1constants.DeploymentNameKubeAPIServer, Namespace: s.namespace}}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer,
	}
}
