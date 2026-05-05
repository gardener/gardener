// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsselfhostedshootexposurecontroller "github.com/gardener/gardener/extensions/pkg/controller/selfhostedshootexposure"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/provider-local/local"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

const portName = "https"

type actuator struct {
	// runtimeClient uses provider-local's in-cluster config, e.g., for the seed/bootstrap cluster it runs in.
	// It's used to interact with extension objects. By default, it's also used as the provider runtimeClient to interact with
	// infrastructure resources, unless a kubeconfig is specified in the cloudprovider secret.
	runtimeClient client.Client
}

func newActuator(mgr manager.Manager) extensionsselfhostedshootexposurecontroller.Actuator {
	return &actuator{runtimeClient: mgr.GetClient()}
}

func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, exposure *extensionsv1alpha1.SelfHostedShootExposure, cluster *extensionscontroller.Cluster) ([]corev1.LoadBalancerIngress, error) {
	providerClient, err := a.providerClient(ctx, log, exposure)
	if err != nil {
		return nil, err
	}

	service := serviceForExposure(exposure)
	if err := providerClient.Patch(ctx, service, client.Apply, local.FieldOwner, client.ForceOwnership); err != nil {
		return nil, fmt.Errorf("could not patch Service: %w", err)
	}

	for _, family := range endpointSliceFamiliesForCluster(cluster) {
		slice, err := endpointSliceForExposure(exposure, family)
		if err != nil {
			return nil, fmt.Errorf("could not build %s EndpointSlice: %w", family, err)
		}
		if err := providerClient.Patch(ctx, slice, client.Apply, local.FieldOwner, client.ForceOwnership); err != nil {
			return nil, fmt.Errorf("could not patch %s EndpointSlice: %w", family, err)
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
}

func (a *actuator) Delete(ctx context.Context, log logr.Logger, exposure *extensionsv1alpha1.SelfHostedShootExposure, cluster *extensionscontroller.Cluster) error {
	providerClient, err := a.providerClient(ctx, log, exposure)
	if err != nil {
		return err
	}

	// Explicitly delete the Service and wait for it to be gone so the LoadBalancer is deprovisioned before
	// releasing the SelfHostedShootExposure.
	err = providerClient.Delete(ctx, serviceForExposure(exposure))
	if err == nil {
		return &reconcilerutils.RequeueAfterError{
			RequeueAfter: 5 * time.Second,
			Cause:        fmt.Errorf("waiting for Service to be deleted"),
		}
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	// Service is gone; delete the EndpointSlices. They have no owner references in the provider
	// cluster so they won't be garbage collected automatically.
	for _, family := range endpointSliceFamiliesForCluster(cluster) {
		if err := providerClient.Delete(ctx, &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      endpointSliceName(exposure, family),
				Namespace: exposure.Namespace,
			},
		}); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("could not delete %s EndpointSlice: %w", family, err)
		}
	}
	log.Info("Service and EndpointSlices gone, releasing SelfHostedShootExposure")
	return nil
}

func (a *actuator) providerClient(ctx context.Context, log logr.Logger, exposure *extensionsv1alpha1.SelfHostedShootExposure) (client.Client, error) {
	if exposure.Spec.CredentialsRef == nil {
		return nil, fmt.Errorf("credentialsRef is required for the provider-local SelfHostedShootExposure implementation but is not set")
	}
	providerClient, err := local.GetProviderClient(ctx, log, a.runtimeClient, corev1.SecretReference{
		Name:      exposure.Spec.CredentialsRef.Name,
		Namespace: exposure.Spec.CredentialsRef.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("could not create provider client: %w", err)
	}
	return providerClient, nil
}

func (a *actuator) ForceDelete(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.SelfHostedShootExposure, _ *extensionscontroller.Cluster) error {
	return nil
}

func endpointSliceFamiliesForCluster(cluster *extensionscontroller.Cluster) []discoveryv1.AddressType {
	families := make([]discoveryv1.AddressType, 0, len(cluster.Shoot.Spec.Networking.IPFamilies))
	for _, family := range cluster.Shoot.Spec.Networking.IPFamilies {
		switch family {
		case gardencorev1beta1.IPFamilyIPv4:
			families = append(families, discoveryv1.AddressTypeIPv4)
		case gardencorev1beta1.IPFamilyIPv6:
			families = append(families, discoveryv1.AddressTypeIPv6)
		}
	}
	return families
}

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
				Name:       portName,
				Port:       exposure.Spec.Port,
				TargetPort: intstr.FromInt32(exposure.Spec.Port),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}
}

func endpointSliceForExposure(exposure *extensionsv1alpha1.SelfHostedShootExposure, family discoveryv1.AddressType) (*discoveryv1.EndpointSlice, error) {
	var endpoints []discoveryv1.Endpoint
	for _, endpoint := range exposure.Spec.Endpoints {
		for _, address := range endpoint.Addresses {
			// Use InternalIPs as the Service backends since they are directly reachable
			// within the cluster network.
			if address.Type != corev1.NodeInternalIP {
				continue
			}
			ip, err := netip.ParseAddr(address.Address)
			if err != nil {
				return nil, fmt.Errorf("could not parse address %q for endpoint %q: %w", address.Address, endpoint.NodeName, err)
			}
			// Using ip.Is4() here excludes IPv4-mapped IPv6 addresses (like ::ffff:192.0.2.1)
			if ip.Is4() != (family == discoveryv1.AddressTypeIPv4) {
				continue
			}
			endpoints = append(endpoints, discoveryv1.Endpoint{
				Addresses: []string{address.Address},
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			})
		}
	}
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no %s endpoints found for exposure %q", family, exposure.Name)
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
			Name:     ptr.To(portName),
			Port:     ptr.To(exposure.Spec.Port),
			Protocol: ptr.To(corev1.ProtocolTCP),
		}},
	}, nil
}

func serviceName(exposure *extensionsv1alpha1.SelfHostedShootExposure) string {
	return "exposure-" + exposure.Name
}

func endpointSliceName(exposure *extensionsv1alpha1.SelfHostedShootExposure, family discoveryv1.AddressType) string {
	return serviceName(exposure) + "-" + strings.ToLower(string(family))
}
