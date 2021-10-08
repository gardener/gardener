// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package mapper_test

import (
	"context"
	"fmt"

	extensionshandler "github.com/gardener/gardener/extensions/pkg/handler"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/resourcemanager/mapper"
	filter2 "github.com/gardener/gardener/pkg/resourcemanager/predicate"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

var _ = Describe("#SecretToManagedResourceMapper", func() {
	var (
		c      *mockclient.MockClient
		ctrl   *gomock.Controller
		m      extensionshandler.Mapper
		secret *corev1.Secret
		filter *filter2.ClassFilter
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mr-secret",
				Namespace: "mr-namespace",
			},
		}

		filter = filter2.NewClassFilter("seed")

		m = mapper.SecretToManagedResourceMapper(filter)

		Expect(inject.ClientInto(c, m)).To(BeTrue())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should be able to inject stop channel", func() {
		Expect(inject.StopChannelInto(context.TODO().Done(), m)).To(BeTrue())
	})

	It("should do nothing, if Object is nil", func() {
		requests := m.Map(nil)
		Expect(requests).To(BeEmpty())
	})

	It("should do nothing, if Object is not a Secret", func() {
		requests := m.Map(&corev1.Pod{})
		Expect(requests).To(BeEmpty())
	})

	It("should do nothing, if list fails", func() {
		c.EXPECT().List(nil, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace)).
			Return(fmt.Errorf("fake"))

		requests := m.Map(secret)
		Expect(requests).To(BeEmpty())
	})

	It("should do nothing, if there are no ManagedResources", func() {
		c.EXPECT().List(nil, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace))

		requests := m.Map(secret)
		Expect(requests).To(BeEmpty())
	})

	It("should do nothing, if there are no ManagedResources we are responsible for", func() {
		mr := resourcesv1alpha1.ManagedResource{
			Spec: resourcesv1alpha1.ManagedResourceSpec{Class: pointer.StringPtr("other")},
		}

		c.EXPECT().List(nil, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace)).
			DoAndReturn(func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
				list.(*resourcesv1alpha1.ManagedResourceList).Items = []resourcesv1alpha1.ManagedResource{mr}
				return nil
			})

		requests := m.Map(secret)
		Expect(requests).To(BeEmpty())
	})

	It("should correctly map to ManagedResources that reference the secret", func() {
		mr := resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mr",
				Namespace: secret.Namespace,
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class:      pointer.StringPtr(filter.ResourceClass()),
				SecretRefs: []corev1.LocalObjectReference{{Name: secret.Name}},
			},
		}

		c.EXPECT().List(nil, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace)).
			DoAndReturn(func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
				list.(*resourcesv1alpha1.ManagedResourceList).Items = []resourcesv1alpha1.ManagedResource{mr}
				return nil
			})

		requests := m.Map(secret)
		Expect(requests).To(ConsistOf(
			reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      mr.Name,
				Namespace: mr.Namespace,
			}},
		))
	})
})
