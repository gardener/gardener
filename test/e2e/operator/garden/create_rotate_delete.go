// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/operator/garden/internal/rotation"
	rotationutils "github.com/gardener/gardener/test/utils/rotation"
)

var _ = Describe("Garden Tests", Label("Garden", "default"), func() {
	Describe("Create Garden, Rotate Credentials and Delete Garden", Ordered, Label("credentials-rotation"), func() {
		var s *GardenContext

		BeforeTestSetup(func() {
			backupSecret := defaultBackupSecret()
			s = NewTestContext().ForGarden(defaultGarden(backupSecret, false), backupSecret)
		})

		ItShouldCreateGarden(s)
		ItShouldWaitForGardenToBeReconciledAndHealthy(s)

		v := rotationutils.Verifiers{
			// basic verifiers checking secrets
			&rotation.CAVerifier{RuntimeClient: s.GardenClient, Garden: s.Garden},
			&rotationutils.ObservabilityVerifier{
				GetObservabilitySecretFunc: func(ctx context.Context) (*corev1.Secret, error) {
					secretList := &corev1.SecretList{}
					if err := s.GardenClient.List(ctx, secretList, client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{
						"managed-by":       "secrets-manager",
						"manager-identity": "gardener-operator",
						"name":             "observability-ingress",
					}); err != nil {
						return nil, err
					}

					if length := len(secretList.Items); length != 1 {
						return nil, fmt.Errorf("expect exactly one secret, found %d", length)
					}

					return &secretList.Items[0], nil
				},
				GetObservabilityEndpoint: func(_ *corev1.Secret) string {
					return "https://plutono-garden." + s.Garden.Spec.RuntimeCluster.Ingress.Domains[0].Name
				},
				GetObservabilityRotation: func() *gardencorev1beta1.ObservabilityRotation {
					return s.Garden.Status.Credentials.Rotation.Observability
				},
			},
			&rotationutils.ETCDEncryptionKeyVerifier{
				GetETCDSecretNamespace: func() string {
					return v1beta1constants.GardenNamespace
				},
				GetRuntimeClient: func() client.Client {
					return s.GardenClient
				},
				SecretsManagerLabelSelector: rotation.ManagedByGardenerOperatorSecretsManager,
				GetETCDEncryptionKeyRotation: func() *gardencorev1beta1.ETCDEncryptionKeyRotation {
					return s.Garden.Status.Credentials.Rotation.ETCDEncryptionKey
				},
				EncryptionKey:  v1beta1constants.SecretNameETCDEncryptionKey,
				RoleLabelValue: v1beta1constants.SecretNamePrefixETCDEncryptionConfiguration,
			},
			&rotationutils.ETCDEncryptionKeyVerifier{
				GetETCDSecretNamespace: func() string {
					return v1beta1constants.GardenNamespace
				},
				GetRuntimeClient: func() client.Client {
					return s.GardenClient
				},
				SecretsManagerLabelSelector: rotation.ManagedByGardenerOperatorSecretsManager,
				GetETCDEncryptionKeyRotation: func() *gardencorev1beta1.ETCDEncryptionKeyRotation {
					return s.Garden.Status.Credentials.Rotation.ETCDEncryptionKey
				},
				EncryptionKey:  v1beta1constants.SecretNameGardenerETCDEncryptionKey,
				RoleLabelValue: v1beta1constants.SecretNamePrefixGardenerETCDEncryptionConfiguration,
			},
			&rotationutils.ServiceAccountKeyVerifier{
				GetServiceAccountKeySecretNamespace: func() string {
					return v1beta1constants.GardenNamespace
				},
				GetRuntimeClient: func() client.Client {
					return s.GardenClient
				},
				SecretsManagerLabelSelector: rotation.ManagedByGardenerOperatorSecretsManager,
				GetServiceAccountKeyRotation: func() *gardencorev1beta1.ServiceAccountKeyRotation {
					return s.Garden.Status.Credentials.Rotation.ServiceAccountKey
				},
			},

			// advanced verifiers testing things from the user's perspective
			&rotationutils.EncryptedDataVerifier{
				NewTargetClientFunc: func(ctx context.Context) (kubernetes.Interface, error) {
					return kubernetes.NewClientFromSecret(ctx, s.GardenClient, v1beta1constants.GardenNamespace, "gardener",
						kubernetes.WithDisabledCachedClient(),
						kubernetes.WithClientOptions(client.Options{Scheme: operatorclient.VirtualScheme}),
					)
				},
				Resources: []rotationutils.EncryptedResource{
					{
						NewObject: func() client.Object {
							return &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{GenerateName: "test-foo-", Namespace: "default"},
								StringData: map[string]string{"content": "foo"},
							}
						},
						NewEmptyList: func() client.ObjectList { return &corev1.SecretList{} },
					},
					{
						NewObject: func() client.Object {
							return &gardencorev1beta1.InternalSecret{
								ObjectMeta: metav1.ObjectMeta{GenerateName: "test-foo-", Namespace: "default"},
								StringData: map[string]string{"content": "foo"},
							}
						},
						NewEmptyList: func() client.ObjectList { return &gardencorev1beta1.InternalSecretList{} },
					},
					{
						NewObject: func() client.Object {
							return &gardencorev1beta1.ShootState{
								ObjectMeta: metav1.ObjectMeta{GenerateName: "test-foo-", Namespace: "default"},
								Spec:       gardencorev1beta1.ShootStateSpec{Gardener: []gardencorev1beta1.GardenerResourceData{{Name: "foo"}}},
							}
						},
						NewEmptyList: func() client.ObjectList { return &gardencorev1beta1.ShootStateList{} },
					},
					{
						NewObject: func() client.Object {
							return &gardencorev1.ControllerDeployment{
								ObjectMeta: metav1.ObjectMeta{GenerateName: "test-foo-", Namespace: "default"},
								Helm: &gardencorev1.HelmControllerDeployment{
									RawChart: []byte("foo"),
								},
							}
						},
						NewEmptyList: func() client.ObjectList { return &gardencorev1.ControllerDeploymentList{} },
					},
					{
						NewObject: func() client.Object {
							suffix, err := utils.GenerateRandomString(5)
							Expect(err).NotTo(HaveOccurred())
							return &gardencorev1beta1.ControllerRegistration{
								ObjectMeta: metav1.ObjectMeta{GenerateName: "test-foo-", Namespace: "default"},
								Spec:       gardencorev1beta1.ControllerRegistrationSpec{Resources: []gardencorev1beta1.ControllerResource{{Kind: "Infrastructure", Type: "test-foo-" + suffix}}},
							}
						},
						NewEmptyList: func() client.ObjectList { return &gardencorev1beta1.ControllerRegistrationList{} },
					},
				},
			},
			&rotation.VirtualGardenAccessVerifier{RuntimeClient: s.GardenClient, Namespace: v1beta1constants.GardenNamespace},
		}

		// the verifiers used in this test still use separate "By" statements for structuring tests and expect to be executed within an "It" statement
		// This is a problem as we removed the "top-level" "It" statements during the refactoring of this test
		// Until all verifiers are refactored, we need to instantiate separate "It" statements for all shared verifiers to allow for assertions
		// Also see test/e2e/gardener/shoot/create_rotate_delete.go, where some of the verifiers already got refactored to use separate "It" statements
		// TODO(Wieneo): Refactor and consolidate verifiers and verifier interface
		for _, vv := range v {
			It(fmt.Sprintf("Verify before for %T", vv), func(ctx SpecContext) {
				vv.Before(ctx)
			}, SpecTimeout(5*time.Minute))
		}

		ItShouldAnnotateGarden(s, map[string]string{
			v1beta1constants.GardenerOperation: v1beta1constants.OperationRotateCredentialsStart,
		})

		ItShouldEventuallyNotHaveOperationAnnotation(s.GardenKomega, s.Garden)

		It("Rotation in Preparing status", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				g.Expect(s.GardenKomega.Get(s.Garden)()).To(Succeed())
				v.ExpectPreparingStatus(g)
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		ItShouldWaitForGardenToBeReconciledAndHealthy(s)

		for _, vv := range v {
			It(fmt.Sprintf("Verify after prepared for %T", vv), func(ctx SpecContext) {
				vv.AfterPrepared(ctx)
			}, SpecTimeout(5*time.Minute))
		}

		ItShouldAnnotateGarden(s, map[string]string{
			v1beta1constants.GardenerOperation: v1beta1constants.OperationRotateCredentialsComplete,
		})

		ItShouldEventuallyNotHaveOperationAnnotation(s.GardenKomega, s.Garden)

		It("Rotation in Completing status", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				g.Expect(s.GardenKomega.Get(s.Garden)()).To(Succeed())
				v.ExpectCompletingStatus(g)
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		ItShouldWaitForGardenToBeReconciledAndHealthy(s)

		for _, vv := range v {
			It(fmt.Sprintf("Verify after completed for %T", vv), func(ctx SpecContext) {
				vv.AfterCompleted(ctx)
			}, SpecTimeout(5*time.Minute))
		}

		for _, vv := range v {
			if cleanup, ok := vv.(rotationutils.CleanupVerifier); ok {
				It(fmt.Sprintf("Cleanup for %T", vv), func(ctx SpecContext) {
					cleanup.Cleanup(ctx)
				})
			}
		}

		ItShouldDeleteGarden(s)
		ItShouldWaitForGardenToBeDeleted(s)
		ItShouldCleanUp(s)
		ItShouldWaitForExtensionToReportDeletion(s, "provider-local")
	})
})
