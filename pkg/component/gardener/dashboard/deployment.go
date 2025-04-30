// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

const (
	dataKeySessionSecret         = "sessionSecret"
	dataKeySessionSecretPrevious = "sessionSecretPrevious"

	portNameServer  = "http"
	portNameMetrics = "metrics"

	portServer  = 8080
	portMetrics = 9050

	readinessProbePeriodSeconds = 10

	volumeMountPathConfig      = "/etc/gardener-dashboard/config"
	volumeMountPathLoginConfig = "/app/public/" + dataKeyLoginConfig
	volumeMountPathAssets      = "/app/public/static/assets"

	volumeSubPathSession           = "sessionSecret"
	volumeMountPathSession         = "/etc/gardener-dashboard/secrets/session/sessionSecret"
	volumeSubPathSessionPrevious   = "sessionSecretPrevious"
	volumeMountPathSessionPrevious = "/etc/gardener-dashboard/secrets/session/sessionSecretPrevious"
	volumeMountPathOIDC            = "/etc/gardener-dashboard/secrets/oidc"
	volumeMountPathGithub          = "/etc/gardener-dashboard/secrets/github"

	volumeNameConfig          = "gardener-dashboard-config"
	volumeNameLoginConfig     = "gardener-dashboard-login-config"
	volumeNameConfigAssets    = "gardener-dashboard-assets"
	volumeNameSession         = "gardener-dashboard-sessionsecret"
	volumeNameSessionPrevious = "gardener-dashboard-sessionsecret-previous"
	volumeNameOIDC            = "gardener-dashboard-oidc"
	volumeNameGithub          = "gardener-dashboard-github"
)

func (g *gardenerDashboard) deployment(
	ctx context.Context,
	secretNameGenericTokenKubeconfig string,
	secretNameVirtualGardenAccess string,
	secretNameSession string,
	secretSessionPrevious *corev1.Secret,
	configMapName string,
) (
	*appsv1.Deployment,
	error,
) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: g.namespace,
			Labels: utils.MergeStringMaps(GetLabels(), map[string]string{
				resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeServer,
			}),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             ptr.To[int32](1),
			RevisionHistoryLimit: ptr.To[int32](2),
			Selector: &metav1.LabelSelector{
				MatchLabels: GetLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.MergeStringMaps(GetLabels(), map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS:             v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPublicNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
						gardenerutils.NetworkPolicyLabel("virtual-garden-"+v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
					}),
				},
				Spec: corev1.PodSpec{
					PriorityClassName:            v1beta1constants.PriorityClassNameGardenSystem200,
					AutomountServiceAccountToken: ptr.To(false),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr.To(true),
						RunAsUser:    ptr.To[int64](65532),
						RunAsGroup:   ptr.To[int64](65532),
						FSGroup:      ptr.To[int64](65532),
					},
					Containers: []corev1.Container{
						{
							Name:            deploymentName,
							Image:           g.values.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--optimize-for-size",
								"server.mjs",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "GARDENER_CONFIG",
									Value: volumeMountPathConfig + "/" + dataKeyConfig,
								},
								{
									Name:  "KUBECONFIG",
									Value: gardenerutils.PathGenericKubeconfig,
								},
								{
									Name:  "METRICS_PORT",
									Value: strconv.Itoa(portMetrics),
								},
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.name",
										},
									},
								},
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.namespace",
										},
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          portNameServer,
									ContainerPort: portServer,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          portNameMetrics,
									ContainerPort: portMetrics,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromString(portNameServer),
									},
								},
								InitialDelaySeconds: 15,
								TimeoutSeconds:      5,
								FailureThreshold:    6,
								SuccessThreshold:    1,
								PeriodSeconds:       20,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromString(portNameServer),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 5,
								TimeoutSeconds:      5,
								FailureThreshold:    6,
								SuccessThreshold:    1,
								PeriodSeconds:       readinessProbePeriodSeconds,
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      volumeNameSession,
									MountPath: volumeMountPathSession,
									SubPath:   volumeSubPathSession,
								},
								{
									Name:      volumeNameConfig,
									MountPath: volumeMountPathConfig,
								},
								{
									Name:      volumeNameLoginConfig,
									MountPath: volumeMountPathLoginConfig,
									SubPath:   dataKeyLoginConfig,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: volumeNameSession,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretNameSession,
									DefaultMode: ptr.To[int32](0640),
									Items: []corev1.KeyToPath{{
										Key:  secretsutils.DataKeyPassword,
										Path: dataKeySessionSecret,
									}},
								},
							},
						},
						{
							Name: volumeNameConfig,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
									Items: []corev1.KeyToPath{{
										Key:  dataKeyConfig,
										Path: dataKeyConfig,
									}},
								},
							},
						},
						{
							Name: volumeNameLoginConfig,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
									Items: []corev1.KeyToPath{{
										Key:  dataKeyLoginConfig,
										Path: dataKeyLoginConfig,
									}},
								},
							},
						},
					},
				},
			},
		},
	}

	if secretSessionPrevious != nil {
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeNameSessionPrevious,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  secretSessionPrevious.Name,
					DefaultMode: ptr.To[int32](0640),
					Items: []corev1.KeyToPath{{
						Key:  secretsutils.DataKeyPassword,
						Path: dataKeySessionSecretPrevious,
					}},
				},
			},
		})
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volumeNameSessionPrevious,
			MountPath: volumeMountPathSessionPrevious,
			SubPath:   volumeSubPathSessionPrevious,
		})
	}

	if g.values.OIDC != nil {
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeNameOIDC,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  g.values.OIDC.SecretRef.Name,
					DefaultMode: ptr.To[int32](0640),
				},
			},
		})
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volumeNameOIDC,
			MountPath: volumeMountPathOIDC,
		})
	}

	if g.values.GitHub != nil {
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeNameGithub,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  g.values.GitHub.SecretRef.Name,
					DefaultMode: ptr.To[int32](0640),
				},
			},
		})
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volumeNameGithub,
			MountPath: volumeMountPathGithub,
		})
	}

	if g.values.AssetsConfigMapName != nil {
		configMapAssets := &corev1.ConfigMap{}
		if err := g.client.Get(ctx, client.ObjectKey{Name: *g.values.AssetsConfigMapName, Namespace: g.namespace}, configMapAssets); err != nil {
			return nil, fmt.Errorf("failed reading assets ConfigMap %s for adding checksum annotation: %w", *g.values.AssetsConfigMapName, err)
		}
		metav1.SetMetaDataAnnotation(&deployment.Spec.Template.ObjectMeta, "checksum-configmap-"+configMapAssets.Name, utils.ComputeSecretChecksum(configMapAssets.BinaryData))

		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name:         volumeNameConfigAssets,
			VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: configMapAssets.Name}}},
		})
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volumeNameConfigAssets,
			MountPath: volumeMountPathAssets,
		})
	}

	utilruntime.Must(gardenerutils.InjectGenericKubeconfig(deployment, secretNameGenericTokenKubeconfig, secretNameVirtualGardenAccess))
	utilruntime.Must(references.InjectAnnotations(deployment))

	return deployment, nil
}
