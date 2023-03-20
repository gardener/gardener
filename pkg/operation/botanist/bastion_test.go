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

package botanist_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
)

var _ = Describe("Bastions", func() {
	var (
		fakeClient         client.Client
		botanist           *Botanist
		namespace          *corev1.Namespace
		ctx                = context.TODO()
		bastion1, bastion2 *extensionsv1alpha1.Bastion
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		botanist = &Botanist{Operation: &operation.Operation{}}
		k8sSeedClient := kubernetesfake.NewClientSetBuilder().WithClient(fakeClient).Build()
		namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		botanist.SeedClientSet = k8sSeedClient
		botanist.Shoot = &shootpkg.Shoot{
			SeedNamespace: namespace.Name,
		}

		bastion1 = &extensionsv1alpha1.Bastion{
			ObjectMeta: metav1.ObjectMeta{Name: "bastion1", Namespace: namespace.Name},
		}
		bastion2 = &extensionsv1alpha1.Bastion{
			ObjectMeta: metav1.ObjectMeta{Name: "bastion2", Namespace: namespace.Name},
		}
	})

	Describe("#DeleteBastions", func() {
		It("should delete all bastions", func() {
			Expect(fakeClient.Create(ctx, bastion1)).To(Succeed())
			Expect(fakeClient.Create(ctx, bastion2)).To(Succeed())

			Expect(botanist.DeleteBastions(ctx)).To(Succeed())

			bastionList := &metav1.PartialObjectMetadataList{}
			bastionList.SetGroupVersionKind(extensionsv1alpha1.SchemeGroupVersion.WithKind("BastionList"))
			Expect(fakeClient.List(ctx, bastionList, client.InNamespace(namespace.Name))).To(Succeed())
			Expect(len(bastionList.Items)).To(Equal(0))
		})
	})
})
