// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretsrotation_test

import (
	"context"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/gardener/secretsrotation"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

var _ = Describe("Observability", func() {
	Context("#CheckIfGlobalObservabilitySecretPropagatedToAllSeeds", func() {
		const lastRotationInitiationTimestamp = 1700000000

		var (
			ctx          context.Context
			gardenClient client.Client
			seeds        []gardencorev1beta1.Seed
		)

		BeforeEach(func() {
			ctx = context.TODO()
			gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

			seeds = []gardencorev1beta1.Seed{
				{ObjectMeta: metav1.ObjectMeta{Name: "seed1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "seed2"}},
			}
			for _, seed := range seeds {
				Expect(gardenClient.Create(ctx, &seed)).To(Succeed())
			}
		})

		createGlobalMonitoringSecretWithRawTimestamp := func(seedName, timestamp string) {
			Expect(gardenClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      "observability-ingress",
				Namespace: gardenerutils.ComputeGardenNamespace(seedName),
				Labels: map[string]string{
					v1beta1constants.GardenRole:                       v1beta1constants.GardenRoleGlobalMonitoring,
					secretsmanager.LabelKeyLastRotationInitiationTime: timestamp,
				},
			}})).To(Succeed())
		}

		createGlobalMonitoringSecret := func(seedName string, timestamp int) {
			createGlobalMonitoringSecretWithRawTimestamp(seedName, strconv.Itoa(timestamp))
		}

		createGlobalMonitoringSecretWithoutTimestamp := func(seedName string) {
			Expect(gardenClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      "observability-ingress",
				Namespace: gardenerutils.ComputeGardenNamespace(seedName),
				Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleGlobalMonitoring},
			}})).To(Succeed())
		}

		It("should succeed when the secret propagated to all seeds with the expected timestamp", func() {
			createGlobalMonitoringSecret("seed1", lastRotationInitiationTimestamp)
			createGlobalMonitoringSecret("seed2", lastRotationInitiationTimestamp)

			Expect(CheckIfGlobalObservabilitySecretPropagatedToAllSeeds(ctx, gardenClient, lastRotationInitiationTimestamp)).To(Succeed())
		})

		It("should succeed when a seed carries a newer timestamp than expected", func() {
			createGlobalMonitoringSecret("seed1", lastRotationInitiationTimestamp)
			createGlobalMonitoringSecret("seed2", lastRotationInitiationTimestamp+1)

			Expect(CheckIfGlobalObservabilitySecretPropagatedToAllSeeds(ctx, gardenClient, lastRotationInitiationTimestamp)).To(Succeed())
		})

		It("should fail when a seed still carries an older timestamp", func() {
			createGlobalMonitoringSecret("seed1", lastRotationInitiationTimestamp)
			createGlobalMonitoringSecret("seed2", lastRotationInitiationTimestamp-1)

			Expect(CheckIfGlobalObservabilitySecretPropagatedToAllSeeds(ctx, gardenClient, lastRotationInitiationTimestamp)).To(MatchError(ContainSubstring("not yet propagated to namespace \"seed-seed2\" of seed \"seed2\"")))
		})

		It("should ignore a secret without the rotation initiation time label", func() {
			createGlobalMonitoringSecret("seed1", lastRotationInitiationTimestamp)
			createGlobalMonitoringSecretWithoutTimestamp("seed2")

			Expect(CheckIfGlobalObservabilitySecretPropagatedToAllSeeds(ctx, gardenClient, lastRotationInitiationTimestamp)).To(Succeed())
		})

		It("should fail when a secret carries an empty rotation initiation time label", func() {
			createGlobalMonitoringSecret("seed1", lastRotationInitiationTimestamp)
			createGlobalMonitoringSecretWithRawTimestamp("seed2", "")

			Expect(CheckIfGlobalObservabilitySecretPropagatedToAllSeeds(ctx, gardenClient, lastRotationInitiationTimestamp)).To(MatchError(ContainSubstring("does not yet carry a last rotation initiation time")))
		})

		It("should fail when a secret carries a non-numeric rotation initiation time label", func() {
			createGlobalMonitoringSecret("seed1", lastRotationInitiationTimestamp)
			createGlobalMonitoringSecretWithRawTimestamp("seed2", "not-a-number")

			Expect(CheckIfGlobalObservabilitySecretPropagatedToAllSeeds(ctx, gardenClient, lastRotationInitiationTimestamp)).To(MatchError(ContainSubstring("error parsing last rotation initiation time")))
		})
	})
})
