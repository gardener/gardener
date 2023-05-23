// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nodeexporter

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
	ManagedResourceName = "shoot-core-node-exporter"
	// labelValue is the value of a label used for the identification of node-exporter pods.
	labelValue = "node-exporter"

	labelKeyComponent = "component"
	serviceName       = "node-exporter"
	daemonsetName     = "node-exporter"
	containerName     = "node-exporter"

	portNameMetrics           = "metrics"
	portMetrics         int32 = 16909
	volumeNameHost            = "host"
	volumeMountPathHost       = "/host"
)

// Interface contains functions for a node-exporter deployer.
type Interface interface {
	component.DeployWaiter
}

// Values is a set of configuration values for the node-exporter component.
type Values struct {
	// Image is the container image used for node-exporter.
	Image string
	// VPAEnabled marks whether VerticalPodAutoscaler is enabled for the shoot.
	VPAEnabled bool
	// PSPDisabled marks whether the PodSecurityPolicy admission plugin is disabled.
	PSPDisabled bool
}

// New creates a new instance of DeployWaiter for node-exporter.
func New(
	client client.Client,
	namespace string,
	values Values,
) Interface {
	return &nodeExporter{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type nodeExporter struct {
	client    client.Client
	namespace string
	values    Values
}

func (n *nodeExporter) Deploy(ctx context.Context) error {
	data, err := n.computeResourcesData()
	if err != nil {
		return err
	}
	return managedresources.CreateForShoot(ctx, n.client, n.namespace, ManagedResourceName, managedresources.LabelValueGardener, false, data)
}

func (n *nodeExporter) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, n.client, n.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (n *nodeExporter) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, n.client, n.namespace, ManagedResourceName)
}

func (n *nodeExporter) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, n.client, n.namespace, ManagedResourceName)
}

func (n *nodeExporter) computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-exporter",
				Namespace: metav1.NamespaceSystem,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: metav1.NamespaceSystem,
				Labels:    getLabels(),
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: corev1.ClusterIPNone,
				Ports: []corev1.ServicePort{
					{
						Name:     portNameMetrics,
						Port:     portMetrics,
						Protocol: corev1.ProtocolTCP,
					},
				},
				Selector: getLabels(),
			},
		}

		daemonSet = &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      daemonsetName,
				Namespace: metav1.NamespaceSystem,
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					v1beta1constants.GardenRole:     v1beta1constants.GardenRoleMonitoring,
					managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
				}),
			},
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: getLabels(),
				},
				UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
					Type: appsv1.RollingUpdateDaemonSetStrategyType,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: utils.MergeStringMaps(getLabels(), map[string]string{
							v1beta1constants.GardenRole:                         v1beta1constants.GardenRoleMonitoring,
							managedresources.LabelKeyOrigin:                     managedresources.LabelValueGardener,
							v1beta1constants.LabelNetworkPolicyToPublicNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyShootFromSeed:    v1beta1constants.LabelNetworkPolicyAllowed,
						}),
					},
					Spec: corev1.PodSpec{
						Tolerations: []corev1.Toleration{
							{
								Effect:   corev1.TaintEffectNoSchedule,
								Operator: corev1.TolerationOpExists,
							},
							{
								Effect:   corev1.TaintEffectNoExecute,
								Operator: corev1.TolerationOpExists,
							},
						},
						HostNetwork:                  true,
						HostPID:                      true,
						PriorityClassName:            "system-cluster-critical",
						ServiceAccountName:           serviceAccount.Name,
						AutomountServiceAccountToken: pointer.Bool(false),
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot: pointer.Bool(true),
							RunAsUser:    pointer.Int64(65534),
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Containers: []corev1.Container{
							{
								Name:            containerName,
								Image:           n.values.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"/bin/node_exporter",
									fmt.Sprintf("--web.listen-address=:%d", portMetrics),
									"--path.procfs=/host/proc",
									"--path.sysfs=/host/sys",
									"--path.rootfs=/host",
									"--log.level=error",
									"--collector.disable-defaults",
									"--collector.conntrack",
									"--collector.cpu",
									"--collector.diskstats",
									"--collector.filefd",
									"--collector.filesystem",
									"--collector.filesystem.mount-points-exclude=^/(run|var)/.+$|^/(boot|dev|sys|usr)($|/.+$)",
									"--collector.loadavg",
									"--collector.meminfo",
									"--collector.uname",
									"--collector.stat",
									"--collector.pressure",
								},
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: portMetrics,
										Protocol:      corev1.ProtocolTCP,
										HostPort:      portMetrics,
										Name:          "scrape",
									},
								},
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/",
											Port: intstr.FromInt(int(portMetrics)),
										},
									},
									InitialDelaySeconds: 5,
									TimeoutSeconds:      5,
								},
								ReadinessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/",
											Port: intstr.FromInt(int(portMetrics)),
										},
									},
									InitialDelaySeconds: 5,
									TimeoutSeconds:      5,
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("50m"),
										corev1.ResourceMemory: resource.MustParse("50Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("250Mi"),
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      volumeNameHost,
										ReadOnly:  true,
										MountPath: volumeMountPathHost,
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: volumeNameHost,
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/",
									},
								},
							},
						},
					},
				},
			},
		}
	)

	return registry.AddAllAndSerialize(
		serviceAccount,
		service,
		daemonSet,
	)
}

func getLabels() map[string]string {
	return map[string]string{
		labelKeyComponent: labelValue,
	}
}
