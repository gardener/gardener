// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package bastion_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/bastion"
)

const (
	bastionName = "foo"
	projectName = "project"
)

var _ = Describe("Add", func() {
	var (
		ctx        = context.TODO()
		log        logr.Logger
		fakeClient client.Client
		reconciler *Reconciler

		operationsBastion *operationsv1alpha1.Bastion
		extensionsBastion *extensionsv1alpha1.Bastion
		cluster           *extensionsv1alpha1.Cluster

		shootTechnicalID = "shoot--" + projectName + "--shootName"
		projectNamespace = "garden" + projectName
	)

	BeforeEach(func() {
		testScheme := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(testScheme)).To(Succeed())
		fakeClient = fakeclient.NewClientBuilder().WithScheme(testScheme).Build()

		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootTechnicalID,
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				Shoot: runtime.RawExtension{
					Object: &gardencorev1beta1.Shoot{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shoot",
							Namespace: projectNamespace,
						},
					},
				},
			},
		}

		reconciler = &Reconciler{
			SeedClient: fakeClient,
		}
	})

	Describe("#MapExtensionsBastionToOperationsBastion", func() {
		BeforeEach(func() {
			log = logr.Discard()

			operationsBastion = &operationsv1alpha1.Bastion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bastionName,
					Namespace: projectNamespace,
				},
			}

			extensionsBastion = &extensionsv1alpha1.Bastion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bastionName,
					Namespace: shootTechnicalID,
				},
			}
		})

		It("should map the extensions Bastion to operations Bastion", func() {
			Expect(fakeClient.Create(ctx, cluster)).To(Succeed())
			Expect(reconciler.MapExtensionsBastionToOperationsBastion(ctx, log, fakeClient, extensionsBastion)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: operationsBastion.Namespace, Name: operationsBastion.Name}},
			))
		})

		It("should return nil if the cluster is not found", func() {
			Expect(reconciler.MapExtensionsBastionToOperationsBastion(ctx, log, fakeClient, extensionsBastion)).To(BeNil())
		})

		It("should return nil when shoot is not present in the cluster", func() {
			cluster.Spec.Shoot.Object = nil
			Expect(fakeClient.Create(ctx, cluster)).To(Succeed())

			Expect(reconciler.MapExtensionsBastionToOperationsBastion(ctx, log, nil, extensionsBastion)).To(BeNil())
		})
	})
})
