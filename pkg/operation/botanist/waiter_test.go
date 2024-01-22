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

package botanist_test

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
)

var _ = Describe("Waiter", func() {
	var (
		botanist *Botanist

		ctx = context.Background()
	)

	BeforeEach(func() {
		shootClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
		shootClientSet := kubernetesfake.NewClientSetBuilder().WithClient(shootClient).Build()

		botanist = &Botanist{
			Operation: &operation.Operation{
				Logger:         logr.Discard(),
				ShootClientSet: shootClientSet,
			},
		}
	})

	Describe("#WaitUntilNoPodRunning", func() {
		var (
			pod *corev1.Pod
		)

		BeforeEach(func() {
			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "infinity-pod",
				},
			}
		})

		It("should return ok when no pod is found", func() {
			Expect(botanist.WaitUntilNoPodRunning(ctx)).To(Succeed())
		})

		It("should return ok when no pod is running", func() {
			ctxCanceled, cancel := context.WithCancel(ctx)
			cancel()

			pod.Status = corev1.PodStatus{Phase: corev1.PodFailed}

			Expect(botanist.ShootClientSet.Client().Create(ctxCanceled, pod)).To(Succeed())

			Expect(botanist.WaitUntilNoPodRunning(ctxCanceled)).To(Succeed())
		})

		It("should return an error when a pod is running in non system namespace", func() {
			ctxCanceled, cancel := context.WithCancel(ctx)
			cancel()

			pod.Status = corev1.PodStatus{Phase: corev1.PodRunning}
			pod.Namespace = "foo"

			Expect(botanist.ShootClientSet.Client().Create(ctxCanceled, pod)).To(Succeed())

			err := botanist.WaitUntilNoPodRunning(ctxCanceled)

			var coder v1beta1helper.Coder
			Expect(errors.As(err, &coder)).To(BeTrue())

			Expect(coder.Codes()[0]).To(Equal(gardencorev1beta1.ErrorCleanupClusterResources))
		})

		It("should return an error when a pod is running in system namespace", func() {
			ctxCanceled, cancel := context.WithCancel(ctx)
			cancel()

			pod.Status = corev1.PodStatus{Phase: corev1.PodRunning}
			pod.Namespace = metav1.NamespaceSystem

			Expect(botanist.ShootClientSet.Client().Create(ctxCanceled, pod)).To(Succeed())

			err := botanist.WaitUntilNoPodRunning(ctxCanceled)

			var coder v1beta1helper.Coder
			Expect(errors.As(err, &coder)).To(BeFalse())

			Expect(err).To(MatchError("retry failed with context canceled, last error: waiting until there are no running Pods in the shoot cluster ... there is still at least one running Pod in the shoot cluster: \"kube-system/infinity-pod\""))
		})
	})
})
