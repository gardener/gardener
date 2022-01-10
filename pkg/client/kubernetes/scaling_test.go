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

package kubernetes_test

import (
	"context"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("scale", func() {
	var (
		ctx context.Context
		c   client.Client
		key client.ObjectKey
	)

	BeforeEach(func() {
		ctx = context.TODO()
		key = client.ObjectKey{Name: "foo", Namespace: "bar"}
		om := metav1.ObjectMeta{Name: "foo", Namespace: "bar"}

		s := runtime.NewScheme()
		Expect(appsv1.AddToScheme(s)).NotTo(HaveOccurred(), "adding apps to schema succeeds")
		Expect(druidv1alpha1.AddToScheme(s)).NotTo(HaveOccurred(), "adding druid to schema succeeds")

		c = fake.NewClientBuilder().WithScheme(s).WithObjects(
			&appsv1.StatefulSet{ObjectMeta: om},
			&appsv1.Deployment{ObjectMeta: om},
			&druidv1alpha1.Etcd{ObjectMeta: om},
		).Build()
	})

	Context("ScaleStatefulSet", func() {
		It("sets scale to 2", func() {
			Expect(ScaleStatefulSet(ctx, c, key, 2)).NotTo(HaveOccurred(), "scale succeeds")

			updated := &appsv1.StatefulSet{}
			Expect(c.Get(ctx, key, updated)).NotTo(HaveOccurred(), "could get the updated resource")

			Expect(updated.Spec.Replicas).To(PointTo(BeEquivalentTo(2)), "updated replica")
		})
	})

	Context("ScaleDeployment", func() {
		It("sets scale to 2", func() {
			Expect(ScaleDeployment(ctx, c, key, 2)).NotTo(HaveOccurred(), "scale succeeds")

			updated := &appsv1.Deployment{}
			Expect(c.Get(ctx, key, updated)).NotTo(HaveOccurred(), "could get the updated resource")

			Expect(updated.Spec.Replicas).To(PointTo(BeEquivalentTo(2)), "updated replica")
		})
	})

	Context("ScaleEtcd", func() {
		It("sets scale to 2", func() {
			Expect(ScaleEtcd(ctx, c, key, 2)).NotTo(HaveOccurred(), "scale succeeds")

			updated := &druidv1alpha1.Etcd{}
			Expect(c.Get(ctx, key, updated)).NotTo(HaveOccurred(), "could get the updated resource")

			Expect(updated.Spec.Replicas).To(BeEquivalentTo(2), "updated replica")
		})
	})
})
