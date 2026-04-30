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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/selfhostedshootexposure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/provider-local/local"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

type actuator struct {
	client client.Client
}

func newActuator(mgr manager.Manager) selfhostedshootexposure.Actuator {
	return &actuator{client: mgr.GetClient()}
}

func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, exposure *extensionsv1alpha1.SelfHostedShootExposure, _ *extensionscontroller.Cluster) ([]corev1.LoadBalancerIngress, error) {
	service := serviceForExposure(exposure)
	if err := a.client.Patch(ctx, service, client.Apply, local.FieldOwner, client.ForceOwnership); err != nil {
		return nil, fmt.Errorf("could not apply Service: %w", err)
	}

	for _, family := range endpointSliceFamilies {
		if err := a.client.Patch(ctx, endpointSliceForExposure(exposure, family), client.Apply, local.FieldOwner, client.ForceOwnership); err != nil {
			return nil, fmt.Errorf("could not apply %s EndpointSlice: %w", family, err)
		}
	}

	// wait for LoadBalancer to be ready
	if err := health.CheckService(service); err != nil {
		return nil, &reconcilerutils.RequeueAfterError{
			RequeueAfter: 5 * time.Second,
			Cause:        fmt.Errorf("LoadBalancer not yet ready"),
		}
	}

	return service.Status.LoadBalancer.Ingress, nil
	log.Info("LoadBalancer ready", "ingress", liveService.Status.LoadBalancer.Ingress)
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

func serviceForExposure(exposure *extensionsv1alpha1.SelfHostedShootExposure) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName(exposure),
			Namespace: exposure.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:           corev1.ServiceTypeLoadBalancer,
			IPFamilyPolicy: ptr.To(corev1.IPFamilyPolicyPreferDualStack),
			Ports: []corev1.ServicePort{{
				Name:       "https",
				Port:       exposure.Spec.Port,
				TargetPort: intstr.FromInt32(exposure.Spec.Port),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}
}

func endpointSliceForExposure(exposure *extensionsv1alpha1.SelfHostedShootExposure, family discoveryv1.AddressType) *discoveryv1.EndpointSlice {
	var endpoints []discoveryv1.Endpoint
	for _, endpoint := range exposure.Spec.Endpoints {
		for _, address := range endpoint.Addresses {
			if address.Type != corev1.NodeInternalIP {
				continue
			}
			isIPv6 := strings.Contains(address.Address, ":")
			if isIPv6 != (family == discoveryv1.AddressTypeIPv6) {
				continue
			}
			endpoints = append(endpoints, discoveryv1.Endpoint{
				Addresses: []string{address.Address},
				NodeName:  ptr.To(endpoint.NodeName),
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			})
		}
	}

	return &discoveryv1.EndpointSlice{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "discovery.k8s.io/v1",
			Kind:       "EndpointSlice",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      endpointSliceName(exposure, family),
			Namespace: exposure.Namespace,
			Labels:    map[string]string{discoveryv1.LabelServiceName: serviceName(exposure)},
		},
		AddressType: family,
		Endpoints:   endpoints,
		Ports: []discoveryv1.EndpointPort{{
			Name:     ptr.To("https"),
			Port:     ptr.To(exposure.Spec.Port),
			Protocol: ptr.To(corev1.ProtocolTCP),
		}},
	}
}

func serviceName(exposure *extensionsv1alpha1.SelfHostedShootExposure) string {
	return exposure.Name + "-exposure"
}

func endpointSliceName(exposure *extensionsv1alpha1.SelfHostedShootExposure, family discoveryv1.AddressType) string {
	return serviceName(exposure) + "-" + strings.ToLower(string(family))
}
