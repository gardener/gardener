// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/bootstrappers"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("VerifyVersion", func() {
	var (
		ctx        = context.Background()
		log        logr.Logger
		fakeClient client.Client
		garden     *operatorv1alpha1.Garden
	)

	BeforeEach(func() {
		log = logr.Discard()
		garden = &operatorv1alpha1.Garden{ObjectMeta: metav1.ObjectMeta{GenerateName: "garden-"}}
		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).WithStatusSubresource(garden).Build()
	})

	Describe("#VerifyGardenerVersion", func() {
		It("should do nothing because no Gardens exist", func() {
			Expect(VerifyGardenerVersion(ctx, log, fakeClient)).To(Succeed())
		})

		It("should fail because more than one Garden exist", func() {
			Expect(fakeClient.Create(ctx, garden.DeepCopy())).To(Succeed())
			Expect(fakeClient.Create(ctx, garden.DeepCopy())).To(Succeed())

			Expect(VerifyGardenerVersion(ctx, log, fakeClient)).To(MatchError(ContainSubstring("expected at most one Garden")))
		})

		It("should do nothing because old Garden version is not maintained", func() {
			Expect(fakeClient.Create(ctx, garden)).To(Succeed())

			Expect(VerifyGardenerVersion(ctx, log, fakeClient)).To(Succeed())
		})

		DescribeTable("tests",
			func(oldVersion, currentVersion string, matcher gomegatypes.GomegaMatcher) {
				Expect(fakeClient.Create(ctx, garden)).To(Succeed())
				garden.Status.Gardener = &gardencorev1beta1.Gardener{Version: oldVersion}
				Expect(fakeClient.Status().Update(ctx, garden)).To(Succeed())

				DeferCleanup(test.WithVar(&GetCurrentVersion, func() apimachineryversion.Info { return apimachineryversion.Info{GitVersion: currentVersion} }))

				Expect(VerifyGardenerVersion(ctx, log, fakeClient)).To(matcher)
			},

			Entry("fail because old version cannot be parsed", "unparsable$version", "v1.2.3", MatchError(ContainSubstring("failed parsing old Garden version"))),
			Entry("fail because current version cannot be parsed", "v1.2.3", "unparsable$version", MatchError(ContainSubstring("failed comparing versions for downgrade check"))),

			Entry("fail because downgrade is unsupported", "v1.2.3", "v1.1.1", MatchError(ContainSubstring("downgrading Gardener is not supported"))),
			Entry("fail because downgrade is unsupported (old version suffixed with '-dev')", "v1.2.3-dev", "v1.1.1", MatchError(ContainSubstring("downgrading Gardener is not supported"))),
			Entry("fail because downgrade is unsupported (new version suffixed with '-dev')", "v1.2.3", "v1.1.1-dev", MatchError(ContainSubstring("downgrading Gardener is not supported"))),
			Entry("fail because downgrade is unsupported (both version suffixed with '-dev')", "v1.2.3-dev", "v1.1.1-dev", MatchError(ContainSubstring("downgrading Gardener is not supported"))),

			Entry("fail because upgrading more than one minor version is unsupported", "v1.2.3", "v1.4.4", MatchError(ContainSubstring("skipping Gardener versions is unsupported"))),
			Entry("fail because upgrading more than one minor version is unsupported (old version suffixed with '-dev')", "v1.2.3-dev", "v1.4.4", MatchError(ContainSubstring("skipping Gardener versions is unsupported"))),
			Entry("fail because upgrading more than one minor version is unsupported (new version suffixed with '-dev')", "v1.2.3", "v1.4.4-dev", MatchError(ContainSubstring("skipping Gardener versions is unsupported"))),
			Entry("fail because upgrading more than one minor version is unsupported (both version suffixed with '-dev')", "v1.2.3-dev", "v1.4.4-dev", MatchError(ContainSubstring("skipping Gardener versions is unsupported"))),

			Entry("succeed because minor version did not change", "v1.2.3", "v1.2.4", Succeed()),
			Entry("succeed because minor version did not change (old version suffixed with '-dev'", "v1.2.3-dev", "v1.2.4", Succeed()),
			Entry("succeed because minor version did not change (new version suffixed with '-dev'", "v1.2.3", "v1.2.4-dev", Succeed()),
			Entry("succeed because minor version did not change (both version suffixed with '-dev'", "v1.2.3-dev", "v1.2.4-dev", Succeed()),

			Entry("succeed because minor version differs by only one 1", "v1.2.3", "v1.3.0", Succeed()),
			Entry("succeed because minor version differs by only one 1 (old version suffixed with '-dev')", "v1.2.3-dev", "v1.3.0", Succeed()),
			Entry("succeed because minor version differs by only one 1 (new version suffixed with '-dev')", "v1.2.3", "v1.3.0-dev", Succeed()),
			Entry("succeed because minor version differs by only one 1 (both version suffixed with '-dev')", "v1.2.3-dev", "v1.3.0-dev", Succeed()),
		)
	})
})
