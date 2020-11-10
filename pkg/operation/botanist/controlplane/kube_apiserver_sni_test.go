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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
			Hosts:              []string{"foo.bar"},
			ApiserverClusterIP: "1.1.1.1",
			IstioIngressGateway: IstioIngressGateway{
				Namespace: "istio-foo",
				Labels:    map[string]string{"foo": "bar"},
			},
			Name:         deployName,
			NamespaceUID: types.UID("123456"),
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

var _ = Describe("#AnyDeployedSNI", func() {
	var (
		ctx      context.Context
		c        client.Client
		createVS = func(name string, namespace string) *unstructured.Unstructured {
			return &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "networking.istio.io/v1beta1",
					"kind":       "VirtualService",
					"metadata": map[string]interface{}{
						"name":      name,
						"namespace": namespace,
					},
				},
			}
		}
	)

	Context("CRD available", func() {
		BeforeEach(func() {
			ctx = context.TODO()
			s := runtime.NewScheme()
			// TODO(mvladev): can't directly import the istio apis due to dependency issues.
			s.AddKnownTypeWithName(schema.FromAPIVersionAndKind("networking.istio.io/v1beta1", "VirtualServiceList"), &unstructured.UnstructuredList{})
			s.AddKnownTypeWithName(schema.FromAPIVersionAndKind("networking.istio.io/v1beta1", "VirtualService"), &unstructured.Unstructured{})
			c = fake.NewFakeClientWithScheme(s)
		})

		It("returns true when exists", func() {
			Expect(c.Create(ctx, createVS("kube-apiserver", "test"))).NotTo(HaveOccurred())
			any, err := AnyDeployedSNI(ctx, c)

			Expect(err).NotTo(HaveOccurred())
			Expect(any).To(BeTrue())
		})

		It("returns false when does not exists", func() {
			any, err := AnyDeployedSNI(ctx, c)

			Expect(err).NotTo(HaveOccurred())
			Expect(any).To(BeFalse())
		})
	})

	Context("CRD not available", func() {
		var (
			ctrl   *gomock.Controller
			client *mockclient.MockClient
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			client = mockclient.NewMockClient(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("returns false", func() {
			client.EXPECT().List(ctx, gomock.AssignableToTypeOf(&unstructured.UnstructuredList{}), gomock.Any()).Return(&meta.NoKindMatchError{})
			any, err := AnyDeployedSNI(ctx, client)

			Expect(err).NotTo(HaveOccurred())
			Expect(any).To(BeFalse())
		})
	})
})
