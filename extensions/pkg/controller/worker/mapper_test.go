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
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/extensions/pkg/controller/worker"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

var _ = Describe("Mapper", func() {
	var (
		ctx = context.TODO()

		fakeClient client.Client

		namespace = "some-namespace"

		worker  *extensionsv1alpha1.Worker
		machine *machinev1alpha1.Machine
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		worker = &extensionsv1alpha1.Worker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker",
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.WorkerSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: "local",
				},
			},
		}

		machine = &machinev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine",
				Namespace: namespace,
				Labels: map[string]string{
					v1beta1constants.LabelWorkerName: worker.Name,
				},
			},
		}
	})

	Describe("#MachineToWorkerMapper", func() {
		var mapper handler.MapFunc

		BeforeEach(func() {
			mapper = MachineToWorkerMapper(fakeClient, nil)

			Expect(fakeClient.Create(ctx, worker)).To(Succeed())
		})

		It("should return nil when the object is not a Machine", func() {
			Expect(mapper(ctx, &corev1.Secret{})).To(BeNil())
		})

		It("should return nil when the machine does not have a worker label", func() {
			delete(machine.Labels, v1beta1constants.LabelWorkerName)

			Expect(mapper(ctx, machine)).To(BeNil())
		})

		It("should return nil when the worker cannot be found", func() {
			Expect(fakeClient.Delete(ctx, worker)).To(Succeed())

			Expect(mapper(ctx, machine)).To(BeNil())
		})

		It("should return nil when the predicates do not match", func() {
			mapper = MachineToWorkerMapper(fakeClient, predicateutils.AddTypeAndClassPredicates(nil, nil, "local2"))

			Expect(mapper(ctx, machine)).To(BeNil())
		})

		It("should map the machine to the worker", func() {
			mapper = MachineToWorkerMapper(fakeClient, predicateutils.AddTypeAndClassPredicates(nil, nil, "local"))

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
