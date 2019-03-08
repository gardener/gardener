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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"k8s.io/client-go/rest"

	"github.com/onsi/gomega/types"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/controllermanager/controller/plant"
	mockdiscovery "github.com/gardener/gardener/pkg/mock/client-go/discovery"
	mockrest "github.com/gardener/gardener/pkg/mock/client-go/rest"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mocknodelister "github.com/gardener/gardener/pkg/mock/gardener/utils/kubernetes"
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

func makeNodeWithProvider(provider string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testNode",
		},
		Spec: corev1.NodeSpec{
			ProviderID: fmt.Sprintf("%s://zone/id", provider),
		},
	}
}
func hasConditonTrue(cond *gardencorev1alpha1.Condition) bool {
	return cond.Status == corev1.ConditionTrue
}

var _ = Describe("Plant", func() {
	var (
		ctrl     *gomock.Controller
		baseNode *corev1.Node
	)

	BeforeSuite(func() {

	})
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})
	AfterEach(func() {
		ctrl.Finish()
	})

	Context("Utils", func() {
		It("should fetch cloud provider")
	})
	Context("HealthChecker", func() {
		var (
			healthChecker *plant.HealthChecker
			c             = mockclient.NewMockClient(ctrl)
		)

		DescribeTable("checkAPIServerAvailablility", func(response *http.Response, matcher types.GomegaMatcher) {
			var (
				apiServerAvailable  = helper.InitCondition(gardencorev1alpha1.PlantAPIServerAvailable)
				discoveryMockclient = mockdiscovery.NewMockDiscoveryInterface(ctrl)
				restMockClient      = mockrest.NewMockInterface(ctrl)
				httpMockClient      = mockrest.NewMockHTTPClient(ctrl)
				body                = mockio.NewMockReadCloser(ctrl)
				healthChecker       = plant.NewHealthCheker(c, discoveryMockclient)

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
			Expect(hasConditonTrue(&actual)).To(matcher)

		},
			Entry("bad response", &http.Response{StatusCode: http.StatusOK}, BeTrue()),
			Entry("bad response", &http.Response{StatusCode: http.StatusNotFound}, BeFalse()),
		)
		DescribeTable("checkClusterNodes",
			func(node *corev1.Node, errMatcher, caseMatcher types.GomegaMatcher) {
				var (
					healthyNodes        = helper.InitCondition(gardencorev1alpha1.PlantEveryNodeReady)
					nodeLister          = mocknodelister.NewMockNodeLister(ctrl)
					discoveryMockclient = mockdiscovery.NewMockDiscoveryInterface(ctrl)
				)

				healthChecker = plant.NewHealthCheker(c, discoveryMockclient)
				nodeLister.EXPECT().List(gomock.Any()).Return([]*corev1.Node{
					node,
				}, nil)

				actual, err := healthChecker.CheckPlantClusterNodes(&healthyNodes, nodeLister)
				Expect(err).To(errMatcher)
				Expect(hasConditonTrue(actual)).To(caseMatcher)
			},
			Entry("healthy cluster nodes", &corev1.Node{
				Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}},
			}, BeNil(), BeTrue()),
			Entry("no ready condition", &corev1.Node{}, BeNil(), Not(BeTrue())),
			Entry("ready condition not indicating true", &corev1.Node{
				Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}}},
			}, BeNil(), Not(BeTrue())),
		)
	})
})
