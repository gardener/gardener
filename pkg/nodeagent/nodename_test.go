// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package nodeagent_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/pkg/nodeagent"
)

var _ = Describe("NodeName", func() {
	Describe("#FetchNodeByHostName", func() {
		var (
			ctx        = context.Background()
			fakeClient client.Client
			hostName   = "foo"

			node *corev1.Node
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()

			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "node-",
					Labels:       map[string]string{"kubernetes.io/hostname": hostName},
				},
			}
		})

		It("should return nil because no node was found", func() {
			node, err := FetchNodeByHostName(ctx, fakeClient, hostName)
			Expect(err).NotTo(HaveOccurred())
			Expect(node).To(BeNil())
		})

		It("should return the found node", func() {
			Expect(fakeClient.Create(ctx, node)).To(Succeed())
			DeferCleanup(func() { Expect(fakeClient.Delete(ctx, node)).To(Succeed()) })

			node, err := FetchNodeByHostName(ctx, fakeClient, hostName)
			Expect(err).NotTo(HaveOccurred())
			Expect(node).To(Equal(node))
		})

		It("should return an error because multiple nodes were found", func() {
			node2 := node.DeepCopy()

			Expect(fakeClient.Create(ctx, node)).To(Succeed())
			DeferCleanup(func() { Expect(fakeClient.Delete(ctx, node)).To(Succeed()) })

			Expect(fakeClient.Create(ctx, node2)).To(Succeed())
			DeferCleanup(func() { Expect(fakeClient.Delete(ctx, node2)).To(Succeed()) })

			node, err := FetchNodeByHostName(ctx, fakeClient, hostName)
			Expect(err).To(MatchError(ContainSubstring("found more than one node with label")))
			Expect(node).To(BeNil())
		})
	})
})
