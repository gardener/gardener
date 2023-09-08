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

package gardenerapiserver_test

import (
	"context"

	"github.com/Masterminds/semver"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/apiserver"
	. "github.com/gardener/gardener/pkg/component/gardenerapiserver"
	componenttest "github.com/gardener/gardener/pkg/component/test"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("GardenerAPIServer", func() {
	var (
		ctx = context.TODO()

		managedResourceNameRuntime       = "gardener-apiserver-runtime"
		managedResourceNameVirtual       = "gardener-apiserver-virtual"
		namespace                        = "some-namespace"
		image                            = "gapi-image"
		clusterIdentity                  = "cluster-id"
		logLevel                         = "log-level"
		logFormat                        = "log-format"
		replicas                   int32 = 1337
		resources                        = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("20Mi")},
			Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("90Mi")},
		}
		clusterIP = "1.2.3.4"

		fakeClient        client.Client
		fakeSecretManager secretsmanager.Interface
		values            Values
		deployer          Interface

		fakeOps *retryfake.Ops

		managedResourceRuntime       *resourcesv1alpha1.ManagedResource
		managedResourceVirtual       *resourcesv1alpha1.ManagedResource
		managedResourceSecretRuntime *corev1.Secret
		managedResourceSecretVirtual *corev1.Secret

		podDisruptionBudget *policyv1.PodDisruptionBudget
		serviceRuntime      *corev1.Service
		vpa                 *vpaautoscalingv1.VerticalPodAutoscaler
		hvpa                *hvpav1alpha1.Hvpa
		deployment          *appsv1.Deployment
		apiServiceFor       = func(group, version string) *apiregistrationv1.APIService {
			return &apiregistrationv1.APIService{
				ObjectMeta: metav1.ObjectMeta{
					Name: version + "." + group,
					Labels: map[string]string{
						"app":  "gardener",
						"role": "apiserver",
					},
				},
				Spec: apiregistrationv1.APIServiceSpec{
					Service: &apiregistrationv1.ServiceReference{
						Name:      "gardener-apiserver",
						Namespace: "kube-system",
					},
					Group:                group,
					Version:              version,
					GroupPriorityMinimum: 10000,
					VersionPriority:      20,
				},
			}
		}
		serviceVirtual                   *corev1.Service
		endpoints                        *corev1.Endpoints
		clusterRole                      *rbacv1.ClusterRole
		clusterRoleBinding               *rbacv1.ClusterRoleBinding
		clusterRoleBindingAuthDelegation *rbacv1.ClusterRoleBinding
		roleBindingAuthReader            *rbacv1.RoleBinding
	)

	BeforeEach(func() {
		testSchemeBuilder := runtime.NewSchemeBuilder(operatorclient.AddRuntimeSchemeToScheme, operatorclient.AddVirtualSchemeToScheme)
		testScheme := runtime.NewScheme()
		Expect(testSchemeBuilder.AddToScheme(testScheme)).To(Succeed())

		fakeClient = fakeclient.NewClientBuilder().WithScheme(testScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(fakeClient, namespace)
		values = Values{
			Values: apiserver.Values{
				Autoscaling: apiserver.AutoscalingConfig{
					Replicas:           &replicas,
					APIServerResources: resources,
				},
				ETCDEncryption: apiserver.ETCDEncryptionConfig{
					Resources: []string{"shootstates.core.gardener.cloud"},
				},
				RuntimeVersion: semver.MustParse("1.27.1"),
			},
			ClusterIdentity:             clusterIdentity,
			Image:                       image,
			LogFormat:                   logFormat,
			LogLevel:                    logLevel,
			TopologyAwareRoutingEnabled: true,
		}
		deployer = New(fakeClient, namespace, fakeSecretManager, values)

		fakeOps = &retryfake.Ops{MaxAttempts: 2}
		DeferCleanup(test.WithVars(
			&retry.Until, fakeOps.Until,
			&retry.UntilTimeout, fakeOps.UntilTimeout,
		))

		managedResourceRuntime = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceNameRuntime,
				Namespace: namespace,
			},
		}
		managedResourceVirtual = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceNameVirtual,
				Namespace: namespace,
			},
		}
		managedResourceSecretRuntime = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceRuntime.Name,
				Namespace: namespace,
			},
		}
		managedResourceSecretVirtual = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceVirtual.Name,
				Namespace: namespace,
			},
		}

		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-apiserver",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: gardenerutils.IntStrPtrFromInt(1),
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				}},
			},
		}
		serviceRuntime = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-apiserver",
				Namespace: namespace,
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-webhook-targets-allowed-ports": `[{"protocol":"TCP","port":8443}]`,
					"service.kubernetes.io/topology-mode":                                        "auto",
				},
				Labels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
					"endpoint-slice-hints.resources.gardener.cloud/consider": "true",
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Selector: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				},
				Ports: []corev1.ServicePort{{
					Port:       443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(8443),
				}},
			},
		}
		vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-apiserver-vpa",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				},
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "gardener-apiserver",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: "*",
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
						},
					},
				},
			},
		}
		hvpa = &hvpav1alpha1.Hvpa{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-apiserver-hvpa",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
					"high-availability-config.resources.gardener.cloud/type": "server",
				},
			},
			Spec: hvpav1alpha1.HvpaSpec{
				Replicas: pointer.Int32(1),
				Hpa: hvpav1alpha1.HpaSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"role": "gardener-apiserver-hpa"}},
					Deploy:   true,
					ScaleUp: hvpav1alpha1.ScaleType{
						UpdatePolicy: hvpav1alpha1.UpdatePolicy{
							UpdateMode: pointer.String("Auto"),
						},
					},
					ScaleDown: hvpav1alpha1.ScaleType{
						UpdatePolicy: hvpav1alpha1.UpdatePolicy{
							UpdateMode: pointer.String("Auto"),
						},
					},
					Template: hvpav1alpha1.HpaTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"role": "gardener-apiserver-hpa"},
						},
						Spec: hvpav1alpha1.HpaTemplateSpec{
							MinReplicas: pointer.Int32(1),
							MaxReplicas: 4,
							Metrics: []autoscalingv2beta1.MetricSpec{
								{
									Type: "Resource",
									Resource: &autoscalingv2beta1.ResourceMetricSource{
										Name:                     corev1.ResourceCPU,
										TargetAverageUtilization: pointer.Int32(80),
									},
								},
								{
									Type: "Resource",
									Resource: &autoscalingv2beta1.ResourceMetricSource{
										Name:                     corev1.ResourceMemory,
										TargetAverageUtilization: pointer.Int32(80),
									},
								},
							},
						},
					},
				},
				Vpa: hvpav1alpha1.VpaSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"role": "gardener-apiserver-vpa"}},
					Deploy:   true,
					ScaleUp: hvpav1alpha1.ScaleType{
						UpdatePolicy: hvpav1alpha1.UpdatePolicy{
							UpdateMode: pointer.String("Auto"),
						},
						StabilizationDuration: pointer.String("3m"),
						MinChange: hvpav1alpha1.ScaleParams{
							CPU: hvpav1alpha1.ChangeParams{
								Value:      pointer.String("300m"),
								Percentage: pointer.Int32(80),
							},
							Memory: hvpav1alpha1.ChangeParams{
								Value:      pointer.String("200M"),
								Percentage: pointer.Int32(80),
							},
						},
					},
					ScaleDown: hvpav1alpha1.ScaleType{
						UpdatePolicy: hvpav1alpha1.UpdatePolicy{
							UpdateMode: pointer.String("Auto"),
						},
						StabilizationDuration: pointer.String("15m"),
						MinChange: hvpav1alpha1.ScaleParams{
							CPU: hvpav1alpha1.ChangeParams{
								Value:      pointer.String("600m"),
								Percentage: pointer.Int32(80),
							},
							Memory: hvpav1alpha1.ChangeParams{
								Value:      pointer.String("600M"),
								Percentage: pointer.Int32(80),
							},
						},
					},
					LimitsRequestsGapScaleParams: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("1"),
							Percentage: pointer.Int32(70),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("1G"),
							Percentage: pointer.Int32(70),
						},
					},
					Template: hvpav1alpha1.VpaTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"role": "gardener-apiserver-vpa"},
						},
						Spec: hvpav1alpha1.VpaTemplateSpec{
							ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
								ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
									ContainerName: "gardener-apiserver",
									MinAllowed: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("400M"),
									},
									MaxAllowed: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("4"),
										corev1.ResourceMemory: resource.MustParse("25G"),
									},
								}},
							},
						},
					},
				},
				WeightBasedScalingIntervals: []hvpav1alpha1.WeightBasedScalingInterval{{
					VpaWeight:         0,
					StartReplicaCount: 1,
					LastReplicaCount:  3,
				}},
				TargetRef: &autoscalingv2beta1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "gardener-apiserver",
				},
			},
		}
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-apiserver",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
					"high-availability-config.resources.gardener.cloud/type": "server",
				},
				Annotations: map[string]string{
					"reference.resources.gardener.cloud/configmap-0e4e3fd5": "gardener-apiserver-audit-policy-config-f5b578b4",
					"reference.resources.gardener.cloud/configmap-a6e4dc6f": "gardener-apiserver-admission-config-e38ff146",
					"reference.resources.gardener.cloud/secret-9dca243c":    "shoot-access-gardener-apiserver",
					"reference.resources.gardener.cloud/secret-47fc132b":    "gardener-apiserver-admission-kubeconfigs-e3b0c442",
					"reference.resources.gardener.cloud/secret-389fbba5":    "etcd-client",
					"reference.resources.gardener.cloud/secret-867d23cd":    "generic-token-kubeconfig",
					"reference.resources.gardener.cloud/secret-02452d55":    "gardener-apiserver-etcd-encryption-configuration-944a649a",
					"reference.resources.gardener.cloud/secret-3696832b":    "gardener-apiserver",
					"reference.resources.gardener.cloud/secret-e01f5645":    "ca-etcd",
				},
			},
			Spec: appsv1.DeploymentSpec{
				MinReadySeconds:      30,
				RevisionHistoryLimit: pointer.Int32(2),
				Replicas:             &replicas,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				}},
				Strategy: appsv1.DeploymentStrategy{
					Type: appsv1.RollingUpdateDeploymentStrategyType,
					RollingUpdate: &appsv1.RollingUpdateDeployment{
						MaxSurge:       gardenerutils.IntStrPtrFromInt(1),
						MaxUnavailable: gardenerutils.IntStrPtrFromInt(0),
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":                              "gardener",
							"role":                             "apiserver",
							"networking.gardener.cloud/to-dns": "allowed",
							"networking.gardener.cloud/to-private-networks":                                   "allowed",
							"networking.gardener.cloud/to-public-networks":                                    "allowed",
							"networking.resources.gardener.cloud/to-all-webhook-targets":                      "allowed",
							"networking.resources.gardener.cloud/to-virtual-garden-etcd-main-client-tcp-2379": "allowed",
							"networking.resources.gardener.cloud/to-virtual-garden-kube-apiserver-tcp-443":    "allowed",
						},
						Annotations: map[string]string{
							"reference.resources.gardener.cloud/configmap-0e4e3fd5": "gardener-apiserver-audit-policy-config-f5b578b4",
							"reference.resources.gardener.cloud/configmap-a6e4dc6f": "gardener-apiserver-admission-config-e38ff146",
							"reference.resources.gardener.cloud/secret-9dca243c":    "shoot-access-gardener-apiserver",
							"reference.resources.gardener.cloud/secret-47fc132b":    "gardener-apiserver-admission-kubeconfigs-e3b0c442",
							"reference.resources.gardener.cloud/secret-389fbba5":    "etcd-client",
							"reference.resources.gardener.cloud/secret-867d23cd":    "generic-token-kubeconfig",
							"reference.resources.gardener.cloud/secret-02452d55":    "gardener-apiserver-etcd-encryption-configuration-944a649a",
							"reference.resources.gardener.cloud/secret-3696832b":    "gardener-apiserver",
							"reference.resources.gardener.cloud/secret-e01f5645":    "ca-etcd",
						},
					},
					Spec: corev1.PodSpec{
						AutomountServiceAccountToken: pointer.Bool(false),
						PriorityClassName:            "gardener-garden-system-500",
						Containers: []corev1.Container{{
							Name:            "gardener-apiserver",
							Image:           image,
							ImagePullPolicy: "IfNotPresent",
							Args: []string{
								"--authorization-always-allow-paths=/healthz",
								"--cluster-identity=" + clusterIdentity,
								"--authentication-kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
								"--authorization-kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
								"--kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
								"--log-level=" + logLevel,
								"--log-format=" + logFormat,
								"--secure-port=8443",
								"--http2-max-streams-per-connection=1000",
								"--etcd-cafile=/srv/kubernetes/etcd/ca/bundle.crt",
								"--etcd-certfile=/srv/kubernetes/etcd/client/tls.crt",
								"--etcd-keyfile=/srv/kubernetes/etcd/client/tls.key",
								"--etcd-servers=https://virtual-garden-etcd-main-client:2379",
								"--livez-grace-period=1m",
								"--profiling=false",
								"--shutdown-delay-duration=15s",
								"--tls-cert-file=/srv/kubernetes/apiserver/tls.crt",
								"--tls-private-key-file=/srv/kubernetes/apiserver/tls.key",
								"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_CHACHA20_POLY1305_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
								"--audit-policy-file=/etc/kubernetes/audit/audit-policy.yaml",
								"--audit-log-path=/tmp/audit/audit.log",
								"--audit-log-maxsize=100",
								"--audit-log-maxbackup=5",
								"--admission-control-config-file=/etc/kubernetes/admission/admission-configuration.yaml",
								"--encryption-provider-config=/etc/kubernetes/etcd-encryption-secret/encryption-configuration.yaml",
							},
							Ports: []corev1.ContainerPort{{
								Name:          "https",
								ContainerPort: 8443,
								Protocol:      corev1.ProtocolTCP,
							}},
							Resources: resources,
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/livez",
										Scheme: "HTTPS",
										Port:   intstr.FromInt(8443),
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
										Scheme: "HTTPS",
										Port:   intstr.FromInt(8443),
									},
								},
								SuccessThreshold:    1,
								FailureThreshold:    3,
								InitialDelaySeconds: 15,
								PeriodSeconds:       10,
								TimeoutSeconds:      15,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "ca-etcd",
									MountPath: "/srv/kubernetes/etcd/ca",
								},
								{
									Name:      "etcd-client",
									MountPath: "/srv/kubernetes/etcd/client",
								},
								{
									Name:      "server",
									MountPath: "/srv/kubernetes/apiserver",
								},
								{
									Name:      "audit-policy-config",
									MountPath: "/etc/kubernetes/audit",
								},
								{
									Name:      "admission-config",
									MountPath: "/etc/kubernetes/admission",
								},
								{
									Name:      "admission-kubeconfigs",
									MountPath: "/etc/kubernetes/admission-kubeconfigs",
								},
								{
									Name:      "etcd-encryption-secret",
									MountPath: "/etc/kubernetes/etcd-encryption-secret",
									ReadOnly:  true,
								},
							},
						}},
						Volumes: []corev1.Volume{
							{
								Name: "ca-etcd",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "ca-etcd",
									},
								},
							},
							{
								Name: "etcd-client",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "etcd-client",
									},
								},
							},
							{
								Name: "server",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "gardener-apiserver",
									},
								},
							},
							{
								Name: "audit-policy-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "gardener-apiserver-audit-policy-config-f5b578b4",
										},
									},
								},
							},
							{
								Name: "admission-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "gardener-apiserver-admission-config-e38ff146",
										},
									},
								},
							},
							{
								Name: "admission-kubeconfigs",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "gardener-apiserver-admission-kubeconfigs-e3b0c442",
									},
								},
							},
							{
								Name: "etcd-encryption-secret",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "gardener-apiserver-etcd-encryption-configuration-944a649a",
									},
								},
							},
						},
					},
				},
			},
		}
		utilruntime.Must(gardener.InjectGenericKubeconfig(deployment, "generic-token-kubeconfig", "shoot-access-gardener-apiserver"))

		serviceVirtual = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-apiserver",
				Namespace: "kube-system",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Selector: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				},
				Ports: []corev1.ServicePort{{
					Port:       443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(8443),
				}},
			},
		}
		endpoints = &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-apiserver",
				Namespace: "kube-system",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				},
			},
			Subsets: []corev1.EndpointSubset{{
				Ports: []corev1.EndpointPort{{
					Port:     443,
					Protocol: corev1.ProtocolTCP,
				}},
				Addresses: []corev1.EndpointAddress{{
					IP: clusterIP,
				}},
			}},
		}
		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:apiserver",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				},
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			}},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:apiserver",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:apiserver",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "gardener-apiserver",
				Namespace: "kube-system",
			}},
		}
		clusterRoleBindingAuthDelegation = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:apiserver:auth-delegator",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "system:auth-delegator",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "gardener-apiserver",
				Namespace: "kube-system",
			}},
		}
		roleBindingAuthReader = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:apiserver:auth-reader",
				Namespace: "kube-system",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "extension-apiserver-authentication-reader",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "gardener-apiserver",
				Namespace: "kube-system",
			}},
		}
	})

	Describe("#Deploy", func() {
		BeforeEach(func() {
			By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-gardener", Namespace: namespace}})).To(Succeed())
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd", Namespace: namespace}})).To(Succeed())
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "etcd-client", Namespace: namespace}})).To(Succeed())
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())

			// Create runtime service manually since it is required by the Deploy function. In reality, it gets created
			// via the ManagedResource, however in this unit test the respective controller is not running, hence we
			// have to create it here.
			svcRuntime := serviceRuntime.DeepCopy()
			Expect(fakeClient.Create(ctx, svcRuntime)).To(Succeed())
			patch := client.MergeFrom(svcRuntime.DeepCopy())
			svcRuntime.Spec.ClusterIP = clusterIP
			Expect(fakeClient.Patch(ctx, svcRuntime, patch)).To(Succeed())
		})

		Context("deployment", func() {
			BeforeEach(func() {
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(BeNotFoundError())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
				})).To(Succeed())
			})

			Context("secrets", func() {
				Context("etcd encryption config secrets", func() {
					It("should successfully deploy the ETCD encryption configuration secret resource", func() {
						etcdEncryptionConfiguration := `apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
- providers:
  - aescbc:
      keys:
      - name: key-62135596800
        secret: ________________________________
  - identity: {}
  resources:
  - shootstates.core.gardener.cloud
`

						By("Verify encryption config secret")
						expectedSecretETCDEncryptionConfiguration := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-etcd-encryption-configuration", Namespace: namespace},
							Data:       map[string][]byte{"encryption-configuration.yaml": []byte(etcdEncryptionConfiguration)},
						}
						Expect(kubernetesutils.MakeUnique(expectedSecretETCDEncryptionConfiguration)).To(Succeed())

						actualSecretETCDEncryptionConfiguration := &corev1.Secret{}
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(BeNotFoundError())

						Expect(deployer.Deploy(ctx)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(Succeed())
						Expect(actualSecretETCDEncryptionConfiguration).To(Equal(&corev1.Secret{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "Secret",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      expectedSecretETCDEncryptionConfiguration.Name,
								Namespace: expectedSecretETCDEncryptionConfiguration.Namespace,
								Labels: map[string]string{
									"resources.gardener.cloud/garbage-collectable-reference": "true",
									"role": "gardener-apiserver-etcd-encryption-configuration",
								},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      expectedSecretETCDEncryptionConfiguration.Data,
						}))

						By("Deploy again and ensure that labels are still present")
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(Succeed())
						Expect(actualSecretETCDEncryptionConfiguration.Labels).To(Equal(map[string]string{
							"resources.gardener.cloud/garbage-collectable-reference": "true",
							"role": "gardener-apiserver-etcd-encryption-configuration",
						}))

						By("Verify encryption key secret")
						secretList := &corev1.SecretList{}
						Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
							"name":       "gardener-apiserver-etcd-encryption-key",
							"managed-by": "secrets-manager",
						})).To(Succeed())
						Expect(secretList.Items).To(HaveLen(1))
						Expect(secretList.Items[0].Labels).To(HaveKeyWithValue("persist", "true"))
					})

					DescribeTable("successfully deploy the ETCD encryption configuration secret resource w/ old key",
						func(encryptWithCurrentKey bool) {
							deployer = New(fakeClient, namespace, fakeSecretManager, Values{
								Values: apiserver.Values{
									ETCDEncryption: apiserver.ETCDEncryptionConfig{EncryptWithCurrentKey: encryptWithCurrentKey, Resources: []string{"shootstates.core.gardener.cloud"}},
								},
							})

							oldKeyName, oldKeySecret := "key-old", "old-secret"
							Expect(fakeClient.Create(ctx, &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "gardener-apiserver-etcd-encryption-key-old",
									Namespace: namespace,
								},
								Data: map[string][]byte{
									"key":    []byte(oldKeyName),
									"secret": []byte(oldKeySecret),
								},
							})).To(Succeed())

							etcdEncryptionConfiguration := `apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
- providers:
  - aescbc:
      keys:`

							if encryptWithCurrentKey {
								etcdEncryptionConfiguration += `
      - name: key-62135596800
        secret: ________________________________
      - name: ` + oldKeyName + `
        secret: ` + oldKeySecret
							} else {
								etcdEncryptionConfiguration += `
      - name: ` + oldKeyName + `
        secret: ` + oldKeySecret + `
      - name: key-62135596800
        secret: ________________________________`
							}

							etcdEncryptionConfiguration += `
  - identity: {}
  resources:
  - shootstates.core.gardener.cloud
`

							expectedSecretETCDEncryptionConfiguration := &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-etcd-encryption-configuration", Namespace: namespace},
								Data:       map[string][]byte{"encryption-configuration.yaml": []byte(etcdEncryptionConfiguration)},
							}
							Expect(kubernetesutils.MakeUnique(expectedSecretETCDEncryptionConfiguration)).To(Succeed())

							actualSecretETCDEncryptionConfiguration := &corev1.Secret{}
							Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(BeNotFoundError())

							Expect(deployer.Deploy(ctx)).To(Succeed())

							Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(Succeed())
							Expect(actualSecretETCDEncryptionConfiguration).To(DeepEqual(&corev1.Secret{
								TypeMeta: metav1.TypeMeta{
									APIVersion: corev1.SchemeGroupVersion.String(),
									Kind:       "Secret",
								},
								ObjectMeta: metav1.ObjectMeta{
									Name:      expectedSecretETCDEncryptionConfiguration.Name,
									Namespace: expectedSecretETCDEncryptionConfiguration.Namespace,
									Labels: map[string]string{
										"resources.gardener.cloud/garbage-collectable-reference": "true",
										"role": "gardener-apiserver-etcd-encryption-configuration",
									},
									ResourceVersion: "1",
								},
								Immutable: pointer.Bool(true),
								Data:      expectedSecretETCDEncryptionConfiguration.Data,
							}))

							secretList := &corev1.SecretList{}
							Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
								"name":       "gardener-apiserver-etcd-encryption-key",
								"managed-by": "secrets-manager",
							})).To(Succeed())
							Expect(secretList.Items).To(HaveLen(1))
							Expect(secretList.Items[0].Labels).To(HaveKeyWithValue("persist", "true"))
						},

						Entry("encrypting with current", true),
						Entry("encrypting with old", false),
					)
				})

				It("should successfully deploy the access secret for the virtual garden", func() {
					accessSecret := &corev1.Secret{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Secret",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shoot-access-gardener-apiserver",
							Namespace: namespace,
							Labels: map[string]string{
								"resources.gardener.cloud/purpose": "token-requestor",
								"resources.gardener.cloud/class":   "shoot",
							},
							Annotations: map[string]string{
								"serviceaccount.resources.gardener.cloud/name":      "gardener-apiserver",
								"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
							},
						},
						Type: corev1.SecretTypeOpaque,
					}

					Expect(deployer.Deploy(ctx)).To(Succeed())

					actualShootAccessSecret := &corev1.Secret{}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(accessSecret), actualShootAccessSecret)).To(Succeed())
					accessSecret.ResourceVersion = "1"
					Expect(actualShootAccessSecret).To(Equal(accessSecret))
				})

				It("should successfully deploy the audit webhook kubeconfig secret resource", func() {
					var (
						kubeconfig  = []byte("some-kubeconfig")
						auditConfig = &apiserver.AuditConfig{Webhook: &apiserver.AuditWebhook{Kubeconfig: kubeconfig}}
					)

					deployer = New(fakeClient, namespace, fakeSecretManager, Values{
						Values: apiserver.Values{
							Audit: auditConfig,
						},
					})

					expectedSecret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-audit-webhook-kubeconfig", Namespace: namespace},
						Data:       map[string][]byte{"kubeconfig.yaml": kubeconfig},
					}
					Expect(kubernetesutils.MakeUnique(expectedSecret)).To(Succeed())

					actualSecret := &corev1.Secret{}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecret), actualSecret)).To(BeNotFoundError())

					Expect(deployer.Deploy(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecret), actualSecret)).To(Succeed())
					Expect(actualSecret).To(DeepEqual(&corev1.Secret{
						TypeMeta: metav1.TypeMeta{
							APIVersion: corev1.SchemeGroupVersion.String(),
							Kind:       "Secret",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            expectedSecret.Name,
							Namespace:       expectedSecret.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: pointer.Bool(true),
						Data:      expectedSecret.Data,
					}))
				})

				Context("admission kubeconfigs", func() {
					It("should successfully deploy the secret resource w/o admission plugin kubeconfigs", func() {
						secretAdmissionKubeconfigs := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-admission-kubeconfigs", Namespace: namespace},
							Data:       map[string][]byte{},
						}
						Expect(kubernetesutils.MakeUnique(secretAdmissionKubeconfigs)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secretAdmissionKubeconfigs), secretAdmissionKubeconfigs)).To(BeNotFoundError())
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secretAdmissionKubeconfigs), secretAdmissionKubeconfigs)).To(Succeed())
						Expect(secretAdmissionKubeconfigs).To(DeepEqual(&corev1.Secret{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "Secret",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            secretAdmissionKubeconfigs.Name,
								Namespace:       secretAdmissionKubeconfigs.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      secretAdmissionKubeconfigs.Data,
						}))
					})

					It("should successfully deploy the configmap resource w/ admission plugins", func() {
						admissionPlugins := []apiserver.AdmissionPluginConfig{
							{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
							{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Baz"}, Kubeconfig: []byte("foo")},
						}

						deployer = New(fakeClient, namespace, fakeSecretManager, Values{
							Values: apiserver.Values{
								EnabledAdmissionPlugins: admissionPlugins,
							},
						})

						secretAdmissionKubeconfigs := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-admission-kubeconfigs", Namespace: namespace},
							Data: map[string][]byte{
								"baz-kubeconfig.yaml": []byte("foo"),
							},
						}
						Expect(kubernetesutils.MakeUnique(secretAdmissionKubeconfigs)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secretAdmissionKubeconfigs), secretAdmissionKubeconfigs)).To(BeNotFoundError())
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secretAdmissionKubeconfigs), secretAdmissionKubeconfigs)).To(Succeed())
						Expect(secretAdmissionKubeconfigs).To(DeepEqual(&corev1.Secret{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "Secret",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            secretAdmissionKubeconfigs.Name,
								Namespace:       secretAdmissionKubeconfigs.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      secretAdmissionKubeconfigs.Data,
						}))
					})
				})
			})

			Context("configmaps", func() {
				Context("audit", func() {
					It("should successfully deploy the configmap resource w/ default policy", func() {
						configMapAuditPolicy := &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-audit-policy-config", Namespace: namespace},
							Data: map[string]string{"audit-policy.yaml": `apiVersion: audit.k8s.io/v1
kind: Policy
metadata:
  creationTimestamp: null
rules:
- level: None
`},
						}
						Expect(kubernetesutils.MakeUnique(configMapAuditPolicy)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAuditPolicy), configMapAuditPolicy)).To(BeNotFoundError())
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAuditPolicy), configMapAuditPolicy)).To(Succeed())
						Expect(configMapAuditPolicy).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAuditPolicy.Name,
								Namespace:       configMapAuditPolicy.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapAuditPolicy.Data,
						}))
					})

					It("should successfully deploy the configmap resource w/o default policy", func() {
						var (
							policy      = "some-audit-policy"
							auditConfig = &apiserver.AuditConfig{Policy: &policy}
						)

						deployer = New(fakeClient, namespace, fakeSecretManager, Values{
							Values: apiserver.Values{
								Audit: auditConfig,
							},
						})

						configMapAuditPolicy := &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-audit-policy-config", Namespace: namespace},
							Data:       map[string]string{"audit-policy.yaml": policy},
						}
						Expect(kubernetesutils.MakeUnique(configMapAuditPolicy)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAuditPolicy), configMapAuditPolicy)).To(BeNotFoundError())
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAuditPolicy), configMapAuditPolicy)).To(Succeed())
						Expect(configMapAuditPolicy).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAuditPolicy.Name,
								Namespace:       configMapAuditPolicy.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapAuditPolicy.Data,
						}))
					})
				})

				Context("admission", func() {
					It("should successfully deploy the configmap resource w/o admission plugins", func() {
						configMapAdmission := &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-admission-config", Namespace: namespace},
							Data: map[string]string{"admission-configuration.yaml": `apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins: null
`},
						}
						Expect(kubernetesutils.MakeUnique(configMapAdmission)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(BeNotFoundError())
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(Succeed())
						Expect(configMapAdmission).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAdmission.Name,
								Namespace:       configMapAdmission.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapAdmission.Data,
						}))
					})

					It("should successfully deploy the configmap resource w/ admission plugins", func() {
						admissionPlugins := []apiserver.AdmissionPluginConfig{
							{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
							{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("some-config-for-baz")}}},
							{
								AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
									Name: "MutatingAdmissionWebhook",
									Config: &runtime.RawExtension{Raw: []byte(`apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/foobar.yaml
`)},
								},
								Kubeconfig: []byte("foo"),
							},
							{
								AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
									Name: "ValidatingAdmissionWebhook",
									Config: &runtime.RawExtension{Raw: []byte(`apiVersion: apiserver.config.k8s.io/v1alpha1
kind: WebhookAdmission
kubeConfigFile: /etc/kubernetes/foobar.yaml
`)},
								},
								Kubeconfig: []byte("foo"),
							},
						}

						deployer = New(fakeClient, namespace, fakeSecretManager, Values{
							Values: apiserver.Values{
								EnabledAdmissionPlugins: admissionPlugins,
							},
						})

						configMapAdmission := &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-admission-config", Namespace: namespace},
							Data: map[string]string{
								"admission-configuration.yaml": `apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins:
- configuration: null
  name: Baz
  path: /etc/kubernetes/admission/baz.yaml
- configuration: null
  name: MutatingAdmissionWebhook
  path: /etc/kubernetes/admission/mutatingadmissionwebhook.yaml
- configuration: null
  name: ValidatingAdmissionWebhook
  path: /etc/kubernetes/admission/validatingadmissionwebhook.yaml
`,
								"baz.yaml": "some-config-for-baz",
								"mutatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/mutatingadmissionwebhook-kubeconfig.yaml
`,
								"validatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1alpha1
kind: WebhookAdmission
kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/validatingadmissionwebhook-kubeconfig.yaml
`,
							},
						}
						Expect(kubernetesutils.MakeUnique(configMapAdmission)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(BeNotFoundError())
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(Succeed())
						Expect(configMapAdmission).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAdmission.Name,
								Namespace:       configMapAdmission.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapAdmission.Data,
						}))
					})

					It("should successfully deploy the configmap resource w/ admission plugins w/ config but w/o kubeconfigs", func() {
						admissionPlugins := []apiserver.AdmissionPluginConfig{
							{
								AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
									Name: "MutatingAdmissionWebhook",
									Config: &runtime.RawExtension{Raw: []byte(`apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/foobar.yaml
`)},
								},
							},
							{
								AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
									Name: "ValidatingAdmissionWebhook",
									Config: &runtime.RawExtension{Raw: []byte(`apiVersion: apiserver.config.k8s.io/v1alpha1
kind: WebhookAdmission
kubeConfigFile: /etc/kubernetes/foobar.yaml
`)},
								},
							},
						}

						deployer = New(fakeClient, namespace, fakeSecretManager, Values{
							Values: apiserver.Values{
								EnabledAdmissionPlugins: admissionPlugins,
							},
						})

						configMapAdmission := &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-admission-config", Namespace: namespace},
							Data: map[string]string{
								"admission-configuration.yaml": `apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins:
- configuration: null
  name: MutatingAdmissionWebhook
  path: /etc/kubernetes/admission/mutatingadmissionwebhook.yaml
- configuration: null
  name: ValidatingAdmissionWebhook
  path: /etc/kubernetes/admission/validatingadmissionwebhook.yaml
`,
								"mutatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: ""
`,
								"validatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1alpha1
kind: WebhookAdmission
kubeConfigFile: ""
`,
							},
						}
						Expect(kubernetesutils.MakeUnique(configMapAdmission)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(BeNotFoundError())
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(Succeed())
						Expect(configMapAdmission).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAdmission.Name,
								Namespace:       configMapAdmission.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapAdmission.Data,
						}))
					})

					It("should successfully deploy the configmap resource w/ admission plugins w/o configs but w/ kubeconfig", func() {
						admissionPlugins := []apiserver.AdmissionPluginConfig{
							{
								AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
									Name: "MutatingAdmissionWebhook",
								},
								Kubeconfig: []byte("foo"),
							},
							{
								AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
									Name: "ValidatingAdmissionWebhook",
								},
								Kubeconfig: []byte("foo"),
							},
						}

						deployer = New(fakeClient, namespace, fakeSecretManager, Values{
							Values: apiserver.Values{
								EnabledAdmissionPlugins: admissionPlugins,
							},
						})

						configMapAdmission := &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-admission-config", Namespace: namespace},
							Data: map[string]string{
								"admission-configuration.yaml": `apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins:
- configuration: null
  name: MutatingAdmissionWebhook
  path: /etc/kubernetes/admission/mutatingadmissionwebhook.yaml
- configuration: null
  name: ValidatingAdmissionWebhook
  path: /etc/kubernetes/admission/validatingadmissionwebhook.yaml
`,
								"mutatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/mutatingadmissionwebhook-kubeconfig.yaml
`,
								"validatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/validatingadmissionwebhook-kubeconfig.yaml
`,
							},
						}
						Expect(kubernetesutils.MakeUnique(configMapAdmission)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(BeNotFoundError())
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(Succeed())
						Expect(configMapAdmission).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAdmission.Name,
								Namespace:       configMapAdmission.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapAdmission.Data,
						}))
					})
				})
			})

			Context("resources generation", func() {
				JustBeforeEach(func() {
					Expect(deployer.Deploy(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(Succeed())
					expectedRuntimeMr := &resourcesv1alpha1.ManagedResource{
						TypeMeta: metav1.TypeMeta{
							APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ManagedResource",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            managedResourceRuntime.Name,
							Namespace:       managedResourceRuntime.Namespace,
							ResourceVersion: "2",
							Generation:      1,
							Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
						},
						Spec: resourcesv1alpha1.ManagedResourceSpec{
							Class:       pointer.String("seed"),
							SecretRefs:  []corev1.LocalObjectReference{{Name: managedResourceRuntime.Spec.SecretRefs[0].Name}},
							KeepObjects: pointer.Bool(false),
						},
					}
					utilruntime.Must(references.InjectAnnotations(expectedRuntimeMr))
					Expect(managedResourceRuntime).To(Equal(expectedRuntimeMr))

					managedResourceSecretRuntime.Name = managedResourceRuntime.Spec.SecretRefs[0].Name
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(Succeed())
					expectedVirtualMr := &resourcesv1alpha1.ManagedResource{
						TypeMeta: metav1.TypeMeta{
							APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ManagedResource",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            managedResourceVirtual.Name,
							Namespace:       managedResourceVirtual.Namespace,
							ResourceVersion: "2",
							Generation:      1,
							Labels:          map[string]string{"origin": "gardener"},
						},
						Spec: resourcesv1alpha1.ManagedResourceSpec{
							InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
							SecretRefs:   []corev1.LocalObjectReference{{Name: managedResourceVirtual.Spec.SecretRefs[0].Name}},
							KeepObjects:  pointer.Bool(false),
						},
					}
					utilruntime.Must(references.InjectAnnotations(expectedVirtualMr))
					Expect(managedResourceVirtual).To(Equal(expectedVirtualMr))

					managedResourceSecretVirtual.Name = managedResourceVirtual.Spec.SecretRefs[0].Name
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(Succeed())

					Expect(managedResourceSecretRuntime.Type).To(Equal(corev1.SecretTypeOpaque))
					Expect(managedResourceSecretRuntime.Data).To(HaveLen(4))
					Expect(string(managedResourceSecretRuntime.Data["poddisruptionbudget__some-namespace__gardener-apiserver.yaml"])).To(Equal(componenttest.Serialize(podDisruptionBudget)))
					Expect(string(managedResourceSecretRuntime.Data["service__some-namespace__gardener-apiserver.yaml"])).To(Equal(componenttest.Serialize(serviceRuntime)))
					Expect(string(managedResourceSecretRuntime.Data["deployment__some-namespace__gardener-apiserver.yaml"])).To(Equal(componenttest.Serialize(deployment)))
					Expect(managedResourceSecretRuntime.Immutable).To(Equal(pointer.Bool(true)))
					Expect(managedResourceSecretRuntime.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

					Expect(managedResourceSecretVirtual.Type).To(Equal(corev1.SecretTypeOpaque))
					Expect(managedResourceSecretVirtual.Data).To(HaveLen(10))
					Expect(string(managedResourceSecretVirtual.Data["apiservice____v1beta1.core.gardener.cloud.yaml"])).To(Equal(componenttest.Serialize(apiServiceFor("core.gardener.cloud", "v1beta1"))))
					Expect(string(managedResourceSecretVirtual.Data["apiservice____v1alpha1.seedmanagement.gardener.cloud.yaml"])).To(Equal(componenttest.Serialize(apiServiceFor("seedmanagement.gardener.cloud", "v1alpha1"))))
					Expect(string(managedResourceSecretVirtual.Data["apiservice____v1alpha1.operations.gardener.cloud.yaml"])).To(Equal(componenttest.Serialize(apiServiceFor("operations.gardener.cloud", "v1alpha1"))))
					Expect(string(managedResourceSecretVirtual.Data["apiservice____v1alpha1.settings.gardener.cloud.yaml"])).To(Equal(componenttest.Serialize(apiServiceFor("settings.gardener.cloud", "v1alpha1"))))
					Expect(string(managedResourceSecretVirtual.Data["service__kube-system__gardener-apiserver.yaml"])).To(Equal(componenttest.Serialize(serviceVirtual)))
					Expect(string(managedResourceSecretVirtual.Data["endpoints__kube-system__gardener-apiserver.yaml"])).To(Equal(componenttest.Serialize(endpoints)))
					Expect(string(managedResourceSecretVirtual.Data["clusterrole____gardener.cloud_system_apiserver.yaml"])).To(Equal(componenttest.Serialize(clusterRole)))
					Expect(string(managedResourceSecretVirtual.Data["clusterrolebinding____gardener.cloud_system_apiserver.yaml"])).To(Equal(componenttest.Serialize(clusterRoleBinding)))
					Expect(string(managedResourceSecretVirtual.Data["clusterrolebinding____gardener.cloud_apiserver_auth-delegator.yaml"])).To(Equal(componenttest.Serialize(clusterRoleBindingAuthDelegation)))
					Expect(string(managedResourceSecretVirtual.Data["rolebinding__kube-system__gardener.cloud_apiserver_auth-reader.yaml"])).To(Equal(componenttest.Serialize(roleBindingAuthReader)))
					Expect(managedResourceSecretVirtual.Immutable).To(Equal(pointer.Bool(true)))
					Expect(managedResourceSecretVirtual.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
				})

				Context("when HVPA is disabled", func() {
					BeforeEach(func() {
						values.Values.Autoscaling.HVPAEnabled = false
						deployer = New(fakeClient, namespace, fakeSecretManager, values)
					})

					It("should successfully deploy all resources", func() {
						Expect(string(managedResourceSecretRuntime.Data["verticalpodautoscaler__some-namespace__gardener-apiserver-vpa.yaml"])).To(Equal(componenttest.Serialize(vpa)))
					})
				})

				Context("when HVPA is enabled", func() {
					BeforeEach(func() {
						values.Values.Autoscaling.HVPAEnabled = true
						deployer = New(fakeClient, namespace, fakeSecretManager, values)
					})

					It("should successfully deploy all resources", func() {
						Expect(string(managedResourceSecretRuntime.Data["hvpa__some-namespace__gardener-apiserver-hvpa.yaml"])).To(Equal(componenttest.Serialize(hvpa)))
					})
				})
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(fakeClient.Create(ctx, managedResourceRuntime)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceVirtual)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecretRuntime)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecretVirtual)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(Succeed())

			Expect(deployer.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		Describe("#Wait", func() {
			It("should fail because reading the runtime ManagedResource fails", func() {
				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the runtime ManagedResource is unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is unhealthy")))
			})

			It("should fail because the runtime ManagedResource is still progressing", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("still progressing")))
			})

			It("should fail because the virtual ManagedResource is unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should succeed because the both ManagedResource are healthy and progressed", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the runtime managed resource deletion times out", func() {
				Expect(fakeClient.Create(ctx, managedResourceRuntime)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should fail when the wait for the virtual managed resource deletion times out", func() {
				Expect(fakeClient.Create(ctx, managedResourceVirtual)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when they are already removed", func() {
				Expect(deployer.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

var (
	healthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
		ObservedGeneration: 1,
		Conditions: []gardencorev1beta1.Condition{
			{
				Type:   resourcesv1alpha1.ResourcesApplied,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   resourcesv1alpha1.ResourcesHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   resourcesv1alpha1.ResourcesProgressing,
				Status: gardencorev1beta1.ConditionFalse,
			},
		},
	}
	unhealthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
		ObservedGeneration: 1,
		Conditions: []gardencorev1beta1.Condition{
			{
				Type:   resourcesv1alpha1.ResourcesApplied,
				Status: gardencorev1beta1.ConditionFalse,
			},
			{
				Type:   resourcesv1alpha1.ResourcesHealthy,
				Status: gardencorev1beta1.ConditionFalse,
			},
			{
				Type:   resourcesv1alpha1.ResourcesProgressing,
				Status: gardencorev1beta1.ConditionTrue,
			},
		},
	}
)
