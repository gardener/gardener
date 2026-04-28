// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	discoveryv1ac "k8s.io/client-go/applyconfigurations/discovery/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/selfhostedshootexposure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

type actuator struct {
	client client.Client
}

func newActuator(mgr manager.Manager) selfhostedshootexposure.Actuator {
	return &actuator{client: mgr.GetClient()}
}

func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, exposure *extensionsv1alpha1.SelfHostedShootExposure, _ *extensionscontroller.Cluster) ([]corev1.LoadBalancerIngress, error) {
	if err := a.client.Apply(ctx, serviceForExposure(exposure), local.FieldOwner, client.ForceOwnership); err != nil {
		return nil, fmt.Errorf("could not apply Service: %w", err)
	}

	for _, family := range endpointSliceFamilies {
		if err := a.client.Apply(ctx, endpointSliceForExposure(exposure, family), local.FieldOwner, client.ForceOwnership); err != nil {
			return nil, fmt.Errorf("could not apply %s EndpointSlice: %w", family, err)
		}
	}

	liveService := &corev1.Service{}
	if err := a.client.Get(ctx, client.ObjectKey{Name: serviceName(exposure), Namespace: exposure.Namespace}, liveService); err != nil {
		return nil, fmt.Errorf("could not get Service: %w", err)
	}

	if len(liveService.Status.LoadBalancer.Ingress) == 0 {
		log.Info("Waiting for LoadBalancer to get an external address")
		return nil, &reconcilerutils.RequeueAfterError{
			RequeueAfter: 5 * time.Second,
			Cause:        fmt.Errorf("LoadBalancer not yet ready"),
		}
	}

	log.Info("LoadBalancer ready", "ingress", liveService.Status.LoadBalancer.Ingress)
	return liveService.Status.LoadBalancer.Ingress, nil
}

func (a *actuator) Delete(ctx context.Context, _ logr.Logger, exposure *extensionsv1alpha1.SelfHostedShootExposure, _ *extensionscontroller.Cluster) error {
	objects := []client.Object{
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: serviceName(exposure), Namespace: exposure.Namespace}},
	}
	for _, family := range endpointSliceFamilies {
		objects = append(objects, &discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: endpointSliceName(exposure, family), Namespace: exposure.Namespace}})
	}
	for _, obj := range objects {
		if err := client.IgnoreNotFound(a.client.Delete(ctx, obj)); err != nil {
			return fmt.Errorf("could not delete %T %q: %w", obj, client.ObjectKeyFromObject(obj), err)
		}
	}
	return nil
}

func (a *actuator) ForceDelete(ctx context.Context, log logr.Logger, exposure *extensionsv1alpha1.SelfHostedShootExposure, cluster *extensionscontroller.Cluster) error {
	return a.Delete(ctx, log, exposure, cluster)
}

// endpointSliceFamilies are the address families for which an EndpointSlice is created.
// On single-stack clusters the non-matching slice is empty and ignored.
var endpointSliceFamilies = []discoveryv1.AddressType{discoveryv1.AddressTypeIPv4, discoveryv1.AddressTypeIPv6}

func serviceForExposure(exposure *extensionsv1alpha1.SelfHostedShootExposure) *corev1ac.ServiceApplyConfiguration {
	return corev1ac.Service(serviceName(exposure), exposure.Namespace).
		WithSpec(corev1ac.ServiceSpec().
			WithType(corev1.ServiceTypeLoadBalancer).
			WithIPFamilyPolicy(corev1.IPFamilyPolicyPreferDualStack).
			WithPorts(corev1ac.ServicePort().
				WithName("https").
				WithPort(exposure.Spec.Port).
				WithTargetPort(intstr.FromInt32(exposure.Spec.Port)).
				WithProtocol(corev1.ProtocolTCP),
			),
		)
}

func endpointSliceForExposure(exposure *extensionsv1alpha1.SelfHostedShootExposure, family discoveryv1.AddressType) *discoveryv1ac.EndpointSliceApplyConfiguration {
	var endpoints []*discoveryv1ac.EndpointApplyConfiguration
	for _, endpoint := range exposure.Spec.Endpoints {
		for _, address := range endpoint.Addresses {
			if address.Type != corev1.NodeInternalIP {
				continue
			}
			isIPv6 := strings.Contains(address.Address, ":")
			if isIPv6 != (family == discoveryv1.AddressTypeIPv6) {
				continue
			}
			endpoints = append(endpoints, discoveryv1ac.Endpoint().
				WithAddresses(address.Address).
				WithNodeName(endpoint.NodeName).
				WithConditions(discoveryv1ac.EndpointConditions().WithReady(true)),
			)
		}
	}

	return discoveryv1ac.EndpointSlice(endpointSliceName(exposure, family), exposure.Namespace).
		WithLabels(map[string]string{discoveryv1.LabelServiceName: serviceName(exposure)}).
		WithAddressType(family).
		WithEndpoints(endpoints...).
		WithPorts(discoveryv1ac.EndpointPort().
			WithName("https").
			WithPort(exposure.Spec.Port).
			WithProtocol(corev1.ProtocolTCP),
		)
}

func serviceName(exposure *extensionsv1alpha1.SelfHostedShootExposure) string {
	return exposure.Name + "-exposure"
}

func endpointSliceName(exposure *extensionsv1alpha1.SelfHostedShootExposure, family discoveryv1.AddressType) string {
	return serviceName(exposure) + "-" + strings.ToLower(string(family))
}
