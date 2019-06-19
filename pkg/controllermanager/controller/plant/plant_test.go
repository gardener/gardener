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
package plant_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/logger"

	"k8s.io/apimachinery/pkg/version"

	"k8s.io/client-go/rest"

	"github.com/onsi/gomega/types"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/controllermanager/controller/plant"
	mockdiscovery "github.com/gardener/gardener/pkg/mock/client-go/discovery"
	mockrest "github.com/gardener/gardener/pkg/mock/client-go/rest"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mockio "github.com/gardener/gardener/pkg/mock/go/io"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPlant(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Plant Test Suite")
}

func makeNodeWithProvider(provider string, withLabels map[string]string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "testNode",
			Labels: withLabels,
		},
		Spec: corev1.NodeSpec{
			ProviderID: provider,
		},
	}
}

func hasConditonTrue(cond gardencorev1alpha1.Condition) bool {
	return cond.Status == gardencorev1alpha1.ConditionTrue
}

func hasConditionUnknown(cond gardencorev1alpha1.Condition) bool {
	return cond.Status == gardencorev1alpha1.ConditionUnknown
}

var _ = Describe("Plant", func() {
	var (
		ctrl     *gomock.Controller
		baseNode *corev1.Node
	)
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})
	AfterEach(func() {
		ctrl.Finish()
	})
	Context("Utils", func() {
		const (
			labelZoneRegion = corev1.LabelZoneRegion
			unKnown         = "<unknown>"
			k8sVersion      = "1.13.1"
			region          = "eu-west-1"
		)
		DescribeTable("should fetch cloud Info", func(mockNode corev1.Node, errMatcher types.GomegaMatcher, expectedInfo *plant.StatusCloudInfo) {
			var (
				discoveryMockclient = mockdiscovery.NewMockDiscoveryInterface(ctrl)
				runtimeClient       = mockclient.NewMockClient(ctrl)
				testLogger          = logger.NewFieldLogger(logger.NewLogger("info"), "test", "test-plant")
			)

			runtimeClient.EXPECT().List(context.TODO(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, list runtime.Object, opts ...client.ListOptionFunc) error {
				Expect(list).To(BeAssignableToTypeOf(&corev1.NodeList{}))
				list.(*corev1.NodeList).Items = []corev1.Node{mockNode}
				return nil
			})

			discoveryMockclient.EXPECT().ServerVersion().Return(&version.Info{GitVersion: "1.13.1"}, nil).AnyTimes()

			statusInfo, err := plant.FetchCloudInfo(context.TODO(), runtimeClient, discoveryMockclient, testLogger)
			Expect(err).To(errMatcher)
			Expect(statusInfo).To(Equal(expectedInfo))
		},
			Entry("It should return unknown if provider is not listed",
				makeNodeWithProvider("", map[string]string{labelZoneRegion: region}), BeNil(), &plant.StatusCloudInfo{CloudType: unKnown, K8sVersion: k8sVersion, Region: region}),
			Entry("It should return the provider successfully",
				makeNodeWithProvider("aws://zones.something", map[string]string{labelZoneRegion: region}), BeNil(), &plant.StatusCloudInfo{CloudType: "aws", K8sVersion: k8sVersion, Region: region}),
		)
	})
	Context("HealthChecker", func() {
		var (
			healthChecker       *plant.HealthChecker
			runtimeClient       = mockclient.NewMockClient(ctrl)
			discoveryMockclient *mockdiscovery.MockDiscoveryInterface
		)

		BeforeEach(func() {
			discoveryMockclient = mockdiscovery.NewMockDiscoveryInterface(ctrl)
		})

		DescribeTable("checkAPIServerAvailablility", func(response *http.Response, matcher types.GomegaMatcher) {
			var (
				apiServerAvailable = helper.InitCondition(gardencorev1alpha1.PlantAPIServerAvailable)
				restMockClient     = mockrest.NewMockInterface(ctrl)
				httpMockClient     = mockrest.NewMockHTTPClient(ctrl)
				body               = mockio.NewMockReadCloser(ctrl)
				healthChecker      = plant.NewHealthChecker(runtimeClient, discoveryMockclient)

				request = rest.NewRequest(httpMockClient, http.MethodGet, &url.URL{}, "", rest.ContentConfig{}, rest.Serializers{}, nil, nil, 0)
			)

			response.Body = body

			gomock.InOrder(
				discoveryMockclient.EXPECT().RESTClient().Return(restMockClient),
				restMockClient.EXPECT().Get().Return(request.AbsPath("/healthz")),
				httpMockClient.EXPECT().Do(gomock.Any()).Return(response, nil),
				body.EXPECT().Read(gomock.Any()).Return(0, io.EOF).AnyTimes(),
				body.EXPECT().Close(),
			)
			_ = baseNode
			actual := healthChecker.CheckAPIServerAvailability(apiServerAvailable)
			Expect(hasConditonTrue(actual)).To(matcher)

		},
			Entry("bad response", &http.Response{StatusCode: http.StatusOK}, BeTrue()),
			Entry("bad response", &http.Response{StatusCode: http.StatusNotFound}, BeFalse()),
		)
		DescribeTable("checkClusterNodes",
			func(node *corev1.Node, caseMatcher types.GomegaMatcher) {
				var (
					conditionEveryNodeReady = helper.InitCondition(gardencorev1alpha1.PlantEveryNodeReady)
					runtimeClient           = mockclient.NewMockClient(ctrl)
				)

				healthChecker = plant.NewHealthChecker(runtimeClient, discoveryMockclient)
				runtimeClient.EXPECT().List(context.TODO(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, list runtime.Object, opts ...client.ListOptionFunc) error {
					Expect(list).To(BeAssignableToTypeOf(&corev1.NodeList{}))
					list.(*corev1.NodeList).Items = []corev1.Node{*node}
					return nil
				})

				condition := healthChecker.CheckPlantClusterNodes(context.TODO(), conditionEveryNodeReady)
				Expect(hasConditonTrue(condition)).To(caseMatcher)
			},
			Entry("healthy cluster nodes", &corev1.Node{
				Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}},
			}, BeTrue()),
			Entry("no ready condition", &corev1.Node{}, Not(BeTrue())),
			Entry("ready condition not indicating true", &corev1.Node{
				Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}}},
			}, Not(BeTrue())),
		)
		DescribeTable("checkClusterNodes - returns correct condition on failure",
			func(caseMatcher types.GomegaMatcher) {
				var (
					conditionEveryNodeReady = helper.InitCondition(gardencorev1alpha1.PlantEveryNodeReady)
					runtimeClient           = mockclient.NewMockClient(ctrl)
				)

				healthChecker = plant.NewHealthChecker(runtimeClient, discoveryMockclient)
				runtimeClient.EXPECT().List(context.TODO(), gomock.Any()).DoAndReturn(func(ctx context.Context, list runtime.Object, opts ...client.ListOptionFunc) error {
					return fmt.Errorf("Some Error")
				})

				condition := healthChecker.CheckPlantClusterNodes(context.TODO(), conditionEveryNodeReady)
				Expect(hasConditionUnknown(condition)).To(caseMatcher)
			},
			Entry("no healthy cluster nodes", BeTrue()),
		)
	})
})
