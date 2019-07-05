// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extensions

import (
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	extensionsinstall "github.com/gardener/gardener/pkg/apis/extensions/install"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	scheme *runtime.Scheme
)

func init() {
	scheme = runtime.NewScheme()
	extensionsinstall.Install(scheme)
}

func mkUnstructuredAccessor(obj extensionsv1alpha1.Object) extensionsv1alpha1.Object {
	u := &unstructured.Unstructured{}
	Expect(scheme.Convert(obj, u, nil)).To(Succeed())
	return UnstructuredAccessor(u)
}

func mkUnstructuredAccessorWithSpec(spec extensionsv1alpha1.DefaultSpec) extensionsv1alpha1.Spec {
	return mkUnstructuredAccessor(&extensionsv1alpha1.Infrastructure{Spec: extensionsv1alpha1.InfrastructureSpec{DefaultSpec: spec}}).GetExtensionSpec()
}

func mkUnstructuredAccessorWithStatus(status extensionsv1alpha1.DefaultStatus) extensionsv1alpha1.Status {
	return mkUnstructuredAccessor(&extensionsv1alpha1.Infrastructure{Status: extensionsv1alpha1.InfrastructureStatus{DefaultStatus: status}}).GetExtensionStatus()
}

func mkUnstructuredAccessorWithLastOperation(lastOperation *gardencorev1alpha1.LastOperation) extensionsv1alpha1.LastOperation {
	return mkUnstructuredAccessorWithStatus(extensionsv1alpha1.DefaultStatus{LastOperation: lastOperation}).GetLastOperation()
}

var _ = Describe("Accessor", func() {
	Describe("#Accessor", func() {
		It("should create an accessor for extensions", func() {
			extension := &extensionsv1alpha1.Infrastructure{}
			acc, err := Accessor(extension)

			Expect(err).NotTo(HaveOccurred())
			Expect(acc).To(BeIdenticalTo(extension))
		})

		It("should create an unstructured accessor for unstructures", func() {
			u := &unstructured.Unstructured{}
			acc, err := Accessor(u)

			Expect(err).NotTo(HaveOccurred())
			Expect(acc).To(Equal(UnstructuredAccessor(u)))
		})

		It("should error for other objects", func() {
			_, err := Accessor(&corev1.ConfigMap{})

			Expect(err).To(HaveOccurred())
		})
	})

	Context("#UnstructuredAccessor", func() {
		Context("#GetExtensionSpec", func() {
			Describe("#GetExtensionType", func() {
				It("should get the extension type", func() {
					var (
						t   = "foo"
						acc = mkUnstructuredAccessorWithSpec(extensionsv1alpha1.DefaultSpec{Type: t})
					)

					Expect(acc.GetExtensionType()).To(Equal(t))
				})
			})
		})

		Context("#GetExtensionStatus", func() {
			Context("#GetLastOperation", func() {
				Describe("#GetDescription", func() {
					It("should get the description", func() {
						var (
							desc = "desc"
							acc  = mkUnstructuredAccessorWithLastOperation(&gardencorev1alpha1.LastOperation{Description: desc})
						)

						Expect(acc.GetDescription()).To(Equal(desc))
					})
				})

				Describe("#GetLastUpdateTime", func() {
					It("should get the last update time", func() {
						var (
							t   = metav1.NewTime(time.Unix(50, 0))
							acc = mkUnstructuredAccessorWithLastOperation(&gardencorev1alpha1.LastOperation{LastUpdateTime: t})
						)

						Expect(acc.GetLastUpdateTime()).To(Equal(t))
					})
				})

				Describe("#GetProgress", func() {
					It("should get the progress", func() {
						var (
							progress = 10
							acc      = mkUnstructuredAccessorWithLastOperation(&gardencorev1alpha1.LastOperation{Progress: progress})
						)

						Expect(acc.GetProgress()).To(Equal(progress))
					})
				})

				Describe("#GetState", func() {
					It("should get the state", func() {
						var (
							state = gardencorev1alpha1.LastOperationStateSucceeded
							acc   = mkUnstructuredAccessorWithLastOperation(&gardencorev1alpha1.LastOperation{State: state})
						)

						Expect(acc.GetState()).To(Equal(state))
					})
				})

				Describe("#GetType", func() {
					It("should get the type", func() {
						var (
							t   = gardencorev1alpha1.LastOperationTypeReconcile
							acc = mkUnstructuredAccessorWithLastOperation(&gardencorev1alpha1.LastOperation{Type: t})
						)

						Expect(acc.GetType()).To(Equal(t))
					})
				})
			})
		})
	})
})
