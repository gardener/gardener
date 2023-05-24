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

package blackboxexporter

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Masterminds/semver"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/version"
)

// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
const ManagedResourceName = "shoot-core-blackbox-exporter"

// Interface contains functions for a blackbox-exporter deployer.
type Interface interface {
	component.DeployWaiter
}

// Values is a set of configuration values for the blackbox-exporter.
type Values struct {
	// Image is the container image used for blackbox-exporter.
	Image string
	// VPAEnabled marks whether VerticalPodAutoscaler is enabled for the shoot.
	VPAEnabled bool
	// KubernetesVersion is the Kubernetes version of the Shoot.
	KubernetesVersion *semver.Version
}

// New creates a new instance of DeployWaiter for blackbox-exporter.
func New(
	client client.Client,
	namespace string,
	values Values,
) Interface {
	return &blackboxExporter{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type blackboxExporter struct {
	client    client.Client
	namespace string
	values    Values
}

func (b *blackboxExporter) Deploy(ctx context.Context) error {
	data, err := b.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, b.client, b.namespace, ManagedResourceName, managedresources.LabelValueGardener, false, data)
}

func (b *blackboxExporter) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, b.client, b.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (b *blackboxExporter) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, b.client, b.namespace, ManagedResourceName)
}

func (b *blackboxExporter) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, b.client, b.namespace, ManagedResourceName)
}

func (b *blackboxExporter) computeResourcesData() (map[string][]byte, error) {
	var (
		intStrOne = intstr.FromInt(1)

		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter",
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleMonitoring,
					"component":                 "blackbox-exporter",
					"origin":                    "gardener",
				},
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter-config",
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					v1beta1constants.LabelApp:  "prometheus",
					v1beta1constants.LabelRole: v1beta1constants.GardenRoleMonitoring,
				},
			},
			Data: map[string]string{
				`blackbox.yaml`: `modules:
  http_kubernetes_service:
    prober: http
    timeout: 10s
    http:
      headers:
        Accept: "*/*"
        Accept-Language: "en-US"
      tls_config:
        ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
      bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
      preferred_ip_protocol: "ip4"
`,
			},
		}
	)

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	var (
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter",
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleMonitoring,
					"component":                 "blackbox-exporter",
					"origin":                    "gardener",
					resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeServer,
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             pointer.Int32(1),
				RevisionHistoryLimit: pointer.Int32(2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"component": "blackbox-exporter",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
						Labels: map[string]string{
							"origin":                    "gardener",
							v1beta1constants.GardenRole: v1beta1constants.GardenRoleMonitoring,
							"component":                 "blackbox-exporter",
							v1beta1constants.LabelNetworkPolicyShootFromSeed:    v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyToPublicNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyShootToAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
						},
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: serviceAccount.Name,
						PriorityClassName:  "system-cluster-critical",
						SecurityContext: &corev1.PodSecurityContext{
							RunAsUser:          pointer.Int64(65534),
							FSGroup:            pointer.Int64(65534),
							SupplementalGroups: []int64{1},
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "blackbox-exporter",
								Image: b.values.Image,
								Args: []string{
									"--config.file=/etc/blackbox_exporter/blackbox.yaml",
									"--log.level=debug",
								},
								ImagePullPolicy: corev1.PullIfNotPresent,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("25Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          "probe",
										ContainerPort: int32(9115),
										Protocol:      corev1.ProtocolTCP,
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "blackbox-exporter-config",
										MountPath: "/etc/blackbox_exporter",
									},
								},
							},
						},
						DNSConfig: &corev1.PodDNSConfig{
							Options: []corev1.PodDNSConfigOption{
								{
									Name:  "ndots",
									Value: pointer.String("3"),
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "blackbox-exporter-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMap.Name,
										},
									},
								},
							},
						},
					},
				},
			},
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter",
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					"component": "blackbox-exporter",
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name:     "probe",
						Port:     int32(9115),
						Protocol: corev1.ProtocolTCP,
					},
				},
				Selector: map[string]string{
					"component": "blackbox-exporter",
				},
			},
		}

		podDisruptionBudget client.Object
		vpa                 *vpaautoscalingv1.VerticalPodAutoscaler
	)

	utilruntime.Must(references.InjectAnnotations(deployment))

	if version.ConstraintK8sGreaterEqual121.Check(b.values.KubernetesVersion) {
		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter",
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleMonitoring,
					"component":                 "blackbox-exporter",
				},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: &intStrOne,
				Selector:       deployment.Spec.Selector,
			},
		}
	} else {
		podDisruptionBudget = &policyv1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter",
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleMonitoring,
					"component":                 "blackbox-exporter",
				},
			},
			Spec: policyv1beta1.PodDisruptionBudgetSpec{
				MaxUnavailable: &intStrOne,
				Selector:       deployment.Spec.Selector,
			},
		}
	}

	if b.values.VPAEnabled {
		vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
		vpaControlledValues := vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter",
				Namespace: metav1.NamespaceSystem,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       deployment.Name,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName:    vpaautoscalingv1.DefaultContainerResourcePolicy,
							ControlledValues: &vpaControlledValues,
						},
					},
				},
			},
		}
	}

	return registry.AddAllAndSerialize(
		serviceAccount,
		configMap,
		deployment,
		podDisruptionBudget,
		service,
		vpa,
	)
}
