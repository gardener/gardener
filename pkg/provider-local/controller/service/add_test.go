// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	DescribeTable("#HasNodesInMultipleZones",
		func(nodes []string, zones []string, funcs *interceptor.Funcs, expectedResult bool, expectedError error) {
			c := fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			Expect(nodes).To(HaveLen(len(zones)))
			for i := range nodes {
				Expect(c.Create(context.TODO(), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodes[i], Labels: map[string]string{"topology.kubernetes.io/zone": zones[i]}}})).To(Succeed(), fmt.Sprintf("creation of %d. node failed", i))
			}

			cl := c
			if funcs != nil {
				cl = interceptor.NewClient(c, *funcs)
			}
			result, err := HasNodesInMultipleZones(context.TODO(), cl)
			Expect(result).To(Equal(expectedResult))
			if expectedError == nil {
				Expect(err).To(Succeed())
			} else {
				Expect(err).To(Equal(expectedError))
			}
		},

		Entry("No nodes/zones", []string{}, []string{}, nil, false, nil),
		Entry("1 node/zone", []string{"node-0"}, []string{"0"}, nil, false, nil),
		Entry("2 nodes, 1 zone", []string{"node-0", "node-1"}, []string{"0", "0"}, nil, false, nil),
		Entry("2 nodes/zones", []string{"node-0", "node-1"}, []string{"0", "1"}, nil, true, nil),
		Entry("3 nodes, 1 zones", []string{"node-0", "node-1", "node-2"}, []string{"0", "0", "0"}, nil, false, nil),
		Entry("3 nodes, 2 zones", []string{"node-0", "node-1", "node-2"}, []string{"0", "1", "1"}, nil, true, nil),
		Entry("3 nodes/zones", []string{"node-0", "node-1", "node-2"}, []string{"0", "1", "2"}, nil, true, nil),
		Entry("No nodes/zones with errors", []string{}, []string{}, createErrorClientInterceptor(), false, errInterceptor),
		Entry("3 nodes/zones with errors", []string{"node-0", "node-1", "node-2"}, []string{"0", "1", "2"}, createErrorClientInterceptor(), false, errInterceptor),
	)
})

var errInterceptor = fmt.Errorf("temporary test error")

func createErrorClientInterceptor() *interceptor.Funcs {
	return &interceptor.Funcs{
		List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
			return errInterceptor
		},
	}
}
