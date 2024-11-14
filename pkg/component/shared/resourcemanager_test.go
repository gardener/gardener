// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	mockresourcemanager "github.com/gardener/gardener/pkg/component/gardener/resourcemanager/mock"
	. "github.com/gardener/gardener/pkg/component/shared"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("ResourceManager", func() {
	var (
		ctrl *gomock.Controller
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployGardenerResourceManager", func() {
		var (
			resourceManager *mockresourcemanager.MockInterface
			secrets         resourcemanager.Secrets

			ctx       = context.TODO()
			fakeErr   = errors.New("fake err")
			namespace = "fake-ns"

			c             *mockclient.MockClient
			k8sSeedClient kubernetes.Interface
			sm            secretsmanager.Interface

			setReplicas         func(ctx context.Context) (int32, error)
			getAPIServerAddress func() string

			bootstrapKubeconfigSecret *corev1.Secret
			shootAccessSecret         *corev1.Secret
			managedResource           *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			resourceManager = mockresourcemanager.NewMockInterface(ctrl)

			c = mockclient.NewMockClient(ctrl)
			k8sSeedClient = kubernetesfake.NewClientSetBuilder().WithClient(c).Build()
			sm = fakesecretsmanager.New(c, namespace)

			setReplicas = func(_ context.Context) (int32, error) {
				return 2, nil
			}
			getAPIServerAddress = func() string { return "kube-apiserver" }

			By("Ensure secrets managed outside of this function for which secretsmanager.Get() will be called")
			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: "ca"}, gomock.AssignableToTypeOf(&corev1.Secret{})).AnyTimes()

			secrets = resourcemanager.Secrets{}

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
						"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(time.Hour).Format(time.RFC3339),
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
					gomock.InOrder(
						resourceManager.EXPECT().GetReplicas(),
						resourceManager.EXPECT().SetReplicas(ptr.To[int32](2)),
						resourceManager.EXPECT().GetReplicas().Return(ptr.To[int32](2)),

						// ensure bootstrapping prerequisites are not met
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(time.Hour).Format(time.RFC3339)}
							return nil
						}),
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(managedResource), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),

						// set secrets
						resourceManager.EXPECT().SetSecrets(secrets),
					)
				})

				It("should set the secrets and deploy", func() {
					resourceManager.EXPECT().Deploy(ctx)
					Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress)).To(Succeed())
				})

				It("should fail when the deploy function fails", func() {
					resourceManager.EXPECT().Deploy(ctx).Return(fakeErr)
					Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress)).To(MatchError(fakeErr))
				})
			})
		})

		Context("w/ bootstrapping", func() {
			Context("with success", func() {
				AfterEach(func() {
					defer test.WithVar(&TimeoutWaitForGardenerResourceManagerBootstrapping, time.Second)()

					gomock.InOrder(
						// create bootstrap kubeconfig
						c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, s *corev1.Secret, _ ...client.CreateOption) error {
							Expect(s.Data["kubeconfig"]).NotTo(BeNil())
							return nil
						}),

						// set secrets and deploy with bootstrap kubeconfig
						resourceManager.EXPECT().SetSecrets(&secretMatcher{
							bootstrapKubeconfigName: &bootstrapKubeconfigSecret.Name,
						}),
						resourceManager.EXPECT().Deploy(ctx),

						// wait for shoot access secret to be reconciled and managed resource to be healthy
						c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(time.Hour).Format(time.RFC3339)}
							return nil
						}),
						c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(managedResource), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *resourcesv1alpha1.ManagedResource, _ ...client.GetOption) error {
							obj.Status.ObservedGeneration = obj.Generation
							obj.Status.Conditions = []gardencorev1beta1.Condition{
								{Type: "ResourcesApplied", Status: gardencorev1beta1.ConditionTrue},
								{Type: "ResourcesHealthy", Status: gardencorev1beta1.ConditionTrue},
							}
							return nil
						}),

						// delete bootstrap kubeconfig
						c.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, obj *corev1.Secret, _ ...client.DeleteOption) error {
							Expect(obj.Name).To(Equal(bootstrapKubeconfigSecret.Name))
							Expect(obj.Namespace).To(Equal(bootstrapKubeconfigSecret.Namespace))
							return nil
						}),

						// set secrets and deploy with shoot access token
						resourceManager.EXPECT().SetSecrets(secrets),
						resourceManager.EXPECT().Deploy(ctx),
					)

					Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress)).To(Succeed())
				})

				tests := func() {
					It("bootstraps because the shoot access secret was not found", func() {
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
					})

					It("bootstraps because the shoot access secret was never reconciled", func() {
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{}))
					})

					It("bootstraps because the shoot access secret was not renewed", func() {
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(-time.Hour).Format(time.RFC3339)}
							return nil
						})
					})

					It("bootstraps because the managed resource was not found", func() {
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(time.Hour).Format(time.RFC3339)}
							return nil
						})
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(managedResource), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
					})

					It("bootstraps because the managed resource indicates that the shoot access token lost access", func() {
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(time.Hour).Format(time.RFC3339)}
							return nil
						})
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(managedResource), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *resourcesv1alpha1.ManagedResource, _ ...client.GetOption) error {
							obj.Status.ObservedGeneration = obj.Generation
							obj.Status.Conditions = []gardencorev1beta1.Condition{
								{Type: "ResourcesApplied", Status: gardencorev1beta1.ConditionFalse, Message: `forbidden: User "system:serviceaccount:kube-system:gardener-resource-manager" cannot do anything`},
								{Type: "ResourcesHealthy", Status: gardencorev1beta1.ConditionTrue},
							}
							return nil
						})
					})

					It("bootstraps because the managed resource indicates that the shoot access token was invalidated", func() {
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(time.Hour).Format(time.RFC3339)}
							return nil
						})
						c.EXPECT().Get(ctx, client.ObjectKeyFromObject(managedResource), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *resourcesv1alpha1.ManagedResource, _ ...client.GetOption) error {
							obj.Status.ObservedGeneration = obj.Generation
							obj.Status.Conditions = []gardencorev1beta1.Condition{
								{Type: "ResourcesApplied", Status: gardencorev1beta1.ConditionFalse, Message: `failed to compute all HPA target ref object keys: failed to list all HPAs: Unauthorized`},
								{Type: "ResourcesHealthy", Status: gardencorev1beta1.ConditionTrue},
							}
							return nil
						})
					})
				}

				Context("deploy with 2 replicas", func() {
					BeforeEach(func() {
						gomock.InOrder(
							resourceManager.EXPECT().GetReplicas(),
							resourceManager.EXPECT().SetReplicas(ptr.To[int32](2)),
							resourceManager.EXPECT().GetReplicas().Return(ptr.To[int32](2)),
						)
					})

					tests()
				})
			})

			Context("with failure", func() {
				BeforeEach(func() {
					// ensure bootstrapping preconditions are met
					resourceManager.EXPECT().GetReplicas().Return(ptr.To[int32](3)).Times(2)
					c.EXPECT().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
				})

				It("fails because the bootstrap kubeconfig secret cannot be created", func() {
					gomock.InOrder(
						c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
					)

					Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress)).To(MatchError(fakeErr))
				})

				Context("waiting for bootstrapping process", func() {
					BeforeEach(func() {
						gomock.InOrder(
							// create bootstrap kubeconfig
							c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),

							// set secrets and deploy with bootstrap kubeconfig
							resourceManager.EXPECT().SetSecrets(&secretMatcher{
								bootstrapKubeconfigName: &bootstrapKubeconfigSecret.Name,
							}),
							resourceManager.EXPECT().Deploy(ctx),
						)
					})

					It("fails because the shoot access token was not generated", func() {
						defer test.WithVar(&TimeoutWaitForGardenerResourceManagerBootstrapping, time.Millisecond)()

						c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
							obj.Annotations = nil
							return nil
						})

						Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress)).To(MatchError(ContainSubstring("token not yet generated")))
					})

					It("fails because the shoot access token renew timestamp cannot be parsed", func() {
						defer test.WithVar(&TimeoutWaitForGardenerResourceManagerBootstrapping, time.Millisecond)()

						c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": "foo"}
							return nil
						})

						Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress).Error()).To(ContainSubstring("could not parse renew timestamp"))
					})

					It("fails because the shoot access token was not renewed", func() {
						defer test.WithVar(&TimeoutWaitForGardenerResourceManagerBootstrapping, time.Millisecond)()

						c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(-time.Hour).Format(time.RFC3339)}
							return nil
						})

						Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress).Error()).To(ContainSubstring("token not yet renewed"))
					})

					It("fails because the managed resource is not getting healthy", func() {
						defer test.WithVar(&TimeoutWaitForGardenerResourceManagerBootstrapping, time.Millisecond)()

						gomock.InOrder(
							c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
								obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(time.Hour).Format(time.RFC3339)}
								return nil
							}),
							c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(managedResource), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *resourcesv1alpha1.ManagedResource, _ ...client.GetOption) error {
								obj.Status.ObservedGeneration = -1
								return nil
							}),
						)

						Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress).Error()).To(ContainSubstring(fmt.Sprintf("managed resource %s/%s is not healthy", namespace, managedResource.Name)))
					})
				})

				It("fails because the bootstrap kubeconfig cannot be deleted", func() {
					gomock.InOrder(
						// create bootstrap kubeconfig
						c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, s *corev1.Secret, _ ...client.CreateOption) error {
							Expect(s.Data["kubeconfig"]).NotTo(BeNil())
							return nil
						}),

						// set secrets and deploy with bootstrap kubeconfig
						resourceManager.EXPECT().SetSecrets(&secretMatcher{
							bootstrapKubeconfigName: &bootstrapKubeconfigSecret.Name,
						}),
						resourceManager.EXPECT().Deploy(ctx),

						// wait for shoot access secret to be reconciled and managed resource to be healthy
						c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(shootAccessSecret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
							obj.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": time.Now().Add(time.Hour).Format(time.RFC3339)}
							return nil
						}),
						c.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(managedResource), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *resourcesv1alpha1.ManagedResource, _ ...client.GetOption) error {
							obj.Status.ObservedGeneration = obj.Generation
							obj.Status.Conditions = []gardencorev1beta1.Condition{
								{Type: "ResourcesApplied", Status: gardencorev1beta1.ConditionTrue},
								{Type: "ResourcesHealthy", Status: gardencorev1beta1.ConditionTrue},
							}
							return nil
						}),

						// delete bootstrap kubeconfig
						c.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, obj *corev1.Secret, _ ...client.DeleteOption) error {
							Expect(obj.Name).To(Equal(bootstrapKubeconfigSecret.Name))
							Expect(obj.Namespace).To(Equal(bootstrapKubeconfigSecret.Namespace))
							return fakeErr
						}),
					)

					Expect(DeployGardenerResourceManager(ctx, k8sSeedClient.Client(), sm, resourceManager, namespace, setReplicas, getAPIServerAddress)).To(MatchError(fakeErr))
				})
			})
		})
	})
})

type secretMatcher struct {
	bootstrapKubeconfigName *string
}

func (m *secretMatcher) Matches(x any) bool {
	req, ok := x.(resourcemanager.Secrets)
	if !ok {
		return false
	}

	if m.bootstrapKubeconfigName != nil && (req.BootstrapKubeconfig == nil || req.BootstrapKubeconfig.Name != *m.bootstrapKubeconfigName) {
		return false
	}

	return true
}

func (m *secretMatcher) String() string {
	return fmt.Sprintf(`Secret Matcher:
bootstrapKubeconfigName: %v,
`, m.bootstrapKubeconfigName)
}
