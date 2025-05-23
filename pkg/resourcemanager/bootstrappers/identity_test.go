// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/resourcemanager/bootstrappers"
)

var _ = Describe("Identity", func() {
	var (
		ctx               = context.TODO()
		fakeClient        client.Client
		cfg               *resourcemanagerconfigv1alpha1.ResourceManagerConfiguration
		determiner        *IdentityDeterminer
		identityConfigMap *corev1.ConfigMap
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().Build()
		cfg = &resourcemanagerconfigv1alpha1.ResourceManagerConfiguration{
			Controllers: resourcemanagerconfigv1alpha1.ResourceManagerControllerConfiguration{
				ClusterID: ptr.To(""),
			},
		}
		determiner = &IdentityDeterminer{
			Logger:       logr.Discard(),
			SourceClient: fakeClient,
			Config:       cfg,
		}
		identityConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster-identity",
				Namespace: "kube-system",
			},
		}
	})

	Describe("#Start", func() {
		It("should do nothing because cluster id is empty", func() {
			Expect(determiner.Start(ctx)).To(Succeed())

			Expect(cfg.Controllers.ClusterID).To(PointTo(Equal("")))
		})

		It("should do nothing because cluster id is already set", func() {
			determiner.Config.Controllers.ClusterID = ptr.To("foo")

			Expect(determiner.Start(ctx)).To(Succeed())
			Expect(cfg.Controllers.ClusterID).To(PointTo(Equal("foo")))
		})

		Context("when cluster id shall be determined but not forced", func() {
			BeforeEach(func() {
				determiner.Config.Controllers.ClusterID = ptr.To("<default>")
			})

			It("should do nothing because cluster-identity configmap does not exist", func() {
				Expect(determiner.Start(ctx)).To(Succeed())
				Expect(cfg.Controllers.ClusterID).To(PointTo(Equal("")))
			})

			It("should do nothing because cluster-identity configmap does not contain identity", func() {
				Expect(fakeClient.Create(ctx, identityConfigMap)).To(Succeed())

				Expect(determiner.Start(ctx)).To(Succeed())
				Expect(cfg.Controllers.ClusterID).To(PointTo(Equal("")))
			})

			It("should use the identity of the cluster-identity configmap", func() {
				identityConfigMap.Data = map[string]string{"cluster-identity": "id"}
				Expect(fakeClient.Create(ctx, identityConfigMap)).To(Succeed())

				Expect(determiner.Start(ctx)).To(Succeed())
				Expect(cfg.Controllers.ClusterID).To(PointTo(Equal("id")))
			})
		})

		Context("when cluster id shall be determined and forced", func() {
			BeforeEach(func() {
				determiner.Config.Controllers.ClusterID = ptr.To("<cluster>")
			})

			It("should return an error because the cluster-identity configmap does not exist", func() {
				Expect(determiner.Start(ctx)).To(MatchError(ContainSubstring(`cannot determine cluster identity from configmap: configmaps "cluster-identity" not found`)))
				Expect(cfg.Controllers.ClusterID).To(PointTo(Equal("<cluster>")))
			})

			It("should do nothing because cluster-identity configmap does not contain identity", func() {
				Expect(fakeClient.Create(ctx, identityConfigMap)).To(Succeed())

				Expect(determiner.Start(ctx)).To(MatchError(ContainSubstring(`cannot determine cluster identity from configmap: no cluster-identity entry`)))
				Expect(cfg.Controllers.ClusterID).To(PointTo(Equal("<cluster>")))
			})

			It("should use the identity of the cluster-identity configmap", func() {
				identityConfigMap.Data = map[string]string{"cluster-identity": "id"}
				Expect(fakeClient.Create(ctx, identityConfigMap)).To(Succeed())

				Expect(determiner.Start(ctx)).To(Succeed())
				Expect(cfg.Controllers.ClusterID).To(PointTo(Equal("id")))
			})
		})
	})
})
