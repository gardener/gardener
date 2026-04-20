// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/resourcemanager/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	. "github.com/gardener/gardener/pkg/component/shared"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ResourceManager", func() {
	var (
		ctx       = context.TODO()
		namespace string
		sm        secretsmanager.Interface
	)

	Describe("#New*GardenerResourceManager", func() {
		var fakeClient client.Client

		BeforeEach(func() {
			namespace = "fake-ns"
			fakeClient = fakeclient.NewClientBuilder().Build()
		})

		JustBeforeEach(func() {
			sm = fakesecretsmanager.New(fakeClient, namespace)
		})

		It("should apply the defaults for new runtime resource managers", func() {
			resourceManager, err := NewRuntimeGardenerResourceManager(fakeClient, namespace, sm, resourcemanager.Values{
				ClusterIdentity: ptr.To("foo"),
				ConcurrentSyncs: ptr.To(21),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resourceManager.GetValues()).To(Equal(resourcemanager.Values{
				ClusterIdentity:                   ptr.To("foo"),
				ConcurrentSyncs:                   ptr.To(21),
				HealthSyncPeriod:                  &metav1.Duration{Duration: time.Minute},
				Image:                             "europe-docker.pkg.dev/gardener-project/releases/gardener/resource-manager:v0.0.0-master+$Format:%H$",
				MaxConcurrentNetworkPolicyWorkers: ptr.To(20),
				NetworkPolicyControllerIngressControllerSelector: &resourcemanagerconfigv1alpha1.IngressControllerSelector{
					Namespace: "garden",
					PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{
						"app":       "nginx-ingress",
						"component": "controller",
					}},
				},
				PodTopologySpreadConstraintsEnabled: false,
				VPAInPlaceUpdatesEnabled:            false,
				Replicas:                            ptr.To[int32](2),
				ResourceClass:                       ptr.To("seed"),
				ResponsibilityMode:                  resourcemanager.ForRuntime,
			}))
		})

		It("should set SystemComponentsConfigWebhookEnabled in the values for runtime resource managers deployed to a self-hosted shoot cluster", func() {
			resourceManager, err := NewRuntimeGardenerResourceManager(fakeClient, namespace, sm, resourcemanager.Values{
				ClusterIdentity:                      ptr.To("foo"),
				SystemComponentsConfigWebhookEnabled: true,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resourceManager.GetValues()).To(Equal(resourcemanager.Values{
				ClusterIdentity:                      ptr.To("foo"),
				ConcurrentSyncs:                      ptr.To(20),
				SystemComponentsConfigWebhookEnabled: true,
				HealthSyncPeriod:                     &metav1.Duration{Duration: time.Minute},
				Image:                                "europe-docker.pkg.dev/gardener-project/releases/gardener/resource-manager:v0.0.0-master+$Format:%H$",
				MaxConcurrentNetworkPolicyWorkers:    ptr.To(20),
				NetworkPolicyControllerIngressControllerSelector: &resourcemanagerconfigv1alpha1.IngressControllerSelector{
					Namespace: "garden",
					PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{
						"app":       "nginx-ingress",
						"component": "controller",
					}},
				},
				PodTopologySpreadConstraintsEnabled: false,
				VPAInPlaceUpdatesEnabled:            false,
				Replicas:                            ptr.To[int32](2),
				ResourceClass:                       ptr.To("seed"),
				ResponsibilityMode:                  resourcemanager.ForRuntime,
			}))
		})

		It("should apply the defaults for new target resource managers", func() {
			resourceManager, err := NewTargetGardenerResourceManager(fakeClient, namespace, sm, resourcemanager.Values{
				ClusterIdentity:  ptr.To("foo"),
				TargetNamespaces: []string{},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resourceManager.GetValues()).To(Equal(resourcemanager.Values{
				AlwaysUpdate:                         ptr.To(true),
				ClusterIdentity:                      ptr.To("foo"),
				ConcurrentSyncs:                      ptr.To(20),
				HealthSyncPeriod:                     &metav1.Duration{Duration: time.Minute},
				Image:                                "europe-docker.pkg.dev/gardener-project/releases/gardener/resource-manager:v0.0.0-master+$Format:%H$",
				MaxConcurrentCSRApproverWorkers:      ptr.To(5),
				MaxConcurrentHealthWorkers:           ptr.To(10),
				MaxConcurrentTokenRequestorWorkers:   ptr.To(5),
				ResponsibilityMode:                   resourcemanager.ForShootOrVirtualGarden,
				TargetNamespaces:                     []string{},
				WatchedNamespace:                     &namespace,
				SystemComponentsConfigWebhookEnabled: true,
			}))
		})

		When("namespace is garden", func() {
			BeforeEach(func() {
				namespace = "garden"
			})

			It("should apply the defaults for new target resource managers", func() {
				resourceManager, err := NewTargetGardenerResourceManager(fakeClient, namespace, sm, resourcemanager.Values{
					ClusterIdentity:  ptr.To("foo"),
					TargetNamespaces: []string{},
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(resourceManager.GetValues()).To(Equal(resourcemanager.Values{
					AlwaysUpdate:                         ptr.To(true),
					ClusterIdentity:                      ptr.To("foo"),
					ConcurrentSyncs:                      ptr.To(20),
					HealthSyncPeriod:                     &metav1.Duration{Duration: time.Minute},
					Image:                                "europe-docker.pkg.dev/gardener-project/releases/gardener/resource-manager:v0.0.0-master+$Format:%H$",
					MaxConcurrentCSRApproverWorkers:      ptr.To(5),
					MaxConcurrentHealthWorkers:           ptr.To(10),
					MaxConcurrentTokenRequestorWorkers:   ptr.To(5),
					ResponsibilityMode:                   resourcemanager.ForShootOrVirtualGarden,
					TargetNamespaces:                     []string{},
					WatchedNamespace:                     &namespace,
					SystemComponentsConfigWebhookEnabled: false,
				}))
			})
		})
	})

	Describe("#DeployGardenerResourceManager", func() {
		var (
			resourceManager *fakeResourceManager
			fakeErr         = errors.New("fake err")

			fakeClient    client.Client
			k8sSeedClient kubernetes.Interface
			scheme        *runtime.Scheme

			setReplicas         func(ctx context.Context) (int32, error)
			getAPIServerAddress func() string

			bootstrapKubeconfigSecret *corev1.Secret
			shootAccessSecret         *corev1.Secret
			managedResource           *resourcesv1alpha1.ManagedResource
			now                       time.Time
		)

		BeforeEach(func() {
			namespace = "fake-ns"

			resourceManager = &fakeResourceManager{}

			now = time.Unix(60, 0)
			DeferCleanup(test.WithVar(&Now, now))

			scheme = runtime.NewScheme()
			Expect(kubernetesscheme.AddToScheme(scheme)).To(Succeed())
			Expect(resourcesv1alpha1.AddToScheme(scheme)).To(Succeed())

			fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme).Build()
			k8sSeedClient = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
			sm = fakesecretsmanager.New(fakeClient, namespace)

			setReplicas = func(_ context.Context) (int32, error) {
				return 2, nil
			}
			getAPIServerAddress = func() string { return "kube-apiserver" }

			By("Ensure secrets managed outside of this function for which secretsmanager.Get() will be called")
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())

			bootstrapKubeconfigSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-access-gardener-resource-manager-bootstrap-d9a4d56e",
					Namespace: namespace,
				},
			}
			shootAccessSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-access-gardener-resource-manager",
					Namespace: namespace,
					Annotations: map[string]string{
						"serviceaccount.resources.gardener.cloud/token-renew-timestamp": now.Add(time.Hour).Format(time.RFC3339),
					},
				},
			}
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-core-gardener-resource-manager",
					Namespace: namespace,
				},
			}

		})

		Context("w/o bootstrapping", func() {
			Context("deploy gardener-resource-manager", func() {
				BeforeEach(func() {
					Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())
					Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())
				})

				It("should set the secrets and deploy", func() {
					Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress)).To(Succeed())

					Expect(resourceManager.replicas).To(PointTo(Equal(int32(2))))
					Expect(resourceManager.secrets.BootstrapKubeconfig).To(BeNil())
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(bootstrapKubeconfigSecret), bootstrapKubeconfigSecret)).To(BeNotFoundError())
				})

				It("should fail when the deploy function fails", func() {
					resourceManager.deployError = fakeErr
					Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress)).To(MatchError(fakeErr))
				})
			})
		})

		Context("w/ bootstrapping", func() {
			Context("with success", func() {
				BeforeEach(func() {
					DeferCleanup(test.WithVar(&WaitUntilGardenerResourceManagerBootstrapped,
						func(_ context.Context, _ client.Client, _ string) error { return nil },
					))
				})

				AfterEach(func() {
					Expect(resourceManager.replicas).To(PointTo(Equal(int32(2))))
					Expect(resourceManager.secrets.BootstrapKubeconfig).To(BeNil())
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(bootstrapKubeconfigSecret), bootstrapKubeconfigSecret)).To(Succeed())
				})

				Context("deploy with 2 replicas", func() {
					It("bootstraps because the shoot access secret was not found", func() {
						Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress)).To(Succeed())
					})

					It("bootstraps because the shoot access secret was never reconciled", func() {
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())

						Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress)).To(Succeed())
					})

					It("bootstraps because the shoot access secret was not renewed", func() {
						shootAccessSecret.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": now.Add(-time.Hour).Format(time.RFC3339)}
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())
						Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

						Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress)).To(Succeed())
					})

					It("bootstraps because the managed resource was not found", func() {
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())

						Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress)).To(Succeed())
					})

					It("bootstraps because the managed resource indicates that the shoot access token lost access", func() {
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())

						managedResource.Status.ObservedGeneration = managedResource.Generation
						managedResource.Status.Conditions = []gardencorev1beta1.Condition{
							{Type: "ResourcesApplied", Status: gardencorev1beta1.ConditionFalse, Message: `forbidden: User "system:serviceaccount:kube-system:gardener-resource-manager" cannot do anything`},
							{Type: "ResourcesHealthy", Status: gardencorev1beta1.ConditionTrue},
						}
						Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

						Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress)).To(Succeed())
					})

					It("bootstraps because the managed resource indicates that the shoot access token was invalidated", func() {
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())

						managedResource.Status.ObservedGeneration = managedResource.Generation
						managedResource.Status.Conditions = []gardencorev1beta1.Condition{
							{Type: "ResourcesApplied", Status: gardencorev1beta1.ConditionFalse, Message: `failed to compute all HPA target ref object keys: failed to list all HPAs: Unauthorized`},
							{Type: "ResourcesHealthy", Status: gardencorev1beta1.ConditionTrue},
						}
						Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

						Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress)).To(Succeed())
					})
				})
			})

			Context("with failure", func() {
				It("fails because the bootstrap kubeconfig secret cannot be created", func() {
					fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme).
						WithObjects(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}}).
						WithInterceptorFuncs(interceptor.Funcs{
							Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
								if _, ok := obj.(*corev1.Secret); ok {
									return fakeErr
								}
								return client.Create(ctx, obj, opts...)
							},
						}).Build()
					k8sSeedClient = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
					sm = fakesecretsmanager.New(fakeClient, namespace)

					Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress)).To(MatchError(fakeErr))
				})

				Context("waiting for bootstrapping process", func() {
					BeforeEach(func() {
						DeferCleanup(test.WithVars(
							&TimeoutWaitForGardenerResourceManagerBootstrapping, 500*time.Millisecond,
							&IntervalWaitForGardenerResourceManagerBootstrapping, 5*time.Millisecond,
						))
					})

					It("fails because the shoot access token was not generated", func() {
						shootAccessSecret.Annotations = nil
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())

						Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress)).To(MatchError(ContainSubstring("token not yet generated")))
					})

					It("fails because the shoot access token renew timestamp cannot be parsed", func() {
						shootAccessSecret.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": "foo"}
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())

						Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress).Error()).To(ContainSubstring("could not parse renew timestamp"))
					})

					It("fails because the shoot access token was not renewed", func() {
						shootAccessSecret.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": now.Add(-time.Hour).Format(time.RFC3339)}
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())
						Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

						Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress).Error()).To(ContainSubstring("token not yet renewed"))
					})

					It("fails because the managed resource is not getting healthy", func() {
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())

						managedResource.Status.Conditions = []gardencorev1beta1.Condition{
							{Type: "ResourcesApplied", Status: gardencorev1beta1.ConditionFalse, Message: ": Unauthorized"},
						}
						Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

						Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress).Error()).To(ContainSubstring(fmt.Sprintf("managed resource %s/%s is not healthy", namespace, managedResource.Name)))
					})
				})
			})
		})
	})
})

type fakeResourceManager struct {
	replicas    *int32
	deployError error
	secrets     resourcemanager.Secrets
}

func (f *fakeResourceManager) GetReplicas() *int32                  { return f.replicas }
func (f *fakeResourceManager) SetReplicas(r *int32)                 { f.replicas = r }
func (f *fakeResourceManager) SetSecrets(s resourcemanager.Secrets) { f.secrets = s }
func (f *fakeResourceManager) GetValues() resourcemanager.Values    { return resourcemanager.Values{} }
func (f *fakeResourceManager) SetBootstrapControlPlaneNode(bool)    {}
func (f *fakeResourceManager) Deploy(_ context.Context) error {
	err := f.deployError
	f.deployError = nil
	return err
}
func (f *fakeResourceManager) Destroy(_ context.Context) error     { return nil }
func (f *fakeResourceManager) Wait(_ context.Context) error        { return nil }
func (f *fakeResourceManager) WaitCleanup(_ context.Context) error { return nil }
