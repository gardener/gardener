// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardeneradmissioncontroller

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	secretNameServerCert      = "gardener-admission-controller-cert"
	volumeMountPathServerCert = "/etc/gardener-admission-controller/srv"
	volumeMountConfig         = "/etc/gardener-admission-controller/config"
)

func (a admissioncontroller) deployment(secretServerCert, secretGenericTokenKubeconfig, secretVirtualGardenAccess, configMapAdmissionConfig string) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: getObjectMeta(deploymentName, a.namespace),
		Spec: appsv1.DeploymentSpec{
			Replicas: &a.values.ReplicaCount,
			Selector: &metav1.LabelSelector{
				MatchLabels: GetLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.MergeStringMaps(GetLabels(), map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS: v1beta1constants.LabelNetworkPolicyAllowed,
						gardenerutils.NetworkPolicyLabel("virtual-garden-"+v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
					}),
				},
				Spec: corev1.PodSpec{
					PriorityClassName:            v1beta1constants.PriorityClassNameGardenSystem500,
					AutomountServiceAccountToken: pointer.Bool(false),
					Containers: []corev1.Container{
						{
							Name:            deploymentName,
							Image:           a.values.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--config=" + volumeMountConfig + "/" + dataConfigKey,
							},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("200Mi"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt(healthzPort),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 10,
								TimeoutSeconds:      5,
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "gardener-admission-controller-cert",
									MountPath: volumeMountPathServerCert,
									ReadOnly:  true,
								},
								{
									Name:      "gardener-admission-controller-config",
									MountPath: volumeMountConfig,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "gardener-admission-controller-cert",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{SecretName: secretServerCert},
							},
						},
						{
							Name: "gardener-admission-controller-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: configMapAdmissionConfig},
								},
							},
						},
					},
				},
			},
		},
	}

	if a.values.ReplicaCount > 1 {
		deployment.Spec.Template.Spec.Affinity = &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{Key: v1beta1constants.LabelApp, Operator: metav1.LabelSelectorOpIn, Values: []string{v1beta1constants.LabelGardener}},
									{Key: v1beta1constants.LabelRole, Operator: metav1.LabelSelectorOpIn, Values: []string{roleName}},
								},
							},
							TopologyKey: corev1.LabelHostname,
						},
						Weight: 1,
					},
				},
			},
		}
	}

	metav1.SetMetaDataLabel(&deployment.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigType, resourcesv1alpha1.HighAvailabilityConfigTypeServer)

	utilruntime.Must(gardenerutils.InjectGenericKubeconfig(deployment, secretGenericTokenKubeconfig, secretVirtualGardenAccess))
	utilruntime.Must(references.InjectAnnotations(deployment))

	return deployment
}
