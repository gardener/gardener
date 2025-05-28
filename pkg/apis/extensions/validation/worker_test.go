// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/validation"
)

var _ = Describe("Worker validation tests", func() {
	var worker *extensionsv1alpha1.Worker

	BeforeEach(func() {
		worker = &extensionsv1alpha1.Worker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-worker",
				Namespace: "test-namespace",
			},
			Spec: extensionsv1alpha1.WorkerSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: "provider",
				},
				Region: "region",
				SecretRef: corev1.SecretReference{
					Name: "test",
				},
				InfrastructureProviderStatus: &runtime.RawExtension{},
				SSHPublicKey:                 []byte("key"),
				Pools: []extensionsv1alpha1.WorkerPool{
					{
						MachineType: "large",
						MachineImage: extensionsv1alpha1.MachineImage{
							Name:    "image1",
							Version: "version1",
						},
						Name:         "pool1",
						Architecture: ptr.To("amd64"),
					},
				},
			},
		}
	})

	Describe("#ValidWorker", func() {
		It("should forbid empty Worker resources", func() {
			errorList := ValidateWorker(&extensionsv1alpha1.Worker{})

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.namespace"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.region"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.secretRef.name"),
			}))))
		})

		It("should forbid Worker resources with invalid pools", func() {
			workerCopy := worker.DeepCopy()

			workerCopy.Spec.Pools[0] = extensionsv1alpha1.WorkerPool{
				Architecture: ptr.To("test"),
			}

			workerCopy.Spec.Pools[0].NodeTemplate = &extensionsv1alpha1.NodeTemplate{
				Capacity: corev1.ResourceList{
					"cpu":    resource.MustParse("-1"),
					"gpu":    resource.MustParse("1"),
					"memory": resource.MustParse("8Gi"),
				},
			}

			errorList := ValidateWorker(workerCopy)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.pools[0].machineType"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.pools[0].machineImage.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.pools[0].machineImage.version"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.pools[0].name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.pools[0].nodeTemplate.capacity.cpu"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.pools[0].architecture"),
			}))))
		})

		It("should allow valid worker resources", func() {
			errorList := ValidateWorker(worker)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidWorkerUpdate", func() {
		It("should prevent updating anything if deletion time stamp is set", func() {
			now := metav1.Now()
			worker.DeletionTimestamp = &now

			newWorker := prepareWorkerForUpdate(worker)
			newWorker.DeletionTimestamp = &now
			newWorker.Spec.SecretRef.Name = "changed-secretref-name"

			errorList := ValidateWorkerUpdate(newWorker, worker)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec"),
				"Detail": Equal("cannot update worker spec if deletion timestamp is set. Requested changes: SecretRef.Name: changed-secretref-name != test"),
			}))))
		})

		It("should prevent updating the type and region", func() {
			newWorker := prepareWorkerForUpdate(worker)
			newWorker.Spec.Type = "changed-type"
			newWorker.Spec.Region = "changed-region"

			errorList := ValidateWorkerUpdate(newWorker, worker)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.region"),
			}))))
		})

		It("should prevent updating architecture to invalid architecture", func() {
			newWorker := prepareWorkerForUpdate(worker)
			newWorker.Spec.Pools[0].Architecture = ptr.To("foo")

			errorList := ValidateWorkerUpdate(newWorker, worker)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.pools[0].architecture"),
			}))))
		})

		It("should allow updating architecture to valid architecture", func() {
			newWorker := prepareWorkerForUpdate(worker)
			newWorker.Spec.Pools[0].Architecture = ptr.To("arm64")

			errorList := ValidateWorkerUpdate(newWorker, worker)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow updating the name of the referenced secret, the infrastructure provider status, the ssh public key, or the worker pools", func() {
			newWorker := prepareWorkerForUpdate(worker)
			newWorker.Spec.SecretRef.Name = "changed-secretref-name"
			newWorker.Spec.InfrastructureProviderStatus = nil
			newWorker.Spec.SSHPublicKey = []byte("other-key")
			newWorker.Spec.Pools = []extensionsv1alpha1.WorkerPool{
				{
					MachineType: "ultra-large",
					MachineImage: extensionsv1alpha1.MachineImage{
						Name:    "image2",
						Version: "version2",
					},
					Name:              "pool2",
					Architecture:      ptr.To("amd64"),
					UserDataSecretRef: corev1.SecretKeySelector{Key: "new-bootstrap-data-key"},
				},
			}

			errorList := ValidateWorkerUpdate(newWorker, worker)

			Expect(errorList).To(BeEmpty())
		})
	})
})

func prepareWorkerForUpdate(obj *extensionsv1alpha1.Worker) *extensionsv1alpha1.Worker {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
