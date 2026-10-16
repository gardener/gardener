// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package cmd_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/pkg/gardenadm/cmd"
)

var _ = Describe("BootstrapCluster", func() {
	Describe("#EnsureNoGardenletOrOperator", func() {
		var (
			ctx        = context.Background()
			fakeClient client.Client
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().Build()
		})

		It("should succeed if neither gardener-operator nor gardenlet is deployed", func() {
			Expect(EnsureNoGardenletOrOperator(ctx, fakeClient, "foo")).To(Succeed())
		})

		It("should fail if gardener-operator is deployed", func() {
			Expect(fakeClient.Create(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: "garden", Name: "gardener-operator"}})).To(Succeed())

			Expect(EnsureNoGardenletOrOperator(ctx, fakeClient, "foo")).To(MatchError(And(
				ContainSubstring(`deployment "garden/gardener-operator" exists on the targeted cluster`),
				ContainSubstring("`gardenadm foo` does not support"),
			)))
		})

		It("should fail if gardenlet is deployed", func() {
			Expect(fakeClient.Create(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: "garden", Name: "gardenlet"}})).To(Succeed())

			Expect(EnsureNoGardenletOrOperator(ctx, fakeClient, "foo")).To(MatchError(ContainSubstring(`deployment "garden/gardenlet" exists on the targeted cluster`)))
		})
	})
})
