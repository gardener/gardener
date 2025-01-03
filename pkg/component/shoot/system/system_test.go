// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package system_test

import (
	"context"
	"net"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/shoot/system"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ShootSystem", func() {
	var (
		ctx = context.Background()

		contain             func(...client.Object) types.GomegaMatcher
		managedResourceName = "shoot-core-system"
		namespace           = "some-namespace"

		projectName       = "foo"
		shootNamespace    = "garden-" + projectName
		shootName         = "bar"
		region            = "test-region"
		providerType      = "test-provider"
		kubernetesVersion = "1.31.1"
		maintenanceBegin  = "123456+0100"
		maintenanceEnd    = "134502+0100"
		domain            = "my-shoot.example.com"
		podCIDRs          = []net.IPNet{{IP: net.ParseIP("10.10.0.0"), Mask: net.CIDRMask(16, 32)}, {IP: net.ParseIP("2001:db8:1::"), Mask: net.CIDRMask(64, 128)}}
		serviceCIDRs      = []net.IPNet{{IP: net.ParseIP("11.11.0.0"), Mask: net.CIDRMask(16, 32)}, {IP: net.ParseIP("2001:db8:2::"), Mask: net.CIDRMask(64, 128)}}
		nodeCIDRs         = []net.IPNet{{IP: net.ParseIP("12.12.0.0"), Mask: net.CIDRMask(16, 32)}, {IP: net.ParseIP("2001:db8:3::"), Mask: net.CIDRMask(64, 128)}}
		extension1        = "some-extension"
		extension2        = "some-other-extension"
		shootObj          = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootNamespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: kubernetesVersion,
				},
				Provider: gardencorev1beta1.Provider{
					Type: providerType,
				},
				Maintenance: &gardencorev1beta1.Maintenance{
					TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{
						Begin: maintenanceBegin,
						End:   maintenanceEnd,
					},
				},
				Region: region,
			},
		}

		c         client.Client
		values    Values
		component Interface

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		contain = NewManagedResourceContainsObjectsMatcher(c)

		values = Values{
			Extensions:            []string{extension1, extension2},
			ExternalClusterDomain: &domain,
			IsWorkerless:          false,
			KubernetesVersion:     semver.MustParse(kubernetesVersion),
			Object:                shootObj,
			PodNetworkCIDRs:       podCIDRs,
			ProjectName:           projectName,
			ServiceNetworkCIDRs:   serviceCIDRs,
			NodeNetworkCIDRs:      nodeCIDRs,
		}
		component = New(c, namespace, values)

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		JustBeforeEach(func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

			component = New(c, namespace, values)
			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResource.Name,
					Namespace:       managedResource.Namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
		})

		Context("kube-controller-manager ServiceAccounts", func() {
			When("shoot is workerless", func() {
				BeforeEach(func() {
					values.IsWorkerless = true
				})

				It("should not deploy any ServiceAccounts", func() {
					for key := range managedResourceSecret.Data {
						Expect(key).NotTo(HavePrefix("serviceaccount_"), key)
					}
				})
			})

			var (
				serviceAccount = func(name string) *corev1.ServiceAccount {
					return &corev1.ServiceAccount{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: "kube-system",
							Annotations: map[string]string{
								"resources.gardener.cloud/keep-object": "true",
							},
						},
						AutomountServiceAccountToken: ptr.To(false),
					}
				}

				defaultKCMControllerSANames = []string{"attachdetach-controller",
					"bootstrap-signer",
					"certificate-controller",
					"clusterrole-aggregation-controller",
					"controller-discovery",
					"cronjob-controller",
					"daemon-set-controller",
					"deployment-controller",
					"disruption-controller",
					"endpoint-controller",
					"endpointslice-controller",
					"expand-controller",
					"generic-garbage-collector",
					"horizontal-pod-autoscaler",
					"job-controller",
					"metadata-informers",
					"namespace-controller",
					"persistent-volume-binder",
					"pod-garbage-collector",
					"pv-protection-controller",
					"pvc-protection-controller",
					"replicaset-controller",
					"replication-controller",
					"resourcequota-controller",
					"root-ca-cert-publisher",
					"service-account-controller",
					"shared-informers",
					"statefulset-controller",
					"token-cleaner",
					"tokens-controller",
					"ttl-after-finished-controller",
					"ttl-controller",
				}
			)

			Context("k8s >= 1.27", func() {
				BeforeEach(func() {
					values.KubernetesVersion = semver.MustParse("1.27.4")
				})

				It("should successfully deploy all resources", func() {
					expectedServiceAccounts := make([]client.Object, 0, len(defaultKCMControllerSANames)+8)
					for _, name := range append(defaultKCMControllerSANames, "default", "endpointslicemirroring-controller", "ephemeral-volume-controller", "storage-version-garbage-collector", "service-controller", "route-controller", "node-controller", "resource-claim-controller") {
						expectedServiceAccounts = append(expectedServiceAccounts, serviceAccount(name))
					}

					Expect(managedResource).To(contain(expectedServiceAccounts...))
				})
			})

			Context("k8s >= 1.28", func() {
				BeforeEach(func() {
					values.KubernetesVersion = semver.MustParse("1.28.2")
				})

				It("should successfully deploy all resources", func() {
					expectedServiceAccounts := make([]client.Object, 0, len(defaultKCMControllerSANames)+10)
					for _, name := range append(defaultKCMControllerSANames, "default", "endpointslicemirroring-controller", "ephemeral-volume-controller", "storage-version-garbage-collector", "service-controller", "route-controller", "node-controller", "resource-claim-controller", "legacy-service-account-token-cleaner", "validatingadmissionpolicy-status-controller") {
						expectedServiceAccounts = append(expectedServiceAccounts, serviceAccount(name))
					}

					Expect(managedResource).To(contain(expectedServiceAccounts...))
				})
			})

			Context("k8s >= 1.29", func() {
				BeforeEach(func() {
					values.KubernetesVersion = semver.MustParse("1.29.1")
				})

				It("should successfully deploy all resources", func() {
					expectedServiceAccounts := make([]client.Object, 0, len(defaultKCMControllerSANames)+10)
					for _, name := range append(defaultKCMControllerSANames, "default", "endpointslicemirroring-controller", "ephemeral-volume-controller", "storage-version-garbage-collector", "service-controller", "route-controller", "node-controller", "resource-claim-controller", "legacy-service-account-token-cleaner", "service-cidrs-controller") {
						expectedServiceAccounts = append(expectedServiceAccounts, serviceAccount(name))
					}

					Expect(managedResource).To(contain(expectedServiceAccounts...))
				})
			})
		})

		Context("shoot-info ConfigMap", func() {
			When("shoot is workerless", func() {
				BeforeEach(func() {
					values.IsWorkerless = true
				})

				It("should not deploy any ConfigMap", func() {
					manifests, err := test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
					Expect(err).NotTo(HaveOccurred())

					for _, manifest := range manifests {
						Expect(manifest).NotTo(And(ContainSubstring("name: shoot-info"), ContainSubstring("kind: ConfigMap")))
					}
				})
			})

			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-info",
					Namespace: "kube-system",
				},
				Data: map[string]string{
					"domain":            domain,
					"extensions":        extension1 + `,` + extension2,
					"kubernetesVersion": kubernetesVersion,
					"maintenanceBegin":  maintenanceBegin,
					"maintenanceEnd":    maintenanceEnd,
					"nodeNetwork":       nodeCIDRs[0].String(),
					"nodeNetworks":      nodeCIDRs[0].String() + "," + nodeCIDRs[1].String(),
					"podNetwork":        podCIDRs[0].String(),
					"podNetworks":       podCIDRs[0].String() + "," + podCIDRs[1].String(),
					"projectName":       projectName,
					"provider":          providerType,
					"region":            region,
					"serviceNetwork":    serviceCIDRs[0].String(),
					"serviceNetworks":   serviceCIDRs[0].String() + "," + serviceCIDRs[1].String(),
					"shootName":         shootName,
				},
			}

			It("should successfully deploy all resources", func() {
				Expect(managedResource).To(contain(configMap))
			})
		})

		Context("PriorityClasses", func() {
			When("shoot is workerless", func() {
				BeforeEach(func() {
					values.IsWorkerless = true
				})

				It("should not deploy any PriorityClasses", func() {
					manifests, err := test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
					Expect(err).NotTo(HaveOccurred())

					for _, manifest := range manifests {
						Expect(manifest).NotTo(ContainSubstring("kind: PriorityClass"))
					}
				})
			})

			It("should successfully deploy all well-known PriorityClasses", func() {
				expectPriorityClasses(managedResource, contain)
			})
		})

		Context("NetworkPolicies", func() {
			When("shoot is workerless", func() {
				BeforeEach(func() {
					values.IsWorkerless = true
				})

				It("should not deploy any NetworkPolicies", func() {
					for key := range managedResourceSecret.Data {
						Expect(key).NotTo(HavePrefix("networkpolicy_"), key)
					}
				})
			})

			var (
				networkPolicyToAPIServer = &networkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gardener.cloud--allow-to-apiserver",
						Namespace: "kube-system",
						Annotations: map[string]string{
							"gardener.cloud/description": "Allows traffic to the API server in TCP port 443 for pods labeled with 'networking.gardener.cloud/to-apiserver=allowed'.",
						},
					},
					Spec: networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"networking.gardener.cloud/to-apiserver": "allowed",
							},
						},
						Egress: []networkingv1.NetworkPolicyEgressRule{
							{
								Ports: []networkingv1.NetworkPolicyPort{
									{
										Protocol: ptr.To(corev1.ProtocolTCP),
										Port:     ptr.To(intstr.FromInt32(443)),
									},
								},
							},
						},
						PolicyTypes: []networkingv1.PolicyType{
							networkingv1.PolicyTypeEgress,
						},
					},
				}

				networkPolicyToDNS = &networkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gardener.cloud--allow-to-dns",
						Namespace: "kube-system",
						Annotations: map[string]string{
							"gardener.cloud/description": "Allows egress traffic from pods labeled with 'networking.gardener.cloud/to-dns=allowed' to DNS running in the 'kube-system' namespace.",
						},
					},
					Spec: networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"networking.gardener.cloud/to-dns": "allowed",
							},
						},
						Egress: []networkingv1.NetworkPolicyEgressRule{
							{
								Ports: []networkingv1.NetworkPolicyPort{
									{
										Protocol: ptr.To(corev1.ProtocolUDP),
										Port:     ptr.To(intstr.FromInt32(8053)),
									},
									{
										Protocol: ptr.To(corev1.ProtocolTCP),
										Port:     ptr.To(intstr.FromInt32(8053)),
									},
								},
								To: []networkingv1.NetworkPolicyPeer{
									{
										PodSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      "k8s-app",
													Operator: metav1.LabelSelectorOpIn,
													Values:   []string{"kube-dns"},
												},
											},
										},
									},
								},
							},
							{
								Ports: []networkingv1.NetworkPolicyPort{
									{
										Protocol: ptr.To(corev1.ProtocolUDP),
										Port:     ptr.To(intstr.FromInt32(53)),
									},
									{
										Protocol: ptr.To(corev1.ProtocolTCP),
										Port:     ptr.To(intstr.FromInt32(53)),
									},
								},
								To: []networkingv1.NetworkPolicyPeer{
									{
										IPBlock: &networkingv1.IPBlock{
											CIDR: "0.0.0.0/0",
										},
									},
									{
										IPBlock: &networkingv1.IPBlock{
											CIDR: "::/0",
										},
									},
									{
										PodSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      "k8s-app",
													Operator: metav1.LabelSelectorOpIn,
													Values:   []string{"node-local-dns"},
												},
											},
										},
									},
								},
							},
						},
						PolicyTypes: []networkingv1.PolicyType{
							networkingv1.PolicyTypeEgress,
						},
					},
				}

				networkPolicyToKubelet = &networkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gardener.cloud--allow-to-kubelet",
						Namespace: "kube-system",
						Annotations: map[string]string{
							"gardener.cloud/description": "Allows egress traffic to kubelet in TCP port 10250 for pods labeled with 'networking.gardener.cloud/to-kubelet=allowed'.",
						},
					},
					Spec: networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"networking.gardener.cloud/to-kubelet": "allowed",
							},
						},
						Egress: []networkingv1.NetworkPolicyEgressRule{
							{
								Ports: []networkingv1.NetworkPolicyPort{
									{
										Protocol: ptr.To(corev1.ProtocolTCP),
										Port:     ptr.To(intstr.FromInt32(10250)),
									},
								},
							},
						},
						PolicyTypes: []networkingv1.PolicyType{
							networkingv1.PolicyTypeEgress,
						},
					},
				}

				networkPolicyToPublicNetworks = &networkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gardener.cloud--allow-to-public-networks",
						Namespace: "kube-system",
						Annotations: map[string]string{
							"gardener.cloud/description": "Allows egress traffic to all networks for pods labeled with 'networking.gardener.cloud/to-public-networks=allowed'.",
						},
					},
					Spec: networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"networking.gardener.cloud/to-public-networks": "allowed",
							},
						},
						Egress: []networkingv1.NetworkPolicyEgressRule{
							{
								To: []networkingv1.NetworkPolicyPeer{
									{
										IPBlock: &networkingv1.IPBlock{
											CIDR: "0.0.0.0/0",
										},
									},
									{
										IPBlock: &networkingv1.IPBlock{
											CIDR: "::/0",
										},
									},
								},
							},
						},
						PolicyTypes: []networkingv1.PolicyType{
							networkingv1.PolicyTypeEgress,
						},
					},
				}
			)

			It("should successfully deploy all resources", func() {
				Expect(managedResource).To(contain(
					networkPolicyToAPIServer,
					networkPolicyToDNS,
					networkPolicyToKubelet,
					networkPolicyToPublicNetworks,
				))
			})
		})

		Context("Read-Only resources", func() {
			It("should do nothing when the API resource list is unset", func() {
				manifests, err := test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
				Expect(err).NotTo(HaveOccurred())

				for _, manifest := range manifests {
					Expect(manifest).NotTo(And(ContainSubstring("name: gardener.cloud:system:read-only"), ContainSubstring("kind: ClusterRole")))
				}
			})

			When("API resource list is set", func() {
				BeforeEach(func() {
					values.APIResourceList = []*metav1.APIResourceList{
						{
							GroupVersion: "foo/v1",
							APIResources: []metav1.APIResource{
								{Name: "bar", Verbs: metav1.Verbs{"create", "delete"}},
								{Name: "baz", Verbs: metav1.Verbs{"get", "list", "watch"}},
								{Name: "dash", Verbs: metav1.Verbs{"get", "list", "watch"}},
							},
						},
						{
							GroupVersion: "v1",
							APIResources: []metav1.APIResource{
								{Name: "secrets", Verbs: metav1.Verbs{"get", "list", "watch"}},
								{Name: "configmaps", Verbs: metav1.Verbs{"get", "list", "watch"}},
								{Name: "services", Verbs: metav1.Verbs{"get", "list", "watch"}},
								{Name: "pods", Verbs: metav1.Verbs{"get", "list", "watch"}},
								{Name: "nodes", Verbs: metav1.Verbs{"get", "list", "watch"}},
							},
						},
						{
							GroupVersion: "bar/v1beta1",
							APIResources: []metav1.APIResource{
								{Name: "foo", Verbs: metav1.Verbs{"get", "list", "watch"}},
								{Name: "baz", Verbs: metav1.Verbs{"get", "list", "watch"}},
							},
						},
						{
							GroupVersion: "fancyoperator.io/v1alpha1",
							APIResources: []metav1.APIResource{
								{Name: "fancyresource1", Verbs: metav1.Verbs{"get", "list", "watch"}},
								{Name: "fancyresource2", Verbs: metav1.Verbs{"get", "list", "watch"}},
							},
						},
						{
							GroupVersion: "apps/v1",
							APIResources: []metav1.APIResource{
								{Name: "deployments", Verbs: metav1.Verbs{"get", "list", "watch"}},
								{Name: "statefulsets", Verbs: metav1.Verbs{"get", "list", "watch"}},
							},
						},
					}

					values.EncryptedResources = []string{
						"secrets",
						"services",
						"statefulsets.apps",
						"fancyresource1.fancyoperator.io",
						"dash.foo",
					}
				})

				It("should successfully deploy the related RBAC resources", func() {
					clusterRole := &rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: "gardener.cloud:system:read-only",
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{"configmaps", "nodes", "pods", "pods/log"},
								Verbs:     []string{"get", "list", "watch"},
							},
							{
								APIGroups: []string{"apps"},
								Resources: []string{"deployments"},
								Verbs:     []string{"get", "list", "watch"},
							},
							{
								APIGroups: []string{"bar"},
								Resources: []string{"baz", "foo"},
								Verbs:     []string{"get", "list", "watch"},
							},
							{
								APIGroups: []string{"fancyoperator.io"},
								Resources: []string{"fancyresource2"},
								Verbs:     []string{"get", "list", "watch"},
							},
							{
								APIGroups: []string{"foo"},
								Resources: []string{"baz"},
								Verbs:     []string{"get", "list", "watch"},
							},
						},
					}

					clusterRoleBinding := &rbacv1.ClusterRoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name: "gardener.cloud:system:read-only",
							Annotations: map[string]string{
								"resources.gardener.cloud/delete-on-invalid-update": "true",
							},
						},
						RoleRef: rbacv1.RoleRef{
							APIGroup: "rbac.authorization.k8s.io",
							Kind:     "ClusterRole",
							Name:     "gardener.cloud:system:read-only",
						},
						Subjects: []rbacv1.Subject{
							{
								Kind: "Group",
								Name: "gardener.cloud:system:viewers",
							},
						},
					}

					Expect(managedResource).To(contain(clusterRole, clusterRoleBinding))
				})
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		var (
			fakeOps   *retryfake.Ops
			resetVars func()
		)

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			resetVars = test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			)
		})

		AfterEach(func() {
			resetVars()
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource doesn't become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
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
					},
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
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
						},
					},
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(component.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

func expectPriorityClasses(mr *resourcesv1alpha1.ManagedResource, contain func(...client.Object) types.GomegaMatcher) {
	expected := []struct {
		name        string
		value       int32
		description string
	}{
		{"gardener-shoot-system-900", 999999900, "PriorityClass for Shoot system components"},
		{"gardener-shoot-system-800", 999999800, "PriorityClass for Shoot system components"},
		{"gardener-shoot-system-700", 999999700, "PriorityClass for Shoot system components"},
		{"gardener-shoot-system-600", 999999600, "PriorityClass for Shoot system components"},
	}

	expectedPriorityClasses := make([]client.Object, 0, len(expected))
	for _, pc := range expected {
		expectedPriorityClasses = append(expectedPriorityClasses, &schedulingv1.PriorityClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: pc.name,
			},
			Description: pc.description,
			Value:       pc.value,
		})
	}

	ExpectWithOffset(1, mr).To(contain(expectedPriorityClasses...))
}
