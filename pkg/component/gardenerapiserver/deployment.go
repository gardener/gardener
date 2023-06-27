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

package gardenerapiserver

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/apiserver"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/constants"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	secretNameServer = "gardener-apiserver"
	containerName    = "gardener-apiserver"

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
	secretVirtualGardenAccess *gardenerutils.AccessSecret,
	configMapAuditPolicy *corev1.ConfigMap,
	configMapAdmissionConfigs *corev1.ConfigMap,
) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName,
			Namespace: g.namespace,
			Labels: utils.MergeStringMaps(GetLabels(), map[string]string{
				resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeServer,
			}),
		},
		Spec: appsv1.DeploymentSpec{
			MinReadySeconds:      30,
			RevisionHistoryLimit: pointer.Int32(2),
			Replicas:             g.values.Autoscaling.Replicas,
			Selector:             &metav1.LabelSelector{MatchLabels: GetLabels()},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       utils.IntStrPtrFromInt(1),
					MaxUnavailable: utils.IntStrPtrFromInt(0),
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
					AutomountServiceAccountToken: pointer.Bool(false),
					PriorityClassName:            v1beta1constants.PriorityClassNameGardenSystem500,
					Containers: []corev1.Container{{
						Name:            containerName,
						Image:           g.values.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Args: []string{
							"--authorization-always-allow-paths=/healthz",
							"--cluster-identity=" + g.values.ClusterIdentity,
							"--authentication-kubeconfig=" + gardenerutils.PathGenericKubeconfig,
							"--authorization-kubeconfig=" + gardenerutils.PathGenericKubeconfig,
							"--kubeconfig=" + gardenerutils.PathGenericKubeconfig,
							"--log-level=" + g.values.LogLevel,
							"--log-format=" + g.values.LogFormat,
							fmt.Sprintf("--secure-port=%d", port),
						},
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
									Port:   intstr.FromInt(port),
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
									Port:   intstr.FromInt(port),
								},
							},
							SuccessThreshold:    1,
							FailureThreshold:    3,
							InitialDelaySeconds: 15,
							PeriodSeconds:       10,
							TimeoutSeconds:      15,
						},
					}},
				},
			},
		},
	}

	apiserver.InjectDefaultSettings(deployment, "virtual-garden-", g.values.Values, nil, secretCAETCD, secretETCDClient, secretServer)
	apiserver.InjectAuditSettings(deployment, configMapAuditPolicy, secretAuditWebhookKubeconfig, g.values.Audit)
	apiserver.InjectAdmissionSettings(deployment, configMapAdmissionConfigs, secretAdmissionKubeconfigs, g.values.Values)
	apiserver.InjectEncryptionSettings(deployment, secretETCDEncryptionConfiguration)

	utilruntime.Must(gardenerutils.InjectGenericKubeconfig(deployment, secretGenericTokenKubeconfig.Name, secretVirtualGardenAccess.Secret.Name))
	utilruntime.Must(references.InjectAnnotations(deployment))
	return deployment
}
