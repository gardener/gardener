// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package discoveryserver_test

import (
	"context"
	"crypto"
	"crypto/rand"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/gardener/discoveryserver"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/pkg/utils/workloadidentity"
)

var _ = Describe("GardenerDiscoveryServer", func() {
	var (
		ctx context.Context

		managedResourceNameRuntime = "gardener-discovery-server-runtime"
		managedResourceNameVirtual = "gardener-discovery-server-virtual"
		namespace                  = "some-namespace"

		image                      = "gardener-discovery-server-image:latest"
		workloadIdentityPrivateKey []byte
		workloadIdentityPublicKey  crypto.PublicKey

		workloadIdentityIssuer string

		fakeClient        client.Client
		fakeSecretManager secretsmanager.Interface
		deployer          component.DeployWaiter
		values            discoveryserver.Values

		fakeOps   *retryfake.Ops
		consistOf func(...client.Object) types.GomegaMatcher

		managedResourceRuntime       *resourcesv1alpha1.ManagedResource
		managedResourceVirtual       *resourcesv1alpha1.ManagedResource
		managedResourceSecretRuntime *corev1.Secret
		managedResourceSecretVirtual *corev1.Secret

		virtualGardenAccessSecret *corev1.Secret
		deployment                *appsv1.Deployment
		service                   *corev1.Service
		podDisruptionBudget       *policyv1.PodDisruptionBudget
		vpa                       *vpaautoscalingv1.VerticalPodAutoscaler
		ingress                   *networkingv1.Ingress
		serviceMonitor            *monitoringv1.ServiceMonitor
		workloadIdentitySecret    *corev1.Secret

		secretConfig *corev1.Secret

		clusterRole        *rbacv1.ClusterRole
		clusterRoleBinding *rbacv1.ClusterRoleBinding
		role               *rbacv1.Role
		roleBinding        *rbacv1.RoleBinding
	)

	BeforeEach(func() {
		ctx = context.Background()

		privateKey, err := secretsutils.FakeGenerateKey(rand.Reader, 2048)
		Expect(err).ToNot(HaveOccurred())

		workloadIdentityPrivateKey = utils.EncodePrivateKey(privateKey)
		workloadIdentityPublicKey = privateKey.Public()
		workloadIdentityIssuer = "https://local.gardener.cloud/garden/workload-identity/issuer"

		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(fakeClient, namespace)

		fakeOps = &retryfake.Ops{MaxAttempts: 2}
		DeferCleanup(test.WithVars(
			&retry.Until, fakeOps.Until,
			&retry.UntilTimeout, fakeOps.UntilTimeout,
		))

		consistOf = NewManagedResourceConsistOfObjectsMatcher(fakeClient)

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
	})

	JustBeforeEach(func() {
		virtualGardenAccessSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-access-gardener-discovery-server",
				Namespace: namespace,
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "gardener-discovery-server",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
			},
			Type: corev1.SecretTypeOpaque,
		}

		openidConfig, err := workloadidentity.OpenIDConfig(workloadIdentityIssuer, workloadIdentityPublicKey)
		Expect(err).ToNot(HaveOccurred())

		jwks, err := workloadidentity.JWKS(workloadIdentityPublicKey)
		Expect(err).ToNot(HaveOccurred())

		workloadIdentitySecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "gardener-discovery-server-garden-workload-identity",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "discovery-server",
				},
			},
			Data: map[string][]byte{
				"openid-configuration.json": openidConfig,
				"jwks.json":                 jwks,
			},
			Type: corev1.SecretTypeOpaque,
		}
		Expect(kubernetesutils.MakeUnique(workloadIdentitySecret)).To(Succeed())

		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-discovery-server",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "discovery-server",
					"high-availability-config.resources.gardener.cloud/type": "server",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             ptr.To[int32](1),
				RevisionHistoryLimit: ptr.To[int32](2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":  "gardener",
						"role": "discovery-server",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":                              "gardener",
							"role":                             "discovery-server",
							"networking.gardener.cloud/to-dns": "allowed",
							"networking.resources.gardener.cloud/to-virtual-garden-kube-apiserver-tcp-443": "allowed",
						},
					},
					Spec: corev1.PodSpec{
						PriorityClassName:            "gardener-garden-system-200",
						AutomountServiceAccountToken: ptr.To(false),
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot: ptr.To(true),
							RunAsUser:    ptr.To[int64](65532),
							RunAsGroup:   ptr.To[int64](65532),
							FSGroup:      ptr.To[int64](65532),
						},
						Containers: []corev1.Container{
							{
								Name:            "gardener-discovery-server",
								Image:           image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args: []string{
									"--tls-cert-file=/var/run/secrets/gardener.cloud/gardener-discovery-server/tls/tls.crt",
									"--tls-private-key-file=/var/run/secrets/gardener.cloud/gardener-discovery-server/tls/tls.key",
									"--kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
									"--workload-identity-openid-configuration-file=/etc/gardener-discovery-server/garden/workload-identity/openid-configuration.json",
									"--workload-identity-jwks-file=/etc/gardener-discovery-server/garden/workload-identity/jwks.json",
								},
								Resources: corev1.ResourceRequirements{
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceCPU:    resource.MustParse("50m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          "https",
										ContainerPort: 10443,
										Protocol:      corev1.ProtocolTCP,
									},
									{
										Name:          "metrics",
										ContainerPort: 8080,
										Protocol:      corev1.ProtocolTCP,
									},
									{
										Name:          "healthz",
										ContainerPort: 8081,
										Protocol:      corev1.ProtocolTCP,
									},
								},
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path:   "/healthz",
											Port:   intstr.FromString("healthz"),
											Scheme: corev1.URISchemeHTTP,
										},
									},
									InitialDelaySeconds: 15,
									TimeoutSeconds:      5,
									FailureThreshold:    3,
									SuccessThreshold:    1,
									PeriodSeconds:       20,
								},
								ReadinessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path:   "/readyz",
											Port:   intstr.FromString("healthz"),
											Scheme: corev1.URISchemeHTTP,
										},
									},
									InitialDelaySeconds: 5,
									TimeoutSeconds:      5,
									FailureThreshold:    3,
									SuccessThreshold:    1,
									PeriodSeconds:       10,
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "gardener-discovery-server-tls",
										MountPath: "/var/run/secrets/gardener.cloud/gardener-discovery-server/tls",
										ReadOnly:  true,
									},
									{
										Name:      "garden-workload-identity",
										MountPath: "/etc/gardener-discovery-server/garden/workload-identity",
										ReadOnly:  true,
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "gardener-discovery-server-tls",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName:  "gardener-discovery-server-tls",
										DefaultMode: ptr.To[int32](0400),
									},
								},
							},
							{
								Name: "garden-workload-identity",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName:  workloadIdentitySecret.GetName(),
										DefaultMode: ptr.To[int32](0400),
									},
								},
							},
						},
					},
				},
			},
		}

		utilruntime.Must(gardener.InjectGenericKubeconfig(deployment, "generic-token-kubeconfig", "shoot-access-gardener-discovery-server"))
		utilruntime.Must(references.InjectAnnotations(deployment))

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-discovery-server",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "discovery-server",
				},
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":8080}]`,
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Selector: map[string]string{
					"app":  "gardener",
					"role": "discovery-server",
				},
				Ports: []corev1.ServicePort{
					{
						Name:       "https",
						Port:       10443,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt32(10443),
					},
					{
						Name:       "metrics",
						Port:       8080,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt32(8080),
					},
					{
						Name:       "healthz",
						Port:       8081,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt32(8081),
					},
				},
				SessionAffinity: corev1.ServiceAffinityClientIP,
			},
		}
		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-discovery-server",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "discovery-server",
				},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: ptr.To(intstr.FromInt32(1)),
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "gardener",
					"role": "discovery-server",
				}},
				UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
			},
		}
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-discovery-server-vpa",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "discovery-server",
				},
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "gardener-discovery-server",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName:    "*",
							ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
							MinAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("10m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
						},
					},
				},
			},
		}

		ingress = &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-discovery-server",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "discovery-server",
				},
				Annotations: map[string]string{
					"nginx.ingress.kubernetes.io/ssl-passthrough": "true",
				},
			},
			Spec: networkingv1.IngressSpec{
				IngressClassName: ptr.To("nginx-ingress-gardener"),
				Rules: []networkingv1.IngressRule{{
					Host: "discovery.local.gardener.cloud",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "gardener-discovery-server",
										Port: networkingv1.ServiceBackendPort{Number: 10443},
									},
								},
								Path:     "/",
								PathType: ptr.To(networkingv1.PathTypePrefix),
							}},
						},
					}},
				},
			},
		}
		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "garden-gardener-discovery-server",
				Namespace: namespace,
				Labels:    map[string]string{"prometheus": "garden"},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "gardener",
					"role": "discovery-server",
				}},
				Endpoints: []monitoringv1.Endpoint{{
					Port: "metrics",
					RelabelConfigs: []monitoringv1.RelabelConfig{{
						Action: "labelmap",
						Regex:  `__meta_kubernetes_service_label_(.+)`,
					}},
				}},
			},
		}

		secretConfig = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-service-account-issuer",
				Namespace: "garden",
				Labels: map[string]string{
					"app":                 "gardener",
					"role":                "discovery-server",
					"gardener.cloud/role": "shoot-service-account-issuer",
				},
			},
			StringData: map[string]string{
				"hostname": "discovery.local.gardener.cloud",
			},
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:discovery-server",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "discovery-server",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"core.gardener.cloud"},
					Resources: []string{"projects", "shoots"},
					Verbs:     []string{"get", "watch", "list"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"configmaps", "namespaces"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:discovery-server",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "discovery-server",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:discovery-server",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "gardener-discovery-server",
				Namespace: "kube-system",
			}},
		}

		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:system:discovery-server",
				Namespace: "gardener-system-shoot-issuer",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "discovery-server",
				},
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "watch", "list"},
			}},
		}
		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:system:discovery-server",
				Namespace: "gardener-system-shoot-issuer",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "discovery-server",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "gardener.cloud:system:discovery-server",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "gardener-discovery-server",
				Namespace: "kube-system",
			}},
		}

		values = discoveryserver.Values{
			Image:                       image,
			Domain:                      "discovery.local.gardener.cloud",
			WorkloadIdentityTokenIssuer: workloadIdentityIssuer,
		}
		deployer = discoveryserver.New(fakeClient, namespace, fakeSecretManager, values)

		By("Create secrets managed outside of this package for which secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())

		workloadIdentityBundleSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-apiserver-workload-identity-signing-key-bundle",
				Namespace: namespace,
				Labels: map[string]string{
					"bundle-for":       "gardener-apiserver-workload-identity-signing-key",
					"managed-by":       "secrets-manager",
					"manager-identity": "gardener-operator",
					"name":             "gardener-apiserver-workload-identity-signing-key-bundle",
				},
			},
			Data: map[string][]byte{
				"bundle.key": workloadIdentityPrivateKey,
			},
		}
		Expect(fakeClient.Create(ctx, workloadIdentityBundleSecret)).To(Succeed())
	})

	Describe("#Deploy", func() {
		Context("resources generation", func() {
			var expectedRuntimeObjects, expectedVirtualObjects []client.Object

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
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())
			})

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
					Status: healthyManagedResourceStatus,
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
					Status: healthyManagedResourceStatus,
				}
				utilruntime.Must(references.InjectAnnotations(expectedVirtualMr))
				Expect(managedResourceVirtual).To(Equal(expectedVirtualMr))
				expectedRuntimeObjects = []client.Object{
					deployment,
					service,
					podDisruptionBudget,
					vpa,
					ingress,
					serviceMonitor,
					workloadIdentitySecret,
				}
				expectedVirtualObjects = []client.Object{
					clusterRole,
					clusterRoleBinding,
					role,
					roleBinding,
					secretConfig,
				}

				managedResourceSecretVirtual.Name = expectedVirtualMr.Spec.SecretRefs[0].Name
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(Succeed())
				Expect(managedResourceSecretRuntime.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecretRuntime.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecretRuntime.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

				Expect(managedResourceSecretVirtual.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecretVirtual.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecretVirtual.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
			})

			It("should successfully deploy all resources", func() {
				Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObjects...))
				Expect(managedResourceVirtual).To(consistOf(expectedVirtualObjects...))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(fakeClient.Create(ctx, managedResourceRuntime)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceVirtual)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecretRuntime)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecretVirtual)).To(Succeed())
			Expect(fakeClient.Create(ctx, virtualGardenAccessSecret)).To(Succeed())

			Expect(deployer.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(virtualGardenAccessSecret), virtualGardenAccessSecret)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		Describe("#Wait", func() {
			It("should fail because reading the runtime ManagedResource fails", func() {
				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the runtime and virtual ManagedResources are unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
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

			It("should fail because the runtime ManagedResource is unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
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

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
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

			It("should succeed because the runtime and virtual ManagedResource are healthy and progressing", func() {
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

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
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

				Expect(deployer.Wait(ctx)).To(Succeed())
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
		},
	}
)
