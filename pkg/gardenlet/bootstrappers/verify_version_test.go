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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/pkg/gardenlet/bootstrappers"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("VerifyVersion", func() {
	var (
		ctx        = context.Background()
		log        logr.Logger
		fakeClient client.Client
		configMap  *corev1.ConfigMap
	)

	BeforeEach(func() {
		log = logr.Discard()
		configMap = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "gardener-info", Namespace: "gardener-system-public"}}
		fakeClient = fakeclient.NewClientBuilder().Build()
	})

	Describe("#VerifyGardenerVersion", func() {
		It("should fail if the gardener-info ConfigMap does not exist", func() {
			Expect(VerifyGardenerVersion(ctx, log, fakeClient)).To(MatchError(ContainSubstring("failed reading ConfigMap gardener")))
		})

		It("should fail if the gardener-apiserver information data cannot be parsed", func() {
			configMap.Data = map[string]string{"gardenerAPIServer": "CANNOT-PARSE-THIS"}
			Expect(fakeClient.Create(ctx, configMap)).To(Succeed())

			Expect(VerifyGardenerVersion(ctx, log, fakeClient)).To(MatchError(ContainSubstring("failed unmarshalling the gardener-apiserver information structure")))
		})

		DescribeTable("tests",
			func(gardenerAPIServerVersion, gardenletVersion string, matcher gomegatypes.GomegaMatcher) {
				configMap.Data = map[string]string{"gardenerAPIServer": "version: " + gardenerAPIServerVersion}
				Expect(fakeClient.Create(ctx, configMap)).To(Succeed())

				DeferCleanup(test.WithVar(&GetCurrentVersion, func() apimachineryversion.Info { return apimachineryversion.Info{GitVersion: gardenletVersion} }))

				Expect(VerifyGardenerVersion(ctx, log, fakeClient)).To(matcher)
			},

			Entry("fail because gardener-apiserver version cannot be parsed", "unparsable$version", "v1.2.3", MatchError(ContainSubstring("failed parsing version of gardener-apiserver"))),
			Entry("fail because gardenlet version cannot be parsed", "v1.2.3", "unparsable$version", MatchError(ContainSubstring("failed parsing version of gardenlet"))),

			Entry("fail because gardenlet version is too high", "v1.2.3", "v1.3.0", MatchError(ContainSubstring("gardenlet version must not be newer than gardener-apiserver version"))),
			Entry("fail because gardenlet version is too high", "v1.2.3", "v1.2.4", MatchError(ContainSubstring("gardenlet version must not be newer than gardener-apiserver version"))),
			Entry("fail because gardenlet version is too high (gardener-apiserver version suffixed with '-dev')", "v1.2.3-dev", "v1.2.4", MatchError(ContainSubstring("gardenlet version must not be newer than gardener-apiserver version"))),
			Entry("fail because gardenlet version is too high (gardenlet version suffixed with '-dev')", "v1.2.3", "v1.2.4-dev", MatchError(ContainSubstring("gardenlet version must not be newer than gardener-apiserver version"))),
			Entry("fail because gardenlet version is too high (both version suffixed with '-dev')", "v1.2.3-dev", "v1.2.4-dev", MatchError(ContainSubstring("gardenlet version must not be newer than gardener-apiserver version"))),

			Entry("fail because gardenlet version is too low", "v1.3.3", "v1.0.4", MatchError(ContainSubstring("gardenlet version must not be older than two minor gardener-apiserver versions"))),
			Entry("fail because gardenlet version is too low (old version suffixed with '-dev')", "v1.3.3-dev", "v1.0.4", MatchError(ContainSubstring("gardenlet version must not be older than two minor gardener-apiserver versions"))),
			Entry("fail because gardenlet version is too low (new version suffixed with '-dev')", "v1.3.3", "v1.0.4-dev", MatchError(ContainSubstring("gardenlet version must not be older than two minor gardener-apiserver versions"))),
			Entry("fail because gardenlet version is too low (both version suffixed with '-dev')", "v1.3.3-dev", "v1.0.4-dev", MatchError(ContainSubstring("gardenlet version must not be older than two minor gardener-apiserver versions"))),

			Entry("succeed because gardenlet version is equal to gardener-apiserver version", "v1.2.3", "v1.2.3", Succeed()),
			Entry("succeed because gardenlet version is equal to gardener-apiserver version (gardener-apiserver version suffixed with '-dev')", "v1.2.3-dev", "v1.2.3", Succeed()),
			Entry("succeed because gardenlet version is equal to gardener-apiserver version (gardenlet version suffixed with '-dev')", "v1.2.3", "v1.2.3-dev", Succeed()),
			Entry("succeed because gardenlet version is equal to gardener-apiserver version (both versions suffixed with '-dev')", "v1.2.3-dev", "v1.2.3-dev", Succeed()),

			Entry("succeed because gardenlet patch version is lower than gardener-apiserver version", "v1.2.3", "v1.2.2", Succeed()),
			Entry("succeed because gardenlet patch version is lower than gardener-apiserver version (gardener-apiserver version suffixed with '-dev')", "v1.2.3-dev", "v1.2.2", Succeed()),
			Entry("succeed because gardenlet patch version is lower than gardener-apiserver version (gardenlet version suffixed with '-dev')", "v1.2.3", "v1.2.2-dev", Succeed()),
			Entry("succeed because gardenlet patch version is lower than gardener-apiserver version (both versions suffixed with '-dev')", "v1.2.3-dev", "v1.2.2-dev", Succeed()),

			Entry("succeed because gardenlet minor version is lower (by 1) than gardener-apiserver version", "v1.2.3", "v1.1.3", Succeed()),
			Entry("succeed because gardenlet minor version is lower (by 1) than gardener-apiserver version (gardener-apiserver version suffixed with '-dev')", "v1.2.3-dev", "v1.1.3", Succeed()),
			Entry("succeed because gardenlet minor version is lower (by 1) than gardener-apiserver version (gardenlet version suffixed with '-dev')", "v1.2.3", "v1.1.3-dev", Succeed()),
			Entry("succeed because gardenlet minor version is lower (by 1) than gardener-apiserver version (both versions suffixed with '-dev')", "v1.2.3-dev", "v1.1.3-dev", Succeed()),

			Entry("succeed because gardenlet minor version is lower (by 2) than gardener-apiserver version", "v1.2.3", "v1.0.3", Succeed()),
			Entry("succeed because gardenlet minor version is lower (by 2) than gardener-apiserver version (gardener-apiserver version suffixed with '-dev')", "v1.2.3-dev", "v1.0.3", Succeed()),
			Entry("succeed because gardenlet minor version is lower (by 2) than gardener-apiserver version (gardenlet version suffixed with '-dev')", "v1.2.3", "v1.0.3-dev", Succeed()),
			Entry("succeed because gardenlet minor version is lower (by 2) than gardener-apiserver version (both versions suffixed with '-dev')", "v1.2.3-dev", "v1.0.3-dev", Succeed()),
		)
	})
})
