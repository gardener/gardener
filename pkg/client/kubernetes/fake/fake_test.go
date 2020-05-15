// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fake_test

import (
	"context"

	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	"k8s.io/client-go/discovery"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	apiregistrationfake "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"

	"github.com/gardener/gardener/pkg/chartrenderer"
	gardencorefake "github.com/gardener/gardener/pkg/client/core/clientset/versioned/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/mock/apimachinery/api/meta"
	mockcache "github.com/gardener/gardener/pkg/mock/controller-runtime/cache"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/rest"
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
		applier := kubernetes.NewMockApplier(ctrl)
		cs := builder.WithApplier(applier).Build()

		Expect(cs.Applier()).To(BeIdenticalTo(applier))
	})

	It("should correctly set chartRenderer attribute", func() {
		chartRenderer := chartrenderer.NewWithServerVersion(&version.Info{Major: "1", Minor: "18"})
		cs := builder.WithChartRenderer(chartRenderer).Build()

		Expect(cs.ChartRenderer()).To(BeIdenticalTo(chartRenderer))
	})

	It("should correctly set chartApplier attribute", func() {
		chartApplier := kubernetes.NewMockChartApplier(ctrl)
		cs := builder.WithChartApplier(chartApplier).Build()

		Expect(cs.ChartApplier()).To(BeIdenticalTo(chartApplier))
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

	It("should correctly set directClient attribute", func() {
		directClient := mockclient.NewMockClient(ctrl)
		cs := builder.WithDirectClient(directClient).Build()

		Expect(cs.DirectClient()).To(BeIdenticalTo(directClient))
	})

	It("should correctly set cache attribute", func() {
		cache := mockcache.NewMockCache(ctrl)
		cs := builder.WithCache(cache).Build()

		Expect(cs.Cache()).To(BeIdenticalTo(cache))
	})

	It("should correctly set restMapper attribute", func() {
		restMapper := meta.NewMockRESTMapper(ctrl)
		cs := builder.WithRESTMapper(restMapper).Build()

		Expect(cs.RESTMapper()).To(BeIdenticalTo(restMapper))
	})

	It("should correctly set kubernetes attribute", func() {
		kubernetes := kubernetesfake.NewSimpleClientset()
		cs := builder.WithKubernetes(kubernetes).Build()

		Expect(cs.Kubernetes()).To(BeIdenticalTo(kubernetes))
	})

	It("should correctly set gardenCore attribute", func() {
		gardenCore := gardencorefake.NewSimpleClientset()
		cs := builder.WithGardenCore(gardenCore).Build()

		Expect(cs.GardenCore()).To(BeIdenticalTo(gardenCore))
	})

	It("should correctly set apiextension attribute", func() {
		apiextension := apiextensionsfake.NewSimpleClientset()
		cs := builder.WithAPIExtension(apiextension).Build()

		Expect(cs.APIExtension()).To(BeIdenticalTo(apiextension))
	})

	It("should correctly set apiregistration attribute", func() {
		apiregistration := apiregistrationfake.NewSimpleClientset()
		cs := builder.WithAPIRegistration(apiregistration).Build()

		Expect(cs.APIRegistration()).To(BeIdenticalTo(apiregistration))
	})

	It("should correctly set restClient attribute", func() {
		disc, err := discovery.NewDiscoveryClientForConfig(&rest.Config{})
		Expect(err).NotTo(HaveOccurred())
		restClient := disc.RESTClient()
		cs := builder.WithRESTClient(restClient).Build()

		Expect(cs.RESTClient()).To(BeIdenticalTo(restClient))
	})

	It("should correctly set version attribute", func() {
		version := "1.18.0"
		cs := builder.WithVersion(version).Build()

		Expect(cs.Version()).To(Equal(version))
	})

	It("should do nothing on Start", func() {
		cs := builder.Build()

		cs.Start(context.Background().Done())
	})

	It("should do nothing on WaitForCacheSync", func() {
		cs := builder.Build()

		Expect(cs.WaitForCacheSync(context.Background().Done())).To(BeTrue())
	})

	It("should do nothing on ForwardPodPort", func() {
		cs := builder.Build()

		ch, err := cs.ForwardPodPort("", "", 0, 0)
		Expect(ch).To(BeNil())
		Expect(err).NotTo(HaveOccurred())
	})

	It("should do nothing on CheckForwardPodPort", func() {
		cs := builder.Build()

		Expect(cs.CheckForwardPodPort("", "", 0, 0)).To(Succeed())
	})

})
