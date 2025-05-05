// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/apiserver"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

const (
	secretNameServerCert = "gardener-apiserver"
	containerName        = "gardener-apiserver"

	port = 8443
)

func (g *gardenerAPIServer) deployment(
	secretCAETCD *corev1.Secret,
	secretETCDClient *corev1.Secret,
	secretGenericTokenKubeconfig *corev1.Secret,
	secretServer *corev1.Secret,
	secretAdmissionKubeconfigs *corev1.Secret,
	secretETCDEncryptionConfiguration *corev1.Secret,
	secretAuditWebhookKubeconfig *corev1.Secret,
	secretWorkloadIdentitySigningKey *corev1.Secret,
	secretVirtualGardenAccess *gardenerutils.AccessSecret,
	configMapAuditPolicy *corev1.ConfigMap,
	configMapAdmissionConfigs *corev1.ConfigMap,
) *appsv1.Deployment {
	args := []string{
		"--authorization-always-allow-paths=/healthz",
		"--cluster-identity=" + g.values.ClusterIdentity,
		"--authentication-kubeconfig=" + gardenerutils.PathGenericKubeconfig,
		"--authorization-kubeconfig=" + gardenerutils.PathGenericKubeconfig,
		"--kubeconfig=" + gardenerutils.PathGenericKubeconfig,
		"--log-level=" + g.values.LogLevel,
		"--log-format=" + g.values.LogFormat,
		fmt.Sprintf("--secure-port=%d", port),
	}

	if g.values.AdminKubeconfigMaxExpiration != nil {
		args = append(args, fmt.Sprintf("--shoot-admin-kubeconfig-max-expiration=%s", g.values.AdminKubeconfigMaxExpiration.Duration))
	}

	if g.values.GoAwayChance != nil {
		args = append(args, fmt.Sprintf("--goaway-chance=%f", *g.values.GoAwayChance))
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName,
			Namespace: g.namespace,
			Labels:    GetLabels(),
		},
		Spec: appsv1.DeploymentSpec{
			MinReadySeconds:      30,
			RevisionHistoryLimit: ptr.To[int32](2),
			Replicas:             g.values.Autoscaling.Replicas,
			Selector:             &metav1.LabelSelector{MatchLabels: GetLabels()},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       ptr.To(intstr.FromString("100%")),
					MaxUnavailable: ptr.To(intstr.FromInt32(0)),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.MergeStringMaps(GetLabels(), map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS:                                                                                                   v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPublicNetworks:                                                                                        v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPrivateNetworks:                                                                                       v1beta1constants.LabelNetworkPolicyAllowed,
						"networking.resources.gardener.cloud/to-" + v1beta1constants.LabelNetworkPolicyWebhookTargets:                                              v1beta1constants.LabelNetworkPolicyAllowed,
						gardenerutils.NetworkPolicyLabel("virtual-garden-"+etcdconstants.ServiceName(v1beta1constants.ETCDRoleMain), etcdconstants.PortEtcdClient): v1beta1constants.LabelNetworkPolicyAllowed,
						gardenerutils.NetworkPolicyLabel("virtual-garden-"+v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port):              v1beta1constants.LabelNetworkPolicyAllowed,
					}),
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: ptr.To(false),
					PriorityClassName:            v1beta1constants.PriorityClassNameGardenSystem500,
					SecurityContext: &corev1.PodSecurityContext{
						// use the nonroot user from a distroless container
						// https://github.com/GoogleContainerTools/distroless/blob/1a8918fcaa7313fd02ae08089a57a701faea999c/base/base.bzl#L8
						RunAsNonRoot: ptr.To(true),
						RunAsUser:    ptr.To[int64](65532),
						RunAsGroup:   ptr.To[int64](65532),
						FSGroup:      ptr.To[int64](65532),
					},
					Containers: []corev1.Container{{
						Name:            containerName,
						Image:           g.values.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Args:            args,
						Ports: []corev1.ContainerPort{{
							Name:          "https",
							ContainerPort: port,
							Protocol:      corev1.ProtocolTCP,
						}},
						Resources: g.values.Autoscaling.APIServerResources,
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/livez",
									Scheme: corev1.URISchemeHTTPS,
									Port:   intstr.FromInt32(port),
								},
							},
							SuccessThreshold:    1,
							FailureThreshold:    3,
							InitialDelaySeconds: 15,
							PeriodSeconds:       10,
							TimeoutSeconds:      15,
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/readyz",
									Scheme: corev1.URISchemeHTTPS,
									Port:   intstr.FromInt32(port),
								},
							},
							SuccessThreshold:    1,
							FailureThreshold:    3,
							InitialDelaySeconds: 15,
							PeriodSeconds:       10,
							TimeoutSeconds:      15,
						},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: ptr.To(false),
						},
					}},
				},
			},
		},
	}

	injectWorkloadIdentitySettings(deployment, g.values.WorkloadIdentityTokenIssuer, secretWorkloadIdentitySigningKey)
	apiserver.InjectDefaultSettings(deployment, "virtual-garden-", g.values.Values, secretCAETCD, secretETCDClient, secretServer)
	apiserver.InjectAuditSettings(deployment, configMapAuditPolicy, secretAuditWebhookKubeconfig, g.values.Audit)
	apiserver.InjectAdmissionSettings(deployment, configMapAdmissionConfigs, secretAdmissionKubeconfigs, g.values.Values)
	apiserver.InjectEncryptionSettings(deployment, secretETCDEncryptionConfiguration)

	utilruntime.Must(gardenerutils.InjectGenericKubeconfig(deployment, secretGenericTokenKubeconfig.Name, secretVirtualGardenAccess.Secret.Name))
	utilruntime.Must(references.InjectAnnotations(deployment))

	return deployment
}

func injectWorkloadIdentitySettings(deployment *appsv1.Deployment, issuer string, secret *corev1.Secret) {
	const (
		mountPath  = "/etc/gardener-apiserver/workload-identity/signing"
		fileName   = "key.pem"
		volumeName = "gardener-apiserver-workload-identity"
	)

	deployment.Spec.Template.Spec.Containers[0].Args = append(
		deployment.Spec.Template.Spec.Containers[0].Args,
		fmt.Sprintf("--workload-identity-token-issuer=%s", issuer),
		fmt.Sprintf("--workload-identity-signing-key-file=%s/%s", mountPath, fileName),
	)

	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      volumeName,
			MountPath: mountPath,
		},
	)

	deployment.Spec.Template.Spec.Volumes = append(
		deployment.Spec.Template.Spec.Volumes,
		corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secret.Name,
					Items: []corev1.KeyToPath{
						{
							Key:  secretsutils.DataKeyRSAPrivateKey,
							Path: fileName,
						},
					},
				},
			},
		},
	)
}
