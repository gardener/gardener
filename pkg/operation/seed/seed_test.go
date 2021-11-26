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

package seed_test

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/operation/seed"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("seed", func() {
	var (
		ctrl          *gomock.Controller
		runtimeClient *mockclient.MockClient
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		runtimeClient = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetWildcardCertificate", func() {
		It("should return no wildcard certificate secret", func() {
			runtimeClient.EXPECT().List(context.TODO(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlaneWildcardCert})

			secret, err := GetWildcardCertificate(context.TODO(), runtimeClient)

			Expect(err).ToNot(HaveOccurred())
			Expect(secret).To(BeNil())
		})

		It("should return a wildcard certificate secret", func() {
			secretList := &corev1.SecretList{
				Items: []corev1.Secret{
					{},
				},
			}
			runtimeClient.EXPECT().List(context.TODO(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlaneWildcardCert}).DoAndReturn(
				func(_ context.Context, secrets *corev1.SecretList, _ ...client.ListOption) error {
					*secrets = *secretList
					return nil
				})

			secret, err := GetWildcardCertificate(context.TODO(), runtimeClient)

			Expect(err).ToNot(HaveOccurred())
			Expect(*secret).To(Equal(secretList.Items[0]))
		})

		It("should return an error because more than one wildcard secrets is found", func() {
			secretList := &corev1.SecretList{
				Items: []corev1.Secret{
					{},
					{},
				},
			}
			runtimeClient.EXPECT().List(context.TODO(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlaneWildcardCert}).DoAndReturn(
				func(_ context.Context, secrets *corev1.SecretList, _ ...client.ListOption) error {
					*secrets = *secretList
					return nil
				})

			secret, err := GetWildcardCertificate(context.TODO(), runtimeClient)

			Expect(err).To(HaveOccurred())
			Expect(secret).To(BeNil())
		})
	})

	Describe("#GetValidVolumeSize", func() {
		It("should return the size because no minimum size was set", func() {
			var (
				size = "20Gi"
				seed = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Volume: nil,
				},
			})

			Expect(seed.GetValidVolumeSize(size)).To(Equal(size))
		})

		It("should return the minimum size because the given value is smaller", func() {
			var (
				size                = "20Gi"
				minimumSize         = "25Gi"
				minimumSizeQuantity = resource.MustParse(minimumSize)
				seed                = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Volume: &gardencorev1beta1.SeedVolume{
						MinimumSize: &minimumSizeQuantity,
					},
				},
			})

			Expect(seed.GetValidVolumeSize(size)).To(Equal(minimumSize))
		})

		It("should return the given value size because the minimum size is smaller", func() {
			var (
				size                = "30Gi"
				minimumSize         = "25Gi"
				minimumSizeQuantity = resource.MustParse(minimumSize)
				seed                = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Volume: &gardencorev1beta1.SeedVolume{
						MinimumSize: &minimumSizeQuantity,
					},
				},
			})

			Expect(seed.GetValidVolumeSize(size)).To(Equal(size))
		})
	})

	Describe("#ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame", func() {
		const (
			lokiPVCName         = "loki-loki-0"
			lokiStatefulSetName = "loki"
			gardenNamespace     = "garden"
		)
		var (
			ctx               = context.TODO()
			log               = logger.NewNopLogger()
			lokiPVCObjectMeta = metav1.ObjectMeta{
				Name:      lokiPVCName,
				Namespace: gardenNamespace,
			}
			lokiPVC = &corev1.PersistentVolumeClaim{
				ObjectMeta: lokiPVCObjectMeta,
				Spec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							"storage": resource.MustParse("100Gi"),
						},
					},
				},
			}
			patch       = client.MergeFrom(lokiPVC.DeepCopy())
			statefulset = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      lokiStatefulSetName,
					Namespace: gardenNamespace,
				},
			}
			scaledToZeroLokiStatefulset = appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:       lokiStatefulSetName,
					Namespace:  gardenNamespace,
					Generation: 2,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointer.Int32Ptr(0),
				},
				Status: appsv1.StatefulSetStatus{
					ObservedGeneration: 2,
					Replicas:           0,
					AvailableReplicas:  0,
				},
			}
			zeroReplicaRawPatch     = client.RawPatch(types.MergePatchType, []byte(`{"spec":{"replicas":0}}`))
			errNotFound             = &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
			errForbidden            = &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonForbidden}}
			new200GiStorageQuantity = resource.MustParse("200Gi")
			new100GiStorageQuantity = resource.MustParse("100Gi")
			new80GiStorageQuantity  = resource.MustParse("80Gi")
			lokiPVCKey              = kutil.Key("garden", "loki-loki-0")
			lokiStatefulSetKey      = kutil.Key("garden", "loki")
			funcGetLokiPVC          = func(_ context.Context, _ types.NamespacedName, pvc *corev1.PersistentVolumeClaim) error {
				*pvc = *lokiPVC
				return nil
			}
			funcGetScaledToZeroLokiStatefulset = func(_ context.Context, _ types.NamespacedName, sts *appsv1.StatefulSet) error {
				*sts = scaledToZeroLokiStatefulset
				return nil
			}
			funcPatchTo200GiStorage = func(_ context.Context, pvc *corev1.PersistentVolumeClaim, _ client.Patch, _ ...interface{}) error {
				if pvc.Spec.Resources.Requests.Storage().Cmp(resource.MustParse("200Gi")) != 0 {
					return fmt.Errorf("expect 200Gi found %v", *pvc.Spec.Resources.Requests.Storage())
				}
				return nil
			}
			objectOfTypePVC = gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{})
			objectOfTypeSTS = gomock.AssignableToTypeOf(&appsv1.StatefulSet{})
		)

		It("should patch garden/loki's PVC when new size is greater than the current one", func() {
			runtimeClient.EXPECT().Get(ctx, lokiPVCKey, objectOfTypePVC).DoAndReturn(funcGetLokiPVC)
			runtimeClient.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch)
			runtimeClient.EXPECT().Get(gomock.Any(), lokiStatefulSetKey, objectOfTypeSTS).DoAndReturn(funcGetScaledToZeroLokiStatefulset)
			runtimeClient.EXPECT().Patch(ctx, objectOfTypePVC, gomock.AssignableToTypeOf(patch)).DoAndReturn(funcPatchTo200GiStorage)
			runtimeClient.EXPECT().Delete(ctx, statefulset)
			Expect(ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, runtimeClient, new200GiStorageQuantity, log)).To(Succeed())
		})

		It("should delete garden/loki's PVC when new size is less than the current one", func() {
			runtimeClient.EXPECT().Get(ctx, lokiPVCKey, objectOfTypePVC).DoAndReturn(funcGetLokiPVC)
			runtimeClient.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch)
			runtimeClient.EXPECT().Get(gomock.Any(), lokiStatefulSetKey, objectOfTypeSTS).DoAndReturn(funcGetScaledToZeroLokiStatefulset)
			runtimeClient.EXPECT().Delete(ctx, lokiPVC)
			runtimeClient.EXPECT().Delete(ctx, statefulset)
			Expect(ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, runtimeClient, new80GiStorageQuantity, log)).To(Succeed())
		})

		It("shouldn't do anything when garden/loki's PVC is missing", func() {
			runtimeClient.EXPECT().Get(ctx, lokiPVCKey, objectOfTypePVC).Return(errNotFound)
			Expect(ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, runtimeClient, new80GiStorageQuantity, log)).To(Succeed())
		})

		It("shouldn't do anything when garden/loki's PVC storage is the same as the new one", func() {
			runtimeClient.EXPECT().Get(ctx, lokiPVCKey, objectOfTypePVC).DoAndReturn(funcGetLokiPVC)
			Expect(ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, runtimeClient, new100GiStorageQuantity, log)).To(Succeed())
		})

		It("should proceed with the garden/loki's PVC resizing when Loki StatefulSet is missing", func() {
			runtimeClient.EXPECT().Get(ctx, lokiPVCKey, objectOfTypePVC).DoAndReturn(funcGetLokiPVC)
			runtimeClient.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch).Return(errNotFound)
			runtimeClient.EXPECT().Patch(ctx, objectOfTypePVC, gomock.AssignableToTypeOf(patch)).DoAndReturn(funcPatchTo200GiStorage)
			runtimeClient.EXPECT().Delete(ctx, statefulset).Return(errNotFound)
			Expect(ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, runtimeClient, new200GiStorageQuantity, log)).To(Succeed())
		})

		It("should succeed with the garden/loki's PVC resizing when Loki StatefulSet was deleted during function execution", func() {
			runtimeClient.EXPECT().Get(ctx, lokiPVCKey, objectOfTypePVC).DoAndReturn(funcGetLokiPVC)
			runtimeClient.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch)
			runtimeClient.EXPECT().Get(gomock.Any(), lokiStatefulSetKey, objectOfTypeSTS).DoAndReturn(funcGetScaledToZeroLokiStatefulset)
			runtimeClient.EXPECT().Patch(ctx, objectOfTypePVC, gomock.AssignableToTypeOf(patch)).DoAndReturn(funcPatchTo200GiStorage)
			runtimeClient.EXPECT().Delete(ctx, statefulset).Return(errNotFound)
			Expect(ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, runtimeClient, new200GiStorageQuantity, log)).To(Succeed())
		})

		It("should not fail with patching garden/loki's PVC when the PVC itself was deleted during function execution", func() {
			runtimeClient.EXPECT().Get(ctx, lokiPVCKey, objectOfTypePVC).DoAndReturn(funcGetLokiPVC)
			runtimeClient.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch)
			runtimeClient.EXPECT().Get(gomock.Any(), lokiStatefulSetKey, objectOfTypeSTS).DoAndReturn(funcGetScaledToZeroLokiStatefulset)
			runtimeClient.EXPECT().Patch(ctx, objectOfTypePVC, gomock.AssignableToTypeOf(patch)).Return(errNotFound)
			runtimeClient.EXPECT().Delete(ctx, statefulset)
			Expect(ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, runtimeClient, new200GiStorageQuantity, log)).To(Succeed())
		})

		It("should not fail with deleting garden/loki's PVC when the PVC itself was deleted during function execution", func() {
			runtimeClient.EXPECT().Get(ctx, lokiPVCKey, objectOfTypePVC).DoAndReturn(funcGetLokiPVC)
			runtimeClient.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch)
			runtimeClient.EXPECT().Get(gomock.Any(), lokiStatefulSetKey, objectOfTypeSTS).DoAndReturn(funcGetScaledToZeroLokiStatefulset)
			runtimeClient.EXPECT().Delete(ctx, lokiPVC).Return(errNotFound)
			runtimeClient.EXPECT().Delete(ctx, statefulset)
			Expect(ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, runtimeClient, new80GiStorageQuantity, log)).To(Succeed())
		})

		It("should not neglect errors when getting garden/loki's PVC", func() {
			runtimeClient.EXPECT().Get(ctx, lokiPVCKey, objectOfTypePVC).Return(errForbidden)
			Expect(ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, runtimeClient, new80GiStorageQuantity, log)).ToNot(Succeed())
		})

		It("should not neglect errors when patching garden/loki's StatefulSet", func() {
			runtimeClient.EXPECT().Get(ctx, lokiPVCKey, objectOfTypePVC).DoAndReturn(funcGetLokiPVC)
			runtimeClient.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch).Return(errForbidden)
			Expect(ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, runtimeClient, new80GiStorageQuantity, log)).ToNot(Succeed())
		})

		It("should not neglect errors when getting garden/loki's StatefulSet", func() {
			runtimeClient.EXPECT().Get(ctx, lokiPVCKey, objectOfTypePVC).DoAndReturn(funcGetLokiPVC)
			runtimeClient.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch)
			runtimeClient.EXPECT().Get(gomock.Any(), lokiStatefulSetKey, objectOfTypeSTS).Return(errForbidden)
			Expect(ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, runtimeClient, new80GiStorageQuantity, log)).ToNot(Succeed())
		})

		It("should not neglect errors when patching garden/loki's PVC", func() {
			runtimeClient.EXPECT().Get(ctx, lokiPVCKey, objectOfTypePVC).DoAndReturn(funcGetLokiPVC)
			runtimeClient.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch)
			runtimeClient.EXPECT().Get(gomock.Any(), lokiStatefulSetKey, objectOfTypeSTS).DoAndReturn(funcGetScaledToZeroLokiStatefulset)
			runtimeClient.EXPECT().Patch(ctx, objectOfTypePVC, gomock.AssignableToTypeOf(patch)).Return(errForbidden)
			Expect(ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, runtimeClient, new200GiStorageQuantity, log)).ToNot(Succeed())
		})

		It("should not neglect errors when deleting garden/loki's PVC", func() {
			runtimeClient.EXPECT().Get(ctx, lokiPVCKey, objectOfTypePVC).DoAndReturn(funcGetLokiPVC)
			runtimeClient.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch)
			runtimeClient.EXPECT().Get(gomock.Any(), lokiStatefulSetKey, objectOfTypeSTS).DoAndReturn(funcGetScaledToZeroLokiStatefulset)
			runtimeClient.EXPECT().Delete(ctx, lokiPVC).Return(errForbidden)
			Expect(ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, runtimeClient, new80GiStorageQuantity, log)).ToNot(Succeed())
		})

		It("should not neglect errors when deleting garden/loki's StatefulSet", func() {
			runtimeClient.EXPECT().Get(ctx, lokiPVCKey, objectOfTypePVC).DoAndReturn(funcGetLokiPVC)
			runtimeClient.EXPECT().Patch(ctx, statefulset, zeroReplicaRawPatch)
			runtimeClient.EXPECT().Get(gomock.Any(), lokiStatefulSetKey, objectOfTypeSTS).DoAndReturn(funcGetScaledToZeroLokiStatefulset)
			runtimeClient.EXPECT().Delete(ctx, lokiPVC)
			runtimeClient.EXPECT().Delete(ctx, statefulset).Return(errForbidden)
			Expect(ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, runtimeClient, new80GiStorageQuantity, log)).ToNot(Succeed())
		})
	})
})
