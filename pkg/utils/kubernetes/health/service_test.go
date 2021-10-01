// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package health_test

import (
	"context"
	"fmt"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Service", func() {
	Describe("#CheckService", func() {
		var (
			ctrl    *gomock.Controller
			message = "foo"
		)
		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
		})
		AfterEach(func() {
			ctrl.Finish()
		})

		DescribeTable("services",
			func(service *corev1.Service, matcher types.GomegaMatcher) {
				scheme := kubernetesscheme.Scheme
				c := mockclient.NewMockClient(ctrl)
				c.EXPECT().Scheme().Return(kubernetesscheme.Scheme).AnyTimes()

				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.EventList{}), gomock.Any()).DoAndReturn(
					func(_ context.Context, list *corev1.EventList, _ ...client.ListOption) error {
						list.Items = []corev1.Event{
							{Message: message},
						}
						return nil
					},
				).MaxTimes(1)
				err := health.CheckService(context.Background(), scheme, c, service)
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
			}, MatchError(fmt.Sprintf("service is missing ingress status\n\n-> Events:\n*  reported <unknown> ago: %s", message))),
		)
	})
})
