// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver_test

import (
	"context"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/apiserver"
	. "github.com/gardener/gardener/pkg/component/gardener/apiserver"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
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
		ctx = context.Background()

		managedResourceNameRuntime       = "gardener-apiserver-runtime"
		managedResourceNameVirtual       = "gardener-apiserver-virtual"
		namespace                        = "some-namespace"
		image                            = "gapi-image"
		clusterIdentity                  = "cluster-id"
		workloadIdentityIssuer           = "https://issuer.gardener.cloud.local"
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
		consistOf         func(...client.Object) types.GomegaMatcher

		fakeOps *retryfake.Ops

		managedResourceRuntime       *resourcesv1alpha1.ManagedResource
		managedResourceVirtual       *resourcesv1alpha1.ManagedResource
		managedResourceSecretRuntime *corev1.Secret
		managedResourceSecretVirtual *corev1.Secret

		podDisruptionBudget *policyv1.PodDisruptionBudget
		serviceRuntime      *corev1.Service
		vpa                 *vpaautoscalingv1.VerticalPodAutoscaler
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
		serviceMonitor                   *monitoringv1.ServiceMonitor
	)

	BeforeEach(func() {
		fakeOps = &retryfake.Ops{MaxAttempts: 2}
		DeferCleanup(test.WithVars(
			&retry.UntilTimeout, fakeOps.UntilTimeout,
		))

		testSchemeBuilder := runtime.NewSchemeBuilder(operatorclient.AddRuntimeSchemeToScheme, operatorclient.AddVirtualSchemeToScheme)
		testScheme := runtime.NewScheme()
		Expect(testSchemeBuilder.AddToScheme(testScheme)).To(Succeed())

		fakeClient = fakeclient.NewClientBuilder().WithScheme(testScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(fakeClient, namespace)
		values = Values{
			Values: apiserver.Values{
				ETCDEncryption: apiserver.ETCDEncryptionConfig{
					ResourcesToEncrypt: []string{"shootstates.core.gardener.cloud"},
				},
				RuntimeVersion: semver.MustParse("1.27.1"),
			},
			Autoscaling: AutoscalingConfig{
				Replicas:           &replicas,
				APIServerResources: resources,
			},
			ClusterIdentity:                   clusterIdentity,
			Image:                             image,
			LogFormat:                         logFormat,
			LogLevel:                          logLevel,
			GoAwayChance:                      ptr.To(0.0015),
			ShootAdminKubeconfigMaxExpiration: &metav1.Duration{Duration: 1 * time.Hour},
			TopologyAwareRoutingEnabled:       true,
			WorkloadIdentityTokenIssuer:       workloadIdentityIssuer,
		}
		deployer = New(fakeClient, namespace, fakeSecretManager, values)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(fakeClient)

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
				MaxUnavailable: ptr.To(intstr.FromInt32(1)),
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				}},
				UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
			},
		}

		serviceRuntime = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-apiserver",
				Namespace: namespace,
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-webhook-targets-allowed-ports":       `[{"protocol":"TCP","port":8443}]`,
					"networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":8443}]`,
					"service.kubernetes.io/topology-mode":                                              "auto",
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
					TargetPort: intstr.FromInt32(8443),
				}},
			},
		}

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
					UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: "gardener-apiserver",
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("200M"),
							},
							MaxAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("12"),
								corev1.ResourceMemory: resource.MustParse("48G"),
							},
							ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
						},
					},
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
				},
				Annotations: map[string]string{
					"reference.resources.gardener.cloud/configmap-0e4e3fd5": "gardener-apiserver-audit-policy-config-f5b578b4",
					"reference.resources.gardener.cloud/configmap-a6e4dc6f": "gardener-apiserver-admission-config-e38ff146",
					"reference.resources.gardener.cloud/secret-9dca243c":    "shoot-access-gardener-apiserver",
					"reference.resources.gardener.cloud/secret-47fc132b":    "gardener-apiserver-admission-kubeconfigs-e3b0c442",
					"reference.resources.gardener.cloud/secret-389fbba5":    "etcd-client",
					"reference.resources.gardener.cloud/secret-867d23cd":    "generic-token-kubeconfig",
					"reference.resources.gardener.cloud/secret-3af026bf":    "gardener-apiserver-etcd-encryption-configuration-fe8711ae",
					"reference.resources.gardener.cloud/secret-3696832b":    "gardener-apiserver",
					"reference.resources.gardener.cloud/secret-e01f5645":    "ca-etcd",
					"reference.resources.gardener.cloud/secret-14294f8f":    "gardener-apiserver-workload-identity-signing-key-f70e59e4",
				},
			},
			Spec: appsv1.DeploymentSpec{
				MinReadySeconds:      30,
				RevisionHistoryLimit: ptr.To[int32](2),
				Replicas:             &replicas,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				}},
				Strategy: appsv1.DeploymentStrategy{
					Type: appsv1.RollingUpdateDeploymentStrategyType,
					RollingUpdate: &appsv1.RollingUpdateDeployment{
						MaxSurge:       ptr.To(intstr.FromString("100%")),
						MaxUnavailable: ptr.To(intstr.FromInt32(0)),
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
							"reference.resources.gardener.cloud/secret-3af026bf":    "gardener-apiserver-etcd-encryption-configuration-fe8711ae",
							"reference.resources.gardener.cloud/secret-3696832b":    "gardener-apiserver",
							"reference.resources.gardener.cloud/secret-e01f5645":    "ca-etcd",
							"reference.resources.gardener.cloud/secret-14294f8f":    "gardener-apiserver-workload-identity-signing-key-f70e59e4",
						},
					},
					Spec: corev1.PodSpec{
						AutomountServiceAccountToken: ptr.To(false),
						PriorityClassName:            "gardener-garden-system-500",
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot: ptr.To(true),
							RunAsUser:    ptr.To[int64](65532),
							RunAsGroup:   ptr.To[int64](65532),
							FSGroup:      ptr.To[int64](65532),
						},
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
								"--shoot-admin-kubeconfig-max-expiration=1h0m0s",
								"--goaway-chance=0.001500",
								"--workload-identity-token-issuer=" + workloadIdentityIssuer,
								"--workload-identity-signing-key-file=/etc/gardener-apiserver/workload-identity/signing/key.pem",
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
										Port:   intstr.FromInt32(8443),
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
										Port:   intstr.FromInt32(8443),
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
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "gardener-apiserver-workload-identity",
									MountPath: "/etc/gardener-apiserver/workload-identity/signing",
								},
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
								Name: "gardener-apiserver-workload-identity",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "gardener-apiserver-workload-identity-signing-key-f70e59e4",
										Items: []corev1.KeyToPath{
											{
												Key:  "id_rsa",
												Path: "key.pem",
											},
										},
									},
								},
							},
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
										SecretName:  "etcd-client",
										DefaultMode: ptr.To[int32](0640),
									},
								},
							},
							{
								Name: "server",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName:  "gardener-apiserver",
										DefaultMode: ptr.To[int32](0640),
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
										SecretName:  "gardener-apiserver-etcd-encryption-configuration-fe8711ae",
										DefaultMode: ptr.To[int32](0640),
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
					TargetPort: intstr.FromInt32(8443),
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
		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "garden-gardener-apiserver",
				Namespace: namespace,
				Labels:    map[string]string{"prometheus": "garden"},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				}},
				Endpoints: []monitoringv1.Endpoint{{
					TargetPort: ptr.To(intstr.FromInt32(8443)),
					Scheme:     "https",
					TLSConfig:  &monitoringv1.TLSConfig{SafeTLSConfig: monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)}},
					Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "shoot-access-prometheus-garden"},
						Key:                  "token",
					}},
					MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
						SourceLabels: []monitoringv1.LabelName{"__name__"},
						Action:       "keep",
						Regex:        `^(authentication_attempts|authenticated_user_requests|apiserver_admission_controller_admission_duration_seconds_.+|apiserver_admission_webhook_admission_duration_seconds_.+|apiserver_admission_step_admission_duration_seconds_.+|apiserver_admission_webhook_rejection_count|apiserver_audit_event_total|apiserver_audit_error_total|apiserver_audit_requests_rejected_total|apiserver_request_total|apiserver_storage_objects|apiserver_latency_seconds|apiserver_current_inflight_requests|apiserver_current_inqueue_requests|apiserver_response_sizes_.+|apiserver_request_duration_seconds_.+|apiserver_request_terminations_total|apiserver_storage_transformation_duration_seconds_.+|apiserver_storage_transformation_operations_total|apiserver_registered_watchers|apiserver_init_events_total|apiserver_watch_events_sizes_.+|apiserver_watch_events_total|etcd_request_duration_seconds_.+|watch_cache_capacity_increase_total|watch_cache_capacity_decrease_total|watch_cache_capacity|go_.+|apiserver_cache_list_.+|apiserver_storage_list_.+)$`,
					}},
				}},
			},
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
        secret: X19fX19fX19fX19fX19fX19fX19fX19fX19fX19fX18=
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
							ObjectMeta: metav1.ObjectMeta{
								Name:      expectedSecretETCDEncryptionConfiguration.Name,
								Namespace: expectedSecretETCDEncryptionConfiguration.Namespace,
								Labels: map[string]string{
									"resources.gardener.cloud/garbage-collectable-reference": "true",
									"role": "gardener-apiserver-etcd-encryption-configuration",
								},
								ResourceVersion: "1",
							},
							Immutable: ptr.To(true),
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
									ETCDEncryption: apiserver.ETCDEncryptionConfig{EncryptWithCurrentKey: encryptWithCurrentKey, ResourcesToEncrypt: []string{"shootstates.core.gardener.cloud"}},
									RuntimeVersion: semver.MustParse("1.27.1"),
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
        secret: X19fX19fX19fX19fX19fX19fX19fX19fX19fX19fX18=
      - name: ` + oldKeyName + `
        secret: ` + oldKeySecret
							} else {
								etcdEncryptionConfiguration += `
      - name: ` + oldKeyName + `
        secret: ` + oldKeySecret + `
      - name: key-62135596800
        secret: X19fX19fX19fX19fX19fX19fX19fX19fX19fX19fX18=`
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
								ObjectMeta: metav1.ObjectMeta{
									Name:      expectedSecretETCDEncryptionConfiguration.Name,
									Namespace: expectedSecretETCDEncryptionConfiguration.Namespace,
									Labels: map[string]string{
										"resources.gardener.cloud/garbage-collectable-reference": "true",
										"role": "gardener-apiserver-etcd-encryption-configuration",
									},
									ResourceVersion: "1",
								},
								Immutable: ptr.To(true),
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
							Audit:          auditConfig,
							RuntimeVersion: semver.MustParse("1.27.1"),
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
						ObjectMeta: metav1.ObjectMeta{
							Name:            expectedSecret.Name,
							Namespace:       expectedSecret.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
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
							ObjectMeta: metav1.ObjectMeta{
								Name:            secretAdmissionKubeconfigs.Name,
								Namespace:       secretAdmissionKubeconfigs.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: ptr.To(true),
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
								RuntimeVersion:          semver.MustParse("1.27.1"),
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
							ObjectMeta: metav1.ObjectMeta{
								Name:            secretAdmissionKubeconfigs.Name,
								Namespace:       secretAdmissionKubeconfigs.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: ptr.To(true),
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
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAuditPolicy.Name,
								Namespace:       configMapAuditPolicy.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: ptr.To(true),
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
								Audit:          auditConfig,
								RuntimeVersion: semver.MustParse("1.27.1"),
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
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAuditPolicy.Name,
								Namespace:       configMapAuditPolicy.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: ptr.To(true),
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
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAdmission.Name,
								Namespace:       configMapAdmission.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: ptr.To(true),
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
								RuntimeVersion:          semver.MustParse("1.27.1"),
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
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAdmission.Name,
								Namespace:       configMapAdmission.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: ptr.To(true),
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
								RuntimeVersion:          semver.MustParse("1.27.1"),
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
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAdmission.Name,
								Namespace:       configMapAdmission.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: ptr.To(true),
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
								RuntimeVersion:          semver.MustParse("1.27.1"),
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
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAdmission.Name,
								Namespace:       configMapAdmission.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: ptr.To(true),
							Data:      configMapAdmission.Data,
						}))
					})
				})
			})

			Context("the gardener-apiserver service is not created", func() {
				It("should fail after retrying", func() {
					Expect(fakeClient.Delete(ctx, serviceRuntime)).To(Succeed())
					Eventually(ctx, func(g Gomega) {
						g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(serviceRuntime), &corev1.Service{})).To(BeNotFoundError())
					}).Should(Succeed())

					Expect(deployer.Deploy(ctx)).To(MatchError(ContainSubstring("failed waiting for service some-namespace/gardener-apiserver to get created by gardener-resource-manager")))
				})
			})

			Context("resources generation", func() {
				var expectedRuntimeObjects []client.Object

				JustBeforeEach(func() {
					Expect(deployer.Deploy(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(Succeed())
					expectedRuntimeMr := &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:            managedResourceRuntime.Name,
							Namespace:       managedResourceRuntime.Namespace,
							ResourceVersion: "2",
							Generation:      1,
							Labels: map[string]string{
								"gardener.cloud/role":                "seed-system-component",
								"care.gardener.cloud/condition-type": "VirtualComponentsHealthy",
							},
						},
						Spec: resourcesv1alpha1.ManagedResourceSpec{
							Class:       ptr.To("seed"),
							SecretRefs:  []corev1.LocalObjectReference{{Name: managedResourceRuntime.Spec.SecretRefs[0].Name}},
							KeepObjects: ptr.To(false),
						},
					}
					utilruntime.Must(references.InjectAnnotations(expectedRuntimeMr))
					Expect(managedResourceRuntime).To(Equal(expectedRuntimeMr))

					managedResourceSecretRuntime.Name = managedResourceRuntime.Spec.SecretRefs[0].Name
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(Succeed())
					expectedVirtualMr := &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:            managedResourceVirtual.Name,
							Namespace:       managedResourceVirtual.Namespace,
							ResourceVersion: "2",
							Generation:      1,
							Labels: map[string]string{
								"origin":                             "gardener",
								"care.gardener.cloud/condition-type": "VirtualComponentsHealthy",
							},
						},
						Spec: resourcesv1alpha1.ManagedResourceSpec{
							InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
							SecretRefs:   []corev1.LocalObjectReference{{Name: managedResourceVirtual.Spec.SecretRefs[0].Name}},
							KeepObjects:  ptr.To(false),
						},
					}
					utilruntime.Must(references.InjectAnnotations(expectedVirtualMr))
					Expect(managedResourceVirtual).To(Equal(expectedVirtualMr))

					managedResourceSecretVirtual.Name = managedResourceVirtual.Spec.SecretRefs[0].Name
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(Succeed())

					expectedRuntimeObjects = []client.Object{deployment, serviceMonitor, vpa, podDisruptionBudget}
					Expect(managedResourceSecretRuntime.Type).To(Equal(corev1.SecretTypeOpaque))
					Expect(managedResourceSecretRuntime.Immutable).To(Equal(ptr.To(true)))
					Expect(managedResourceSecretRuntime.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

					Expect(managedResourceVirtual).To(consistOf(
						apiServiceFor("core.gardener.cloud", "v1"),
						apiServiceFor("core.gardener.cloud", "v1beta1"),
						apiServiceFor("seedmanagement.gardener.cloud", "v1alpha1"),
						apiServiceFor("operations.gardener.cloud", "v1alpha1"),
						apiServiceFor("settings.gardener.cloud", "v1alpha1"),
						apiServiceFor("security.gardener.cloud", "v1alpha1"),
						serviceVirtual,
						endpoints,
						clusterRole,
						clusterRoleBinding,
						clusterRoleBindingAuthDelegation,
						roleBindingAuthReader,
					))
					Expect(managedResourceSecretVirtual.Type).To(Equal(corev1.SecretTypeOpaque))
					Expect(managedResourceSecretVirtual.Immutable).To(Equal(ptr.To(true)))
					Expect(managedResourceSecretVirtual.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
				})

				Context("when kubernetes version is >= 1.27", func() {
					BeforeEach(func() {
						values.RuntimeVersion = semver.MustParse("1.27.0")
						deployer = New(fakeClient, namespace, fakeSecretManager, values)
					})

					It("should successfully deploy all resources", func() {
						expectedRuntimeObjects = append(
							expectedRuntimeObjects,
							podDisruptionBudget,
							serviceRuntime,
						)

						Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObjects...))
					})
				})

				Context("when kubernetes version is < 1.26", func() {
					BeforeEach(func() {
						values.RuntimeVersion = semver.MustParse("1.25.0")
						deployer = New(fakeClient, namespace, fakeSecretManager, values)
					})

					It("should successfully deploy all resources", func() {
						expectedRuntimeObjects = append(
							expectedRuntimeObjects,
							podDisruptionBudget,
							serviceRuntime,
						)

						Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObjects...))
					})
				})

				Context("when kubernetes version is < 1.27", func() {
					BeforeEach(func() {
						values.RuntimeVersion = semver.MustParse("1.26.0")
						deployer = New(fakeClient, namespace, fakeSecretManager, values)
					})

					It("should successfully deploy all resources", func() {
						expectedRuntimeObjects = append(
							expectedRuntimeObjects,
							serviceMonitor,
							podDisruptionBudget,
							serviceRuntime,
						)

						Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObjects...))
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

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
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
