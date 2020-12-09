// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package chart

import (
	"context"
	"testing"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mockkubernetes "github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	namespace    = "test-namespace"
	chartName    = "test"
	chartPath    = "test-path"
	subChartName = "test-subchart"

	targetVersion = "1.0"
)

func TestChart(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Chart Suite")
}

var _ = Describe("Chart", func() {
	var (
		ctrl *gomock.Controller

		c            *mockclient.MockClient
		chartApplier *mockkubernetes.MockChartApplier

		ctx = context.TODO()

		imgSource1 = &imagevector.ImageSource{
			Name:       "img1",
			Repository: "repo1",
		}
		imgSource2 = &imagevector.ImageSource{
			Name:       "img2",
			Repository: "repo2",
		}
		imageVector = imagevector.ImageVector{imgSource1, imgSource2}

		chart = Chart{
			Name:   chartName,
			Path:   chartPath,
			Images: []string{imgSource1.Name},
			Objects: []*Object{
				{Type: &corev1.Secret{}, Name: chartName},
			},
			SubCharts: []*SubChart{
				{
					Chart: Chart{
						Name:   subChartName,
						Images: []string{imgSource2.Name},
						Objects: []*Object{
							{Type: &corev1.Secret{}, Name: subChartName},
						},
					},
					Condition: subChartName + ".enabled",
				},
			},
		}

		chartSecretKey = client.ObjectKey{Namespace: namespace, Name: chartName}
		chartSecret    = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: chartName, Namespace: namespace},
			Data:       map[string][]byte{"foo": []byte("bar")},
		}

		subChartSecretKey = client.ObjectKey{Namespace: namespace, Name: subChartName}
		subChartSecret    = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: subChartName, Namespace: namespace},
			Data:       map[string][]byte{"foo": []byte("bar")},
		}

		values = func(enabled bool) map[string]interface{} {
			return map[string]interface{}{
				"foo": "bar",
				subChartName: map[string]interface{}{
					"enabled": enabled,
				},
			}
		}
		mergedValues = func(enabled bool) map[string]interface{} {
			return map[string]interface{}{
				"foo": "bar",
				"images": map[string]interface{}{
					imgSource1.Name: imgSource1.ToImage(pointer.StringPtr(targetVersion)).String(),
				},
				subChartName: map[string]interface{}{
					"enabled": enabled,
					"images": map[string]interface{}{
						imgSource2.Name: imgSource2.ToImage(pointer.StringPtr(targetVersion)).String(),
					},
				},
			}
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		chartApplier = mockkubernetes.NewMockChartApplier(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Apply", func() {
		It("should apply the chart with correct values", func() {
			chartApplier.EXPECT().Apply(ctx, chartPath, namespace, chartName, kubernetes.Values(mergedValues(true)))

			err := chart.Apply(ctx, chartApplier, c, namespace, imageVector, "", targetVersion, values(true))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should apply the chart with correct values and delete the disabled subcharts", func() {
			chartApplier.EXPECT().Apply(ctx, chartPath, namespace, chartName, kubernetes.Values(mergedValues(false)))
			c.EXPECT().Get(ctx, subChartSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(subChartSecret))
			c.EXPECT().Delete(ctx, subChartSecret).Return(nil)

			err := chart.Apply(ctx, chartApplier, c, namespace, imageVector, "", targetVersion, values(false))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#Delete", func() {
		It("should delete the chart and its subcharts", func() {
			c.EXPECT().Get(ctx, subChartSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(subChartSecret))
			c.EXPECT().Delete(ctx, subChartSecret).Return(nil)
			c.EXPECT().Get(ctx, chartSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(chartSecret))
			c.EXPECT().Delete(ctx, chartSecret).Return(nil)

			err := chart.Delete(ctx, c, namespace)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("ImageMapToValues", func() {
		It("should transform the given image map to values", func() {
			var (
				img1 = &imagevector.Image{
					Name:       "img1",
					Repository: "repo1",
				}
				img2 = &imagevector.Image{
					Name:       "img2",
					Repository: "repo2",
				}
			)

			values := ImageMapToValues(map[string]*imagevector.Image{
				img1.Name: img1,
				img2.Name: img2,
			})
			Expect(values).To(Equal(map[string]interface{}{
				img1.Name: img1.String(),
				img2.Name: img2.String(),
			}))
		})
	})

	Describe("#InjectImages", func() {
		It("should find the images and inject the image as value map at the 'images' key into a shallow copy", func() {
			injected, err := InjectImages(nil, imageVector, []string{imgSource1.Name, imgSource2.Name})
			Expect(err).NotTo(HaveOccurred())
			Expect(injected).To(Equal(map[string]interface{}{
				"images": map[string]interface{}{
					imgSource1.Name: imgSource1.ToImage(nil).String(),
					imgSource2.Name: imgSource2.ToImage(nil).String(),
				},
			}))
		})
	})

	Describe("#CopyValues", func() {
		It("should create a shallow copy of the map", func() {
			v := map[string]interface{}{"foo": nil, "bar": map[string]interface{}{"baz": nil}}

			c := CopyValues(v)

			Expect(c).To(Equal(v))

			c["foo"] = 1
			Expect(v["foo"]).To(BeNil())

			c["bar"].(map[string]interface{})["baz"] = "bang"
			Expect(v["bar"].(map[string]interface{})["baz"]).To(Equal("bang"))
		})
	})
})

func clientGet(result runtime.Object) interface{} {
	return func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
		switch obj.(type) {
		case *corev1.Secret:
			*obj.(*corev1.Secret) = *result.(*corev1.Secret)
		}
		return nil
	}
}
