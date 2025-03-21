// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker_test

import (
	"context"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/extensions/pkg/controller/worker"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("Mapper", func() {
	var (
		ctx = context.TODO()

		namespace = "some-namespace"

		worker  *extensionsv1alpha1.Worker
		machine *machinev1alpha1.Machine
	)

	BeforeEach(func() {
		worker = &extensionsv1alpha1.Worker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker",
				Namespace: namespace,
			},
		}

		machine = &machinev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine",
				Namespace: namespace,
				Labels: map[string]string{
					"worker.gardener.cloud/name": worker.Name,
				},
			},
		}
	})

	Describe("#MachineToWorkerMapper", func() {
		var mapper handler.MapFunc

		BeforeEach(func() {
			mapper = MachineToWorkerMapper()
		})

		It("should return nil when the object is not a Machine", func() {
			Expect(mapper(ctx, &corev1.Secret{})).To(BeNil())
		})

		It("should return nil when the machine does not have a worker label", func() {
			delete(machine.Labels, "worker.gardener.cloud/name")

			Expect(mapper(ctx, machine)).To(BeNil())
		})

		It("should map the machine to the worker", func() {
			Expect(mapper(ctx, machine)).To(ConsistOf(
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      worker.Name,
						Namespace: namespace,
					},
				}))
		})
	})
})
