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

package kubernetes_test

import (
	"context"
	"path/filepath"

	cr "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/test"
	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/helm/pkg/engine"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/test/gomega"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("chart applier", func() {
	const (
		cn     = "test-chart-name"
		cns    = "test-chart-namespace"
		cmName = "test-configmap-name"
	)
	var (
		cp         = filepath.Join("testdata", "render-test")
		ctrl       *gomock.Controller
		ca         kubernetes.ChartApplier
		ctx        context.Context
		c          client.Client
		expectedCM *corev1.ConfigMap
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.TODO()

		c = fake.NewFakeClient()
		d := &test.FakeDiscovery{
			GroupListFn: func() *metav1.APIGroupList {
				return &metav1.APIGroupList{
					Groups: []metav1.APIGroup{v1Group},
				}
			},
			ResourceMapFn: func() map[string]*metav1.APIResourceList {
				return map[string]*metav1.APIResourceList{
					"v1": {
						GroupVersion: "v1",
						APIResources: []metav1.APIResource{configMapAPIResource},
					},
				}
			},
		}

		expectedCM = &corev1.ConfigMap{
			TypeMeta: configMapTypeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name:            cmName,
				Namespace:       cns,
				ResourceVersion: "1",
				Labels:          map[string]string{"baz": "goo"},
			},
			Data: map[string]string{"key": "valz"},
		}

		cap, err := cr.DiscoverCapabilities(d)
		Expect(err).ToNot(HaveOccurred())

		renderer := cr.New(engine.New(), cap)
		a, err := test.NewTestApplier(c, d)
		Expect(err).ToNot(HaveOccurred())
		ca = kubernetes.NewChartApplier(renderer, a)
		Expect(ca).NotTo(BeNil(), "should return chart applier")
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Apply", func() {

		It("renders the chart with default values", func() {
			Expect(ca.Apply(ctx, cp, cns, cn)).ToNot(HaveOccurred())

			actual := &corev1.ConfigMap{}
			err := c.Get(context.TODO(), client.ObjectKey{Name: cmName, Namespace: cns}, actual)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(DeepDerivativeEqual(expectedCM))
		})

		It("renders the chart with custom values", func() {
			const (
				newNS = "other-namespace"
			)

			existingCM := &corev1.ConfigMap{
				TypeMeta: configMapTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmName,
					Namespace: newNS,
				},
			}

			Expect(c.Create(ctx, existingCM), "dummy configmap creation should succeed")

			expectedCM.Namespace = newNS
			expectedCM.Data = map[string]string{"key": "new-value"}
			expectedCM.Labels = map[string]string{"baz": "new-foo"}
			expectedCM.Annotations = map[string]string{"new": "new-value"}
			expectedCM.ResourceVersion = "2"

			val := &renderTestValues{
				Service: renderTestValuesService{
					Data:   map[string]string{"key": "new-value"},
					Labels: map[string]string{"baz": "new-foo"},
				},
			}

			m := kubernetes.MergeFuncs{
				corev1.SchemeGroupVersion.WithKind("ConfigMap").GroupKind(): func(newObj, oldObj *unstructured.Unstructured) {
					newObj.SetAnnotations(map[string]string{"new": "new-value"})
				},
			}

			Expect(ca.Apply(
				ctx,
				cp,
				newNS,
				cn,
				kubernetes.Values(val),
				kubernetes.ForceNamespace,
				nil, // simulate nil entry
				m,
				nil, // simulate nil entry
			)).ToNot(HaveOccurred())

			actual := &corev1.ConfigMap{}
			err := c.Get(context.TODO(), client.ObjectKey{Name: cmName, Namespace: newNS}, actual)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(DeepDerivativeEqual(expectedCM))
		})
	})

	Describe("#Delete", func() {

		It("deletes the chart with default values", func() {
			existingCM := &corev1.ConfigMap{
				TypeMeta: configMapTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmName,
					Namespace: cmName,
				},
			}
			Expect(c.Create(ctx, existingCM), "dummy configmap creation should succeed")

			Expect(ca.Delete(ctx, cp, cns, cn)).ToNot(HaveOccurred())

			actual := &corev1.ConfigMap{}
			err := c.Get(context.TODO(), client.ObjectKey{Name: cmName, Namespace: cns}, actual)

			Expect(err).To(BeNotFoundError())
		})

		It("deletes the chart with custom values", func() {
			const (
				newNS = "other-namespace"
			)

			existingCM := &corev1.ConfigMap{
				TypeMeta: configMapTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmName,
					Namespace: newNS,
				},
			}

			Expect(c.Create(ctx, existingCM), "dummy configmap creation should succeed")

			val := &renderTestValues{
				Service: renderTestValuesService{
					Data:   map[string]string{"key": "new-value"},
					Labels: map[string]string{"baz": "new-foo"},
				},
			}

			Expect(ca.Delete(
				ctx,
				cp,
				newNS,
				cn,
				kubernetes.Values(val),
				nil, // simulate nil entry
				kubernetes.ForceNamespace,
				nil, // simulate nil entry
			)).ToNot(HaveOccurred())

			actual := &corev1.ConfigMap{}
			err := c.Get(context.TODO(), client.ObjectKey{Name: cmName, Namespace: cns}, actual)

			Expect(err).To(BeNotFoundError())
		})
	})
})

type renderTestValues struct {
	Service renderTestValuesService `json:"service,omitempty"`
}

type renderTestValuesService struct {
	Labels map[string]string `json:"labels,omitempty"`
	Data   map[string]string `json:"data,omitempty"`
}
