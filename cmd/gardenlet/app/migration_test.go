// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/cmd/gardenlet/app"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Migration", func() {
	Describe("#CleanupHashVersioningSecrets", func() {
		var (
			ctx        = context.Background()
			fakeClient client.Client
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		})

		It("should do nothing when there are no shoot namespaces", func() {
			Expect(CleanupHashVersioningSecrets(ctx, fakeClient)).To(Succeed())
		})

		It("should not error when shoot namespaces exist but the secret is absent", func() {
			ns := shootNamespace("shoot--foo--bar")
			Expect(fakeClient.Create(ctx, ns)).To(Succeed())

			Expect(CleanupHashVersioningSecrets(ctx, fakeClient)).To(Succeed())
		})

		It("should delete the secret in a shoot namespace", func() {
			ns := shootNamespace("shoot--foo--bar")
			Expect(fakeClient.Create(ctx, ns)).To(Succeed())

			secret := oscHashSecret("shoot--foo--bar")
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			Expect(CleanupHashVersioningSecrets(ctx, fakeClient)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(MatchError(ContainSubstring("not found")))
		})

		It("should delete secrets across multiple shoot namespaces", func() {
			for _, name := range []string{"shoot--p--a", "shoot--p--b", "shoot--p--c"} {
				Expect(fakeClient.Create(ctx, shootNamespace(name))).To(Succeed())
				Expect(fakeClient.Create(ctx, oscHashSecret(name))).To(Succeed())
			}

			Expect(CleanupHashVersioningSecrets(ctx, fakeClient)).To(Succeed())

			for _, name := range []string{"shoot--p--a", "shoot--p--b", "shoot--p--c"} {
				secret := oscHashSecret(name)
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(BeNotFoundError())
			}
		})

		It("should handle a mix of namespaces with and without the secret", func() {
			for _, name := range []string{"shoot--p--a", "shoot--p--b"} {
				Expect(fakeClient.Create(ctx, shootNamespace(name))).To(Succeed())
			}
			Expect(fakeClient.Create(ctx, oscHashSecret("shoot--p--a"))).To(Succeed())

			Expect(CleanupHashVersioningSecrets(ctx, fakeClient)).To(Succeed())

			secretA := oscHashSecret("shoot--p--a")
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secretA), secretA)).To(BeNotFoundError())
		})
	})

	Describe("#VerifyRemoveHTTPProxyLegacyPortMigration", func() {
		const seedName = "seed"

		var (
			ctx = context.Background()

			verify = func(shoots ...*gardencorev1beta1.Shoot) error {
				builder := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme)
				for _, shoot := range shoots {
					builder = builder.WithObjects(shoot)
				}
				return VerifyRemoveHTTPProxyLegacyPortMigration(ctx, builder.Build(), seedName)
			}
		)

		It("should succeed when there are no shoots at all", func() {
			Expect(verify()).To(Succeed())
		})

		Context("shoots this seed is responsible for", func() {
			It("should succeed when the constraint is 'True'", func() {
				Expect(verify(shootUsingUnifiedPort("a", seedName))).To(Succeed())
			})

			It("should error when the constraint is absent", func() {
				shoot := shootUsingUnifiedPort("a", seedName)
				shoot.Status.Constraints = nil

				Expect(verify(shoot)).To(MatchError(ContainSubstring("UsesUnifiedHTTPProxyPort")))
			})

			It("should error when the constraint is 'False'", func() {
				shoot := shootUsingUnifiedPort("a", seedName)
				shoot.Status.Constraints[0].Status = gardencorev1beta1.ConditionFalse

				Expect(verify(shoot)).To(MatchError(ContainSubstring("UsesUnifiedHTTPProxyPort")))
			})

			It("should error when the constraint is 'Unknown'", func() {
				shoot := shootUsingUnifiedPort("a", seedName)
				shoot.Status.Constraints[0].Status = gardencorev1beta1.ConditionUnknown

				Expect(verify(shoot)).To(MatchError(ContainSubstring("UsesUnifiedHTTPProxyPort")))
			})

			It("should error when only one out of many shoots misses the constraint", func() {
				bad := shootUsingUnifiedPort("b", seedName)
				bad.Status.Constraints = nil

				Expect(verify(shootUsingUnifiedPort("a", seedName), bad, shootUsingUnifiedPort("c", seedName))).To(MatchError(ContainSubstring("UsesUnifiedHTTPProxyPort")))
			})
		})

		Context("shoots this seed is not responsible for", func() {
			It("should ignore a shoot assigned to another seed", func() {
				shoot := shootUsingUnifiedPort("a", "other-seed")
				shoot.Status.Constraints = nil

				Expect(verify(shoot)).To(Succeed())
			})

			It("should ignore a shoot which is not assigned to any seed", func() {
				shoot := shootUsingUnifiedPort("a", seedName)
				shoot.Spec.SeedName, shoot.Status.SeedName, shoot.Status.Constraints = nil, nil, nil

				Expect(verify(shoot)).To(Succeed())
			})

			It("should ignore a shoot being migrated away from this seed once status.seedName was updated", func() {
				// spec.seedName != status.seedName => the seed in status.seedName prepares the migration.
				shoot := shootUsingUnifiedPort("a", seedName)
				shoot.Spec.SeedName, shoot.Status.SeedName, shoot.Status.Constraints = new(seedName), new("other-seed"), nil

				Expect(verify(shoot)).To(Succeed())
			})

			It("should consider a shoot being migrated to this seed", func() {
				shoot := shootUsingUnifiedPort("a", seedName)
				shoot.Spec.SeedName, shoot.Status.SeedName, shoot.Status.Constraints = new("other-seed"), new(seedName), nil

				Expect(verify(shoot)).To(MatchError(ContainSubstring("UsesUnifiedHTTPProxyPort")))
			})

			It("should consider a shoot whose status.seedName is not set yet", func() {
				shoot := shootUsingUnifiedPort("a", seedName)
				shoot.Status.SeedName, shoot.Status.Constraints = nil, nil

				Expect(verify(shoot)).To(MatchError(ContainSubstring("UsesUnifiedHTTPProxyPort")))
			})
		})

		Context("shoots which cannot have the constraint yet", func() {
			It("should ignore workerless shoots", func() {
				shoot := shootUsingUnifiedPort("a", seedName)
				shoot.Spec.Provider.Workers, shoot.Status.Constraints = nil, nil

				Expect(verify(shoot)).To(Succeed())
			})

			It("should ignore shoots which were not picked up yet", func() {
				shoot := shootUsingUnifiedPort("a", seedName)
				shoot.Status.LastOperation, shoot.Status.Constraints = nil, nil

				Expect(verify(shoot)).To(Succeed())
			})

			DescribeTable("should ignore shoots which are still being created or deleted",
				func(operationType gardencorev1beta1.LastOperationType, state gardencorev1beta1.LastOperationState) {
					shoot := shootUsingUnifiedPort("a", seedName)
					shoot.Status.LastOperation.Type, shoot.Status.LastOperation.State = operationType, state
					shoot.Status.Constraints = nil

					Expect(verify(shoot)).To(Succeed())
				},

				Entry("creation processing", gardencorev1beta1.LastOperationTypeCreate, gardencorev1beta1.LastOperationStateProcessing),
				Entry("creation pending", gardencorev1beta1.LastOperationTypeCreate, gardencorev1beta1.LastOperationStatePending),
				Entry("creation error", gardencorev1beta1.LastOperationTypeCreate, gardencorev1beta1.LastOperationStateError),
				Entry("deletion processing", gardencorev1beta1.LastOperationTypeDelete, gardencorev1beta1.LastOperationStateProcessing),
				Entry("deletion error", gardencorev1beta1.LastOperationTypeDelete, gardencorev1beta1.LastOperationStateError),
			)

			DescribeTable("should consider shoots which finished creation or are reconciling",
				func(operationType gardencorev1beta1.LastOperationType, state gardencorev1beta1.LastOperationState) {
					shoot := shootUsingUnifiedPort("a", seedName)
					shoot.Status.LastOperation.Type, shoot.Status.LastOperation.State = operationType, state
					shoot.Status.Constraints = nil

					Expect(verify(shoot)).To(MatchError(ContainSubstring("UsesUnifiedHTTPProxyPort")))
				},

				Entry("creation succeeded", gardencorev1beta1.LastOperationTypeCreate, gardencorev1beta1.LastOperationStateSucceeded),
				Entry("reconciliation processing", gardencorev1beta1.LastOperationTypeReconcile, gardencorev1beta1.LastOperationStateProcessing),
				Entry("reconciliation succeeded", gardencorev1beta1.LastOperationTypeReconcile, gardencorev1beta1.LastOperationStateSucceeded),
			)
		})
	})
})

// shootUsingUnifiedPort returns a regular Shoot which is assigned to the given Seed, was reconciled successfully,
// and carries the UsesUnifiedHTTPProxyPort constraint with status 'True', i.e. a Shoot which does not block the migration.
func shootUsingUnifiedPort(name, seedName string) *gardencorev1beta1.Shoot {
	return &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "garden-proj"},
		Spec: gardencorev1beta1.ShootSpec{
			SeedName: &seedName,
			Provider: gardencorev1beta1.Provider{
				Workers: []gardencorev1beta1.Worker{{Name: "pool"}},
			},
		},
		Status: gardencorev1beta1.ShootStatus{
			SeedName: &seedName,
			LastOperation: &gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeReconcile,
				State: gardencorev1beta1.LastOperationStateSucceeded,
			},
			Constraints: []gardencorev1beta1.Condition{{
				Type:   gardencorev1beta1.ShootUsesUnifiedHTTPProxyPort,
				Status: gardencorev1beta1.ConditionTrue,
			}},
		},
	}
}

func shootNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot},
		},
	}
}

func oscHashSecret(namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatingsystemconfig.WorkerPoolHashesSecretName,
			Namespace: namespace,
		},
	}
}
