// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package service_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	. "github.com/gardener/gardener/pkg/provider-local/controller/service"
)

var _ = Describe("Add", func() {
	DescribeTable("NodeZoneInitializer#HasNodesInMultipleZones",
		func(nodes []string, zones []string, funcs *interceptor.Funcs, expectedResult []bool, expectedError []error) {
			c := fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			Expect(nodes).To(HaveLen(len(zones)))
			for i := range nodes {
				Expect(c.Create(context.TODO(), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodes[i], Labels: map[string]string{"topology.kubernetes.io/zone": zones[i]}}})).To(Succeed(), fmt.Sprintf("creation of %d. node failed", i))
			}

			nzi := &NodeZoneInitializer{}
			Expect(expectedResult).To(HaveLen(len(expectedError)))
			for i := range expectedResult {
				cl := c
				if funcs != nil {
					cl = interceptor.NewClient(c, *funcs)
				}
				result, err := nzi.HasNodesInMultipleZones(context.TODO(), cl)
				Expect(result).To(Equal(expectedResult[i]), fmt.Sprintf("failure on %d. iteration: expected=%t, got=%t", i, expectedResult[i], result))
				if expectedError[i] == nil {
					Expect(err).To(Succeed(), fmt.Sprintf("failure on %d. iteration: expected=%s, got=%s", i, expectedError[i], err))
				} else {
					Expect(err).To(Equal(expectedError[i]), fmt.Sprintf("failure on %d. iteration: expected=%s, got=%s", i, expectedError[i], err))
				}
			}
		},

		Entry("No nodes/zones", []string{}, []string{}, nil, []bool{false, false}, []error{nil, nil}),
		Entry("1 node/zone", []string{"node-0"}, []string{"0"}, nil, []bool{false, false}, []error{nil, nil}),
		Entry("2 nodes, 1 zone", []string{"node-0", "node-1"}, []string{"0", "0"}, nil, []bool{false, false}, []error{nil, nil}),
		Entry("2 nodes/zones", []string{"node-0", "node-1"}, []string{"0", "1"}, nil, []bool{true, true}, []error{nil, nil}),
		Entry("3 nodes, 1 zones", []string{"node-0", "node-1", "node-2"}, []string{"0", "0", "0"}, nil, []bool{false, false}, []error{nil, nil}),
		Entry("3 nodes, 2 zones", []string{"node-0", "node-1", "node-2"}, []string{"0", "1", "1"}, nil, []bool{true, true}, []error{nil, nil}),
		Entry("3 nodes/zones", []string{"node-0", "node-1", "node-2"}, []string{"0", "1", "2"}, nil, []bool{true, true}, []error{nil, nil}),
		Entry("No nodes/zones with errors", []string{}, []string{}, createAlwaysErrorClientInterceptor(), []bool{false, false, false}, []error{errInterceptor, errInterceptor, errInterceptor}),
		Entry("1 node/zone with errors", []string{"node-0"}, []string{"0"}, createAlwaysErrorClientInterceptor(), []bool{false, false, false}, []error{errInterceptor, errInterceptor, errInterceptor}),
		Entry("2 nodes, 1 zone with errors", []string{"node-0", "node-1"}, []string{"0", "0"}, createAlwaysErrorClientInterceptor(), []bool{false, false, false}, []error{errInterceptor, errInterceptor, errInterceptor}),
		Entry("2 nodes/zones with errors", []string{"node-0", "node-1"}, []string{"0", "1"}, createAlwaysErrorClientInterceptor(), []bool{false, false, false}, []error{errInterceptor, errInterceptor, errInterceptor}),
		Entry("3 nodes, 1 zones with errors", []string{"node-0", "node-1", "node-2"}, []string{"0", "0", "0"}, createAlwaysErrorClientInterceptor(), []bool{false, false, false}, []error{errInterceptor, errInterceptor, errInterceptor}),
		Entry("3 nodes, 2 zones with errors", []string{"node-0", "node-1", "node-2"}, []string{"0", "1", "1"}, createAlwaysErrorClientInterceptor(), []bool{false, false, false}, []error{errInterceptor, errInterceptor, errInterceptor}),
		Entry("3 nodes/zones with errors", []string{"node-0", "node-1", "node-2"}, []string{"0", "1", "2"}, createAlwaysErrorClientInterceptor(), []bool{false, false, false}, []error{errInterceptor, errInterceptor, errInterceptor}),
		Entry("No nodes/zones with one error", []string{}, []string{}, createOnceErrorClientInterceptor(), []bool{false, false, false}, []error{errInterceptor, nil, nil}),
		Entry("1 node/zone with one error", []string{"node-0"}, []string{"0"}, createOnceErrorClientInterceptor(), []bool{false, false, false}, []error{errInterceptor, nil, nil}),
		Entry("2 nodes, 1 zone with one error", []string{"node-0", "node-1"}, []string{"0", "0"}, createOnceErrorClientInterceptor(), []bool{false, false, false}, []error{errInterceptor, nil, nil}),
		Entry("2 nodes/zones with one error", []string{"node-0", "node-1"}, []string{"0", "1"}, createOnceErrorClientInterceptor(), []bool{false, true, true}, []error{errInterceptor, nil, nil}),
		Entry("3 nodes, 1 zones with one error", []string{"node-0", "node-1", "node-2"}, []string{"0", "0", "0"}, createOnceErrorClientInterceptor(), []bool{false, false, false}, []error{errInterceptor, nil, nil}),
		Entry("3 nodes, 2 zones with one error", []string{"node-0", "node-1", "node-2"}, []string{"0", "1", "1"}, createOnceErrorClientInterceptor(), []bool{false, true, true}, []error{errInterceptor, nil, nil}),
		Entry("3 nodes/zones with one error", []string{"node-0", "node-1", "node-2"}, []string{"0", "1", "2"}, createOnceErrorClientInterceptor(), []bool{false, true, true}, []error{errInterceptor, nil, nil}),
	)
})

var errInterceptor = fmt.Errorf("temporary test error")

func createAlwaysErrorClientInterceptor() *interceptor.Funcs {
	return &interceptor.Funcs{
		List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
			return errInterceptor
		},
	}
}

func createOnceErrorClientInterceptor() *interceptor.Funcs {
	result := &interceptor.Funcs{}
	result.List = func(ctx context.Context, client client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
		result.List = nil
		return errInterceptor
	}
	return result
}
