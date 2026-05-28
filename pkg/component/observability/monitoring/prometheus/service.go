// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	istioapiannotation "istio.io/api/annotation"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

func (p *prometheus) service() *corev1.Service {
	var port = servicePorts.Web.TargetPort.IntVal
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.name(),
			Namespace: p.namespace,
			Labels:    p.getLabels(),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"prometheus": p.values.Name},
			Ports:    []corev1.ServicePort{servicePorts.Web},
		},
	}
	if p.values.Ingress != nil {
		service.Annotations = map[string]string{istioapiannotation.NetworkingExportTo.Name: p.values.Ingress.IstioIngressGatewayNamespace}
	}
	if p.values.Cortex != nil {
		service.Spec.Ports = append(service.Spec.Ports, servicePorts.Cortex)
	}

	switch p.values.ClusterType {
	case component.ClusterTypeShoot:
		metav1.SetMetaDataAnnotation(&service.ObjectMeta, resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias, v1beta1constants.LabelNetworkPolicyShootNamespaceAlias)
		utilruntime.Must(gardenerutils.InjectNetworkPolicyNamespaceSelectors(service, metav1.LabelSelector{MatchLabels: map[string]string{
			corev1.LabelMetadataName: v1beta1constants.GardenNamespace,
		}}))

	default:
		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForGardenScrapeTargets(service, networkingv1.NetworkPolicyPort{
			Port:     new(intstr.FromInt32(port)),
			Protocol: new(corev1.ProtocolTCP),
		}))
		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForSeedScrapeTargets(service, networkingv1.NetworkPolicyPort{
			Port:     new(intstr.FromInt32(port)),
			Protocol: new(corev1.ProtocolTCP),
		}))
		utilruntime.Must(gardenerutils.InjectNetworkPolicyNamespaceSelectors(service, metav1.LabelSelector{MatchLabels: map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot,
		}}))
	}

	return service
}
