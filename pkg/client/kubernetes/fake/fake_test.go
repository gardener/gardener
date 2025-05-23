// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fake_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	fakekubernetes "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/client/kubernetes/test"
	mockdiscovery "github.com/gardener/gardener/third_party/mock/client-go/discovery"
	mockcache "github.com/gardener/gardener/third_party/mock/controller-runtime/cache"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Fake ClientSet", func() {
	var (
		builder *fake.ClientSetBuilder
		ctrl    *gomock.Controller
	)

	BeforeEach(func() {
		builder = fake.NewClientSetBuilder()
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should correctly set applier attribute", func() {
		applier := kubernetesmock.NewMockApplier(ctrl)
		cs := builder.WithApplier(applier).Build()

		Expect(cs.Applier()).To(BeIdenticalTo(applier))
	})

	It("should correctly set chartRenderer attribute", func() {
		chartRenderer := chartrenderer.NewWithServerVersion(&version.Info{Major: "1", Minor: "18"})
		cs := builder.WithChartRenderer(chartRenderer).Build()

		Expect(cs.ChartRenderer()).To(BeIdenticalTo(chartRenderer))
	})

	It("should correctly set chartApplier attribute", func() {
		chartApplier := kubernetesmock.NewMockChartApplier(ctrl)
		cs := builder.WithChartApplier(chartApplier).Build()

		Expect(cs.ChartApplier()).To(BeIdenticalTo(chartApplier))
	})

	It("should correctly set podExecutor attribute", func() {
		podExecutor := kubernetesmock.NewMockPodExecutor(ctrl)
		cs := builder.WithPodExecutor(podExecutor).Build()

		Expect(cs.PodExecutor()).To(BeIdenticalTo(podExecutor))
	})

	It("should correctly set restConfig attribute", func() {
		restConfig := &rest.Config{}
		cs := builder.WithRESTConfig(restConfig).Build()

		Expect(cs.RESTConfig()).To(BeIdenticalTo(restConfig))
	})

	It("should correctly set client attribute", func() {
		client := mockclient.NewMockClient(ctrl)
		cs := builder.WithClient(client).Build()

		Expect(cs.Client()).To(BeIdenticalTo(client))
	})

	It("should correctly set apiReader attribute", func() {
		apiReader := mockclient.NewMockReader(ctrl)
		cs := builder.WithAPIReader(apiReader).Build()

		Expect(cs.APIReader()).To(BeIdenticalTo(apiReader))
	})

	It("should correctly set cache attribute", func() {
		cache := mockcache.NewMockCache(ctrl)
		cs := builder.WithCache(cache).Build()

		Expect(cs.Cache()).To(BeIdenticalTo(cache))
	})

	It("should correctly set kubernetes attribute", func() {
		kubernetes := fakekubernetes.NewSimpleClientset()
		cs := builder.WithKubernetes(kubernetes).Build()

		Expect(cs.Kubernetes()).To(BeIdenticalTo(kubernetes))
	})

	It("should correctly set restClient attribute", func() {
		disc, err := discovery.NewDiscoveryClientForConfig(&rest.Config{})
		Expect(err).NotTo(HaveOccurred())
		restClient := disc.RESTClient()
		cs := builder.WithRESTClient(restClient).Build()

		Expect(cs.RESTClient()).To(BeIdenticalTo(restClient))
	})

	It("should correctly set version attribute", func() {
		version := "1.24.0"
		cs := builder.WithVersion(version).Build()

		Expect(cs.Version()).To(Equal(version))
	})

	Context("#DiscoverVersion", func() {
		It("should correctly refresh server version", func() {
			oldVersion, newVersion := "1.24.1", "1.24.2"
			cs := builder.
				WithVersion(oldVersion).
				WithKubernetes(test.NewClientSetWithFakedServerVersion(nil, &version.Info{GitVersion: newVersion})).
				Build()

			Expect(cs.Version()).To(Equal(oldVersion))
			_, err := cs.DiscoverVersion()
			Expect(err).NotTo(HaveOccurred())
			Expect(cs.Version()).To(Equal(newVersion))
		})

		It("should fail if discovery fails", func() {
			discovery := mockdiscovery.NewMockDiscoveryInterface(ctrl)
			discovery.EXPECT().ServerVersion().Return(nil, errors.New("fake"))

			cs := builder.
				WithKubernetes(test.NewClientSetWithDiscovery(nil, discovery)).
				Build()

			_, err := cs.DiscoverVersion()
			Expect(err).To(MatchError("fake"))
		})
	})

	It("should do nothing on Start", func() {
		cs := fake.NewClientSet()

		cs.Start(context.Background())
	})

	It("should do nothing on WaitForCacheSync", func() {
		cs := fake.NewClientSet()

		Expect(cs.WaitForCacheSync(context.Background())).To(BeTrue())
	})
})
