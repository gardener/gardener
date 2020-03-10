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

package project

import (
	gardenercore "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("#rolebindingDelete", func() {
	const (
		ns = "test"
	)
	var (
		c           *Controller
		indexer     cache.Indexer
		queue       workqueue.RateLimitingInterface
		proj        *gardenercore.Project
		rolebinding *rbacv1.RoleBinding
	)

	BeforeEach(func() {
		// This should not be here!!! Hidden dependency!!!
		logger.Logger = logger.NewNopLogger()

		indexer = cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
		proj = &gardenercore.Project{
			ObjectMeta: metav1.ObjectMeta{Name: "project-1"},
			Spec: gardenercore.ProjectSpec{
				Namespace: pointer.StringPtr(ns),
			},
		}
		rolebinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "role-1", Namespace: ns},
		}
		c = &Controller{
			projectLister: gardencorelisters.NewProjectLister(indexer),
			projectQueue:  queue,
		}
	})

	It("should not requeue random rolebinding", func() {
		Expect(indexer.Add(proj)).ToNot(HaveOccurred())

		c.rolebindingDelete(rolebinding)

		Expect(queue.Len()).To(Equal(0), "no items in the queue")
	})

	DescribeTable("requeue when rolebinding is",
		func(rolebindingName string) {
			rolebinding.Name = rolebindingName
			Expect(indexer.Add(proj)).ToNot(HaveOccurred())

			c.rolebindingDelete(rolebinding)

			Expect(queue.Len()).To(Equal(1), "only one item in queue")
			actual, _ := queue.Get()
			Expect(actual).To(Equal("project-1"))
		},

		Entry("project-member", "gardener.cloud:system:project-member"),
		Entry("project-viewer", "gardener.cloud:system:project-viewer"),
		Entry("custom role", "gardener.cloud:extension:project:project-1:foo"),
	)

	DescribeTable("no requeue when project is being deleted and rolebinding is",
		func(rolebindingName string) {
			now := metav1.Now()
			proj.DeletionTimestamp = &now
			rolebinding.Name = rolebindingName
			Expect(indexer.Add(proj)).ToNot(HaveOccurred())

			c.rolebindingDelete(rolebinding)

			Expect(queue.Len()).To(Equal(0), "no projects in queue")
		},

		Entry("project-member", "gardener.cloud:system:project-member"),
		Entry("project-viewer", "gardener.cloud:system:project-viewer"),
		Entry("custom role", "gardener.cloud:extension:project:project-1:foo"),
	)

	It("should requeue multiple projects with the same namespace", func() {
		rolebinding.Name = "gardener.cloud:system:project-member"
		proj2 := proj.DeepCopy()
		proj2.Name = "project-2"
		Expect(indexer.Add(proj)).ToNot(HaveOccurred())
		Expect(indexer.Add(proj2)).ToNot(HaveOccurred())

		c.rolebindingDelete(rolebinding)

		Expect(queue.Len()).To(Equal(2), "two items in queue")
		actual, _ := queue.Get()
		Expect(actual).To(Equal("project-1"))
		actual, _ = queue.Get()
		Expect(actual).To(Equal("project-2"))
	})
})
