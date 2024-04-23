// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardenercustommetrics

import (
	"fmt"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

const (
	deploymentName    = "gardener-custom-metrics"
	gcmxContainerName = "gardener-custom-metrics"
)

// getLabels returns a set of labels, common to GCMx resources.
func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:   "gardener-custom-metrics",
		v1beta1constants.GardenRole: "gardener-custom-metrics",
	}
}

func (gcmx *gardenerCustomMetrics) deployment(serverSecretName string) *appsv1.Deployment {
	const tlsSecretMountPath = "/var/run/secrets/gardener.cloud/tls"

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: gcmx.namespace,
			Labels: utils.MergeStringMaps(getLabels(), map[string]string{
				resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeServer,
			}),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             ptr.To[int32](1),
			RevisionHistoryLimit: ptr.To[int32](2),
			Selector: &metav1.LabelSelector{
				MatchLabels: getLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.MergeStringMaps(getLabels(), map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS:                                   v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:                      v1beta1constants.LabelNetworkPolicyAllowed,
						"networking.resources.gardener.cloud/to-all-shoots-kube-apiserver-tcp-443": v1beta1constants.LabelNetworkPolicyAllowed,
					}),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Args: []string{
								fmt.Sprintf("--secure-port=%d", servingPort),
								"--tls-cert-file=" + filepath.Join(tlsSecretMountPath, secretsutils.DataKeyCertificate),
								"--tls-private-key-file=" + filepath.Join(tlsSecretMountPath, secretsutils.DataKeyPrivateKey),
								"--leader-election=true",
								"--namespace=" + gcmx.namespace,
								"--access-ip=$(POD_IP)",
								fmt.Sprintf("--access-port=%d", servingPort),
								"--log-level=74",
							},
							Env: []corev1.EnvVar{
								{
									Name: "POD_IP",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "status.podIP",
										},
									},
								},
								{
									Name: "LEADER_ELECTION_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							Image:           gcmx.values.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Name:            gcmxContainerName,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: servingPort,
									Name:          "metrics-server",
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("80m"),
									corev1.ResourceMemory: resource.MustParse("200Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: tlsSecretMountPath,
									Name:      "gardener-custom-metrics-tls",
									ReadOnly:  true,
								},
							},
						},
					},
					PriorityClassName:  v1beta1constants.PriorityClassNameSeedSystem700,
					ServiceAccountName: serviceAccountName,
					Volumes: []corev1.Volume{
						{
							Name: "gardener-custom-metrics-tls",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: serverSecretName,
								},
							},
						},
					},
				},
			},
		},
	}
}
