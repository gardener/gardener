// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

var _ = Describe("Seed Tests", Label("Seed", "default"), func() {
	Describe("Garden Cluster Access For Seed Components", Ordered, func() {
		var (
			s                *SeedContext
			seedNamespace    string
			gardenAccessName string
			accessSecret     *corev1.Secret
		)

		BeforeTestSetup(func() {
			testContext := NewTestContext()

			// Find the first seed which is not "e2e-managedseed". Seed name differs between test scenarios, e.g., non-ha/ha.
			// However, this test should not use "e2e-managedseed", because it is created and deleted in a separate e2e test.
			// This e2e test already includes tests for the "Renew gardenlet kubeconfig" functionality. Additionally,
			// it might be already gone before the kubeconfig was renewed.
			seedList := &gardencorev1beta1.SeedList{}
			if err := testContext.GardenClient.List(context.Background(), seedList); err != nil {
				testContext.Log.Error(err, "Failed to list seeds")
				Fail(err.Error())
			}

			seedIndex := slices.IndexFunc(seedList.Items, func(item gardencorev1beta1.Seed) bool {
				return item.Name != DefaultManagedSeedName()
			})

			if seedIndex == -1 {
				Fail("failed to find applicable seed")
			}

			s = testContext.ForSeed(&seedList.Items[seedIndex])

			seedNamespace = gardenerutils.ComputeGardenNamespace(s.Seed.Name)
			gardenAccessName = "test-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
			accessSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gardenAccessName,
					Namespace: v1beta1constants.GardenNamespace,
					Labels: map[string]string{
						resourcesv1alpha1.ResourceManagerPurpose: resourcesv1alpha1.LabelPurposeTokenRequest,
						resourcesv1alpha1.ResourceManagerClass:   resourcesv1alpha1.ResourceManagerClassGarden,
					},
					Annotations: map[string]string{
						resourcesv1alpha1.ServiceAccountName: gardenAccessName,
					},
				},
			}
		})

		It("Should create garden access secret", func(ctx SpecContext) {
			Eventually(ctx, func() error {
				if err := s.GardenClient.Create(ctx, accessSecret); !apierrors.IsAlreadyExists(err) {
					return err
				}
				return StopTrying("access secret already exists")
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		It("Should create RBAC role for service account", func(ctx SpecContext) {
			role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gardenAccessName,
					Namespace: seedNamespace,
				},
				Rules: []rbacv1.PolicyRule{{
					APIGroups:     []string{""},
					Resources:     []string{"serviceaccounts"},
					Verbs:         []string{"get"},
					ResourceNames: []string{gardenAccessName},
				}},
			}

			Eventually(ctx, func() error {
				if err := s.GardenClient.Create(ctx, role); !apierrors.IsAlreadyExists(err) {
					return err
				}
				return StopTrying("role already exists")
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		It("Should create RoleBinding for service account", func(ctx SpecContext) {
			roleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gardenAccessName,
					Namespace: seedNamespace,
				},
				Subjects: []rbacv1.Subject{{
					Kind: rbacv1.ServiceAccountKind,
					Name: gardenAccessName,
				}},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "Role",
					Name:     gardenAccessName,
				},
			}

			Eventually(ctx, func() error {
				if err := s.GardenClient.Create(ctx, roleBinding); !apierrors.IsAlreadyExists(err) {
					return err
				}
				return StopTrying("rolebinding already exists")
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		var accessSecretBefore *corev1.Secret
		It("Should wait for to be populated in garden access secret", func(ctx SpecContext) {
			Eventually(func(g Gomega) {
				g.Expect(s.GardenClient.Get(ctx, client.ObjectKeyFromObject(accessSecret), accessSecret)).To(Succeed())
				g.Expect(accessSecret.Data).To(HaveKeyWithValue(resourcesv1alpha1.DataKeyToken, Not(BeEmpty())))
				accessSecretBefore = accessSecret.DeepCopy()
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		It("Should initialize client from garden access secret", func(ctx SpecContext) {
			gardenAccessConfig := rest.CopyConfig(s.GardenClientSet.RESTConfig())

			// drop kind admin client certificate so that we can test other credentials
			gardenAccessConfig.CertData = nil
			gardenAccessConfig.KeyData = nil

			// use the requested token and create a client
			gardenAccessConfig.BearerToken = string(accessSecret.Data[resourcesv1alpha1.DataKeyToken])
			gardenAccessClient, err := client.New(gardenAccessConfig, client.Options{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				g.Expect(gardenAccessClient.Get(ctx, client.ObjectKey{Name: gardenAccessName, Namespace: seedNamespace}, &corev1.ServiceAccount{})).To(Succeed())
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		ItShouldAnnotateSeed(s, map[string]string{
			v1beta1constants.GardenerOperation: v1beta1constants.SeedOperationRenewGardenAccessSecrets,
		})

		ItShouldEventuallyNotHaveOperationAnnotation(s.GardenKomega, s.Seed)

		It("Should wait for token to be renewed in garden access secret", func(ctx SpecContext) {
			Eventually(func(g Gomega) {
				g.Expect(s.GardenClient.Get(ctx, client.ObjectKeyFromObject(accessSecret), accessSecret)).To(Succeed())
				g.Expect(accessSecret.Data).To(HaveKeyWithValue(resourcesv1alpha1.DataKeyToken, Not(Equal(accessSecretBefore.Data[resourcesv1alpha1.DataKeyToken]))))
				g.Expect(accessSecret.Annotations).To(HaveKeyWithValue(resourcesv1alpha1.ServiceAccountTokenRenewTimestamp, Not(Equal(accessSecretBefore.Annotations[resourcesv1alpha1.ServiceAccountTokenRenewTimestamp]))))
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		Describe("usage in provider-local", func() {
			It("should be allowed via seed authorizer to annotate its own seed", func(ctx SpecContext) {
				const testAnnotation = "provider-local-e2e-test-garden-access"

				Eventually(func(g Gomega) {
					g.Expect(s.GardenClient.Get(ctx, client.ObjectKeyFromObject(s.Seed), s.Seed)).To(Succeed())

					g.Expect(s.Seed.Annotations).To(HaveKey(testAnnotation))
					g.Expect(time.Parse(time.RFC3339, s.Seed.Annotations[testAnnotation])).
						Should(BeTemporally(">", s.Seed.CreationTimestamp.UTC()),
							"Timestamp in %s annotation on seed %s should be after creationTimestamp of seed", testAnnotation, s.Seed.Name)
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))
		})
	})
})
