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

package managedseedset

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	name      = "test"
	namespace = "garden"
)

var _ = Describe("Add", func() {
	var reconciler *Reconciler

	BeforeEach(func() {
		reconciler = &Reconciler{}
	})

	Describe("#MapSeedToManagedSeedSet", func() {
		var (
			ctx        = context.TODO()
			log        logr.Logger
			fakeClient client.Client

			seed        *gardencorev1beta1.Seed
			managedSeed *seedmanagementv1alpha1.ManagedSeed
			set         *seedmanagementv1alpha1.ManagedSeedSet
		)

		BeforeEach(func() {
			log = logr.Discard()
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

			seed = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: name + "0",
				},
			}

			managedSeed = &seedmanagementv1alpha1.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "0",
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "ManagedSeedSet",
							Name: name,
						},
					},
				},
				Spec: seedmanagementv1alpha1.ManagedSeedSpec{},
			}

			set = &seedmanagementv1alpha1.ManagedSeedSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: seedmanagementv1alpha1.ManagedSeedSetSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"name": name,
						},
					},
				},
			}
		})

		It("should do nothing if the object is no Seed", func() {
			Expect(reconciler.MapSeedToManagedSeedSet(ctx, log, fakeClient, &corev1.Secret{})).To(BeEmpty())
		})

		It("should do nothing if there is no related ManagedSeed", func() {
			Expect(reconciler.MapSeedToManagedSeedSet(ctx, log, fakeClient, seed)).To(BeEmpty())
		})

		It("should do nothing if the ManagedSeed does not reference any ManagedSeedSet", func() {
			managedSeed.OwnerReferences = nil
			Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
			Expect(reconciler.MapSeedToManagedSeedSet(ctx, log, fakeClient, seed)).To(BeEmpty())
		})

		It("should do nothing if the referenced ManagedSeedSet doesnot exist", func() {
			set.Name = "foo"
			Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
			Expect(reconciler.MapSeedToManagedSeedSet(ctx, log, fakeClient, seed)).To(BeEmpty())
		})

		It("should map the Seed to the ManagedSeedSet", func() {
			Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())
			Expect(fakeClient.Create(ctx, set)).To(Succeed())

			Expect(reconciler.MapSeedToManagedSeedSet(ctx, log, fakeClient, seed)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: set.Name, Namespace: set.Namespace}},
			))
		})
	})
})
