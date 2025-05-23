// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("Service", func() {
	Describe("#CheckService", func() {
		DescribeTable("services",
			func(service *corev1.Service, matcher types.GomegaMatcher) {
				err := health.CheckService(service)
				Expect(err).To(matcher)
			},
			Entry("no LoadBalancer service", &corev1.Service{
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName},
			}, BeNil()),
			Entry("LoadBalancer w/ ingress status", &corev1.Service{
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{Hostname: "foo.bar"},
						},
					},
				},
			}, BeNil()),
			Entry("LoadBalancer w/o ingress status", &corev1.Service{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
				Spec:     corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
			}, MatchError("service is missing ingress status")),
		)
	})
})
