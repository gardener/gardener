// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0package x509certificateexporter

package x509certificateexporter

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

func (x *x509CertificateExporter) service(resName string, selector labels.Set) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resName,
			Namespace: x.namespace,
			Labels:    selector,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports: []corev1.ServicePort{
				{
					Name:       portName,
					Port:       port,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(port),
				},
			},
			Type: corev1.ServiceType("ClusterIP"),
		},
	}

	utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForGardenScrapeTargets(service, networkingv1.NetworkPolicyPort{
		Port:     ptr.To(intstr.FromInt(port)),
		Protocol: ptr.To(corev1.ProtocolTCP),
	}))
	return service
}

func (x *x509CertificateExporter) getGenericLabels(source string) map[string]string {
	return map[string]string{
		v1beta1constants.LabelRole: labelComponent,
		certificateSourceLabelName: source,
	}
}

func (x *x509CertificateExporter) defaultPodSpec(sa *corev1.ServiceAccount) corev1.PodSpec {
	return corev1.PodSpec{
		ServiceAccountName: sa.Name,
		SecurityContext: &corev1.PodSecurityContext{
			RunAsGroup: ptr.To(int64(1000)),
			RunAsUser:  ptr.To(int64(1000)),
		},
		Containers: []corev1.Container{
			{
				Name:            containerName,
				Image:           x.values.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Ports: []corev1.ContainerPort{
					{
						Name:          portName,
						ContainerPort: port,
					},
				},
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
					ReadOnlyRootFilesystem: ptr.To(true),
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("20m"),
						corev1.ResourceMemory: resource.MustParse("20Mi"),
					},
				},
			},
		},
		PriorityClassName: x.values.PriorityClassName,
		RestartPolicy:     corev1.RestartPolicyAlways,
	}
}
