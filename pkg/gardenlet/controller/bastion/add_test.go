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

package bastion

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
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
		project           *gardencorev1beta1.Project

		shootTechnicalID = "shoot--" + projectName + "--shootName"
	)

	BeforeEach(func() {
		reconciler = &Reconciler{}
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{Name: projectName},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: pointer.String("test-" + projectName),
			},
		}
	})

	Describe("#MapExtensionsBastionToOperationsBastion", func() {
		BeforeEach(func() {
			log = logr.Discard()

			operationsBastion = &operationsv1alpha1.Bastion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bastionName,
					Namespace: *project.Spec.Namespace,
				},
			}

			extensionsBastion = &extensionsv1alpha1.Bastion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bastionName,
					Namespace: shootTechnicalID,
				},
			}
		})

		It("should do nothing if object is not extensions Bastion", func() {
			Expect(reconciler.MapExtensionsBastionToOperationsBastion(ctx, log, fakeClient, &operationsv1alpha1.Bastion{})).To(BeEmpty())
		})

		It("should do nothing if operations Bastion does not exist", func() {
			Expect(reconciler.MapExtensionsBastionToOperationsBastion(ctx, log, fakeClient, extensionsBastion)).To(BeEmpty())
		})

		It("should do nothing if project does not exist", func() {
			Expect(fakeClient.Create(ctx, operationsBastion)).To(Succeed())
			Expect(reconciler.MapExtensionsBastionToOperationsBastion(ctx, log, fakeClient, extensionsBastion)).To(BeEmpty())
		})

		It("should map the extensions Bastion to operations Bastion", func() {
			Expect(fakeClient.Create(ctx, project)).To(Succeed())
			Expect(fakeClient.Create(ctx, operationsBastion)).To(Succeed())
			Expect(reconciler.MapExtensionsBastionToOperationsBastion(ctx, log, fakeClient, extensionsBastion)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: *project.Spec.Namespace, Name: operationsBastion.Name}},
			))
		})
	})

	Describe("#GetProjectNameFromTechincalId", func() {
		It("should return empty string if project does not exist", func() {
			projectName, err := GetProjectNameFromTechincalId(ctx, fakeClient, shootTechnicalID)
			Expect(projectName).To(Equal(""))
			Expect(err).NotTo(Succeed())
		})

		It("should return project name if project exist", func() {
			Expect(fakeClient.Create(ctx, project)).To(Succeed())
			projectName, err := GetProjectNameFromTechincalId(ctx, fakeClient, shootTechnicalID)
			Expect(projectName).To(Equal(*project.Spec.Namespace))
			Expect(err).To(BeNil())
		})
	})
})
