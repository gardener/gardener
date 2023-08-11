// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seedfinalizer_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/seedfinalizer"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Reconciler", func() {
	var (
		ctrl *gomock.Controller
		c    client.Client

		reconciler *Reconciler

		ctx      = context.TODO()
		seedName = "seed"
		seed     *gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithIndex(&gardencorev1beta1.ControllerInstallation{}, core.SeedRefName, indexer.ControllerInstallationSeedRefNameIndexerFunc).
			Build()

		reconciler = &Reconciler{Client: c}
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: seedName,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		It("should return nil because object not found", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("deletion timestamp not set", func() {
			BeforeEach(func() {
				Expect(c.Create(ctx, seed)).To(Succeed())
			})

			It("should ensure the finalizer", func() {
				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				Expect(seed.Finalizers).To(ConsistOf("core.gardener.cloud/controllerregistration"))
			})
		})

		Context("deletion timestamp set", func() {
			BeforeEach(func() {
				Expect(c.Create(ctx, seed)).To(Succeed())
				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				Expect(seed.Finalizers).To(ConsistOf("core.gardener.cloud/controllerregistration"))

				Expect(c.Delete(ctx, seed)).To(Succeed())
			})

			It("should return an error because installation referencing seed exists", func() {
				controllerInstallation := &gardencorev1beta1.ControllerInstallation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "controllerInstallation",
					},
					Spec: gardencorev1beta1.ControllerInstallationSpec{
						SeedRef: corev1.ObjectReference{
							Name: seedName,
						},
					},
				}

				Expect(c.Create(ctx, controllerInstallation)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).To(MatchError(ContainSubstring("cannot remove finalizer of Seed %q because still found ControllerInstallations: [%s]", seed.Name, controllerInstallation.Name)))
			})

			It("should remove the finalizer", func() {
				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(BeNotFoundError())
			})
		})
	})
})
