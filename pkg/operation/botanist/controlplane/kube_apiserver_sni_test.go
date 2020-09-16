// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane_test

import (
	"context"
	"fmt"

	cr "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/controlplane"

	"github.com/golang/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("#KubeAPIServerSNI", func() {
	const (
		deployNS   = "test-chart-namespace"
		deployName = "test-deploy"
	)
	var (
		ca               kubernetes.ChartApplier
		ctx              context.Context
		c                client.Client
		defaultDepWaiter component.DeployWaiter
	)

	BeforeEach(func() {
		ctx = context.TODO()
		s := runtime.NewScheme()
		c = fake.NewFakeClientWithScheme(s)
	})

	JustBeforeEach(func() {
		ca = kubernetes.NewChartApplier(
			cr.NewWithServerVersion(&version.Info{}),
			kubernetes.NewApplier(c, meta.NewDefaultRESTMapper([]schema.GroupVersion{})),
		)

		defaultDepWaiter = NewKubeAPIServerSNI(&KubeAPIServerSNIValues{
			Hosts:                 []string{"foo.bar"},
			ApiserverClusterIP:    "1.1.1.1",
			IstioIngressNamespace: "istio-foo",
			Name:                  deployName,
			NamespaceUID:          types.UID("123456"),
		}, deployNS, ca, chartsRoot())
	})

	It("deploys succeeds", func() {
		Expect(defaultDepWaiter.Deploy(ctx)).ToNot(HaveOccurred())
	})

	It("destroy succeeds", func() {
		Expect(defaultDepWaiter.Deploy(ctx)).ToNot(HaveOccurred())
		Expect(defaultDepWaiter.Destroy(ctx)).ToNot(HaveOccurred())
	})

	It("wait succeeds", func() {
		Expect(defaultDepWaiter.Deploy(ctx)).ToNot(HaveOccurred())
		Expect(defaultDepWaiter.Wait(ctx)).ToNot(HaveOccurred())
	})

	Context("destroy", func() {

		Context("applier returns an error", func() {
			var (
				ctrl *gomock.Controller
				mc   *mockclient.MockClient
			)

			BeforeEach(func() {
				ctrl = gomock.NewController(GinkgoT())
				mc = mockclient.NewMockClient(ctrl)
				c = mc
			})

			AfterEach(func() {
				ctrl.Finish()
			})

			It("destroy succeeds when returning no match error", func() {
				mc.EXPECT().Delete(gomock.Any(), gomock.Any()).AnyTimes().Return(
					&meta.NoResourceMatchError{PartialResource: schema.GroupVersionResource{}},
				)
				Expect(defaultDepWaiter.Destroy(ctx)).ToNot(HaveOccurred())
			})

			It("destroy succeeds when returning not found error", func() {
				mc.EXPECT().Delete(gomock.Any(), gomock.Any()).AnyTimes().Return(
					apierrors.NewNotFound(schema.GroupResource{}, "foo"),
				)
				Expect(defaultDepWaiter.Destroy(ctx)).ToNot(HaveOccurred())
			})

			It("destroy fails when returning internal server error", func() {
				mc.EXPECT().Delete(gomock.Any(), gomock.Any()).AnyTimes().Return(
					apierrors.NewInternalError(fmt.Errorf("bad")),
				)
				Expect(defaultDepWaiter.Destroy(ctx)).To(HaveOccurred())
			})
		})

		It("destroy succeeds", func() {
			Expect(defaultDepWaiter.Deploy(ctx)).ToNot(HaveOccurred())
			Expect(defaultDepWaiter.Destroy(ctx)).ToNot(HaveOccurred())
			Expect(defaultDepWaiter.WaitCleanup(ctx)).ToNot(HaveOccurred())
		})
	})
})
