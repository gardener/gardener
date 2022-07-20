// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed_test

import (
	"context"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/seed"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	schedulingv1 "k8s.io/api/scheduling/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("Seed Control", func() {
	var (
		ctx        context.Context
		seedClient client.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		logf.IntoContext(ctx, logr.Discard())
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
	})

	Describe("#Legacy PriorityClass Cleanup", func() {
		BeforeEach(func() {
			pcNames := []string{"reversed-vpn-auth-server", "fluent-bit", "loki", "istio-ingressgateway", "istiod", "random"}
			for _, name := range pcNames {
				pc := &schedulingv1.PriorityClass{
					ObjectMeta: v1.ObjectMeta{
						Name: name,
					},
					Value: 1,
				}
				Expect(seedClient.Create(ctx, pc)).To(Succeed())
			}
		})

		Context("delete legacy priority classes", func() {
			It("should delete all legacy priority classes", func() {
				Expect(CleanupLegacyPriorityClasses(ctx, seedClient)).To(Succeed())
				time.Sleep(10 * time.Second)
				priorityClasses := &schedulingv1.PriorityClassList{}
				Expect(seedClient.List(ctx, priorityClasses)).To(Succeed())
				Expect(len(priorityClasses.Items)).To(Equal(1))
				Expect(priorityClasses.Items[0].Name).To(Equal("random"))
			})
		})
	})
})
