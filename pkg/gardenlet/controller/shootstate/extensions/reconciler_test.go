// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extensions_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shootstate/extensions"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
)

var _ = Describe("Reconciler", func() {
	var (
		objectKind    = extensionsv1alpha1.ExtensionResource
		newObjectFunc = func() client.Object { return &extensionsv1alpha1.Extension{} }

		fakeGardenClient client.Client
		fakeSeedClient   client.Client
		reconciler       *Reconciler

		ctx = context.TODO()

		shootName             = "my-shoot"
		projectName           = "my-project"
		secretName            = "my-secret"
		clusterName           = fmt.Sprintf("shoot--%s--%s", projectName, shootName)
		controlPlaneNamespace = clusterName
		projectNamespace      = fmt.Sprintf("garden-%s", projectName)
		secretDataJSON        = []byte(fmt.Sprintf(`{"apiVersion":"v1","data":{"data":"c29tZSBzZWNyZXQgZGF0YQ=="},"kind":"Secret","metadata":{"name":"%s","namespace":"%s"}}`, secretName, controlPlaneNamespace))

		reconcileRequest = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: controlPlaneNamespace,
				Name:      shootName,
			},
		}

		shootState                  *gardencorev1beta1.ShootState
		shootStateWithExtensionData *gardencorev1beta1.ShootState
		cluster                     *extensionsv1alpha1.Cluster
		extension                   *extensionsv1alpha1.Extension
		extensionState              *runtime.RawExtension
		extensionResources          []gardencorev1beta1.NamedResourceReference
		stateResources              []gardencorev1beta1.ResourceData
		secretObj                   *corev1.Secret
	)

	BeforeEach(func() {
		gardenScheme := runtime.NewScheme()
		Expect(gardencorev1alpha1.AddToScheme(gardenScheme)).NotTo(HaveOccurred())
		Expect(gardencorev1beta1.AddToScheme(gardenScheme)).NotTo(HaveOccurred())
		fakeGardenClient = fake.NewClientBuilder().WithScheme(gardenScheme).Build()

		seedScheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(seedScheme)).NotTo(HaveOccurred())
		Expect(extensionsv1alpha1.AddToScheme(seedScheme)).NotTo(HaveOccurred())
		fakeSeedClient = fake.NewClientBuilder().WithScheme(seedScheme).Build()

		reconciler = &Reconciler{
			GardenClient:  fakeGardenClient,
			SeedClient:    fakeSeedClient,
			ObjectKind:    objectKind,
			NewObjectFunc: newObjectFunc,
		}

		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterName,
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				Shoot: runtime.RawExtension{
					Raw: encode(&gardencorev1beta1.Shoot{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
							Kind:       "Shoot",
						},
						ObjectMeta: metav1.ObjectMeta{
							Namespace: projectNamespace,
							Name:      shootName,
						},
					}),
				},
			},
		}

		shootState = &gardencorev1beta1.ShootState{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: projectNamespace,
				Name:      shootName,
			},
		}

		secretObj = &corev1.Secret{}
		Expect(json.Unmarshal(secretDataJSON, secretObj)).To(Succeed())

		stateResources = []gardencorev1beta1.ResourceData{
			{
				CrossVersionObjectReference: autoscalingv1.CrossVersionObjectReference{
					Name:       secretName,
					Kind:       "Secret",
					APIVersion: "v1",
				},
				Data: runtime.RawExtension{
					Raw: secretDataJSON,
				},
			},
		}
		extensionState = &runtime.RawExtension{
			Raw: []byte(`{"data":"data1"}`),
		}

		extensionResources = make([]gardencorev1beta1.NamedResourceReference, 0, len(stateResources))
		for _, resource := range stateResources {
			extensionResources = append(extensionResources, gardencorev1beta1.NamedResourceReference{
				Name:        resource.Name,
				ResourceRef: resource.CrossVersionObjectReference,
			})
		}

		extension = &extensionsv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: controlPlaneNamespace,
			},
			Status: extensionsv1alpha1.ExtensionStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{
					State: extensionState.DeepCopy(),
				},
			},
		}

		shootStateWithExtensionData = &gardencorev1beta1.ShootState{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: projectNamespace,
				Name:      shootName,
			},
			Spec: gardencorev1beta1.ShootStateSpec{
				Extensions: []gardencorev1beta1.ExtensionResourceState{
					{
						Kind:  extensionsv1alpha1.ExtensionResource,
						Name:  &shootName,
						State: extensionState.DeepCopy(),
					},
				},
			},
		}
	})

	Describe("#CreateShootStateSyncReconcileFunc", func() {
		It("should properly update ShootState with extension state", func() {
			Expect(fakeGardenClient.Create(ctx, shootState)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, extension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, cluster)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcileRequest)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeGardenClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
			Expect(shootState.Spec).To(Equal(shootStateWithExtensionData.Spec))
		})

		It("should properly update ShootState with extension state and resources", func() {
			extension.Status.Resources = extensionResources
			shootStateWithExtensionData.Spec.Resources = stateResources
			shootStateWithExtensionData.Spec.Extensions[0].Resources = extensionResources

			Expect(fakeGardenClient.Create(ctx, shootState)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, extension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, secretObj)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, cluster)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcileRequest)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeGardenClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
			Expect(shootState.Spec).To(Equal(shootStateWithExtensionData.Spec))
		})

		It("should properly update ShootState with extension state if it was changed", func() {
			extension.Status.State = &runtime.RawExtension{Raw: []byte(`{"data":"newdata"}`)}
			expectedShootState := shootStateWithExtensionData.DeepCopy()
			expectedShootState.Spec.Extensions[0].State = extension.Status.State

			Expect(fakeGardenClient.Create(ctx, shootStateWithExtensionData)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, extension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, cluster)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcileRequest)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeGardenClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
			Expect(shootState.Spec).To(Equal(expectedShootState.Spec))
		})

		It("should update ShootState with extension resources if they changed", func() {
			extension.Status.Resources = extensionResources
			shootStateWithExtensionData.Spec.Resources = stateResources
			shootStateWithExtensionData.Spec.Extensions[0].Resources = extensionResources

			newSecretJSON := []byte(fmt.Sprintf(`{"apiVersion":"v1","data":{"data":"bmV3IHNlY3JldCBkYXRh"},"kind":"Secret","metadata":{"name":"%s","namespace":"%s"}}`, secretName, controlPlaneNamespace))
			newSecretObj := &corev1.Secret{}
			Expect(json.Unmarshal(newSecretJSON, newSecretObj)).To(Succeed())
			expectedShootState := shootStateWithExtensionData.DeepCopy()
			expectedShootState.Spec.Resources[0].Data.Raw = newSecretJSON

			Expect(fakeGardenClient.Create(ctx, shootStateWithExtensionData)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, extension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, newSecretObj)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, cluster)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcileRequest)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeGardenClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
			Expect(shootState.Spec).To(Equal(expectedShootState.Spec))
		})

		It("should remove the extension state from the ShootState if its new value is null", func() {
			expectedShootState := shootState.DeepCopy()
			extension.Status.State = nil
			Expect(fakeGardenClient.Create(ctx, shootStateWithExtensionData)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, extension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, cluster)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcileRequest)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeGardenClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
			Expect(shootState.Spec).To(Equal(expectedShootState.Spec))
		})

		It("should delete resources which do not exist in the new extension state", func() {
			expectedShootState := shootStateWithExtensionData.DeepCopy()

			shootStateWithExtensionData.Spec.Resources = stateResources
			shootStateWithExtensionData.Spec.Extensions[0].Resources = extensionResources

			Expect(fakeGardenClient.Create(ctx, shootStateWithExtensionData)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, extension)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, cluster)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcileRequest)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeGardenClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
			Expect(shootState.Spec).To(Equal(expectedShootState.Spec))
		})

		It("should not try to patch the ShootState if there are no changes to the extension state", func() {
			ctrl := gomock.NewController(GinkgoT())
			mockClient := mockclient.NewMockClient(ctrl)
			reconciler.GardenClient = mockClient
			reconciler.SeedClient = mockClient

			gomock.InOrder(
				mockClient.EXPECT().Get(ctx, client.ObjectKeyFromObject(extension), gomock.AssignableToTypeOf(&extensionsv1alpha1.Extension{})),
				mockClient.EXPECT().Get(ctx, client.ObjectKeyFromObject(cluster), gomock.AssignableToTypeOf(&extensionsv1alpha1.Cluster{})).SetArg(2, *cluster),
				mockClient.EXPECT().Get(ctx, client.ObjectKeyFromObject(shootState), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootState{})),
			)

			_, err := reconciler.Reconcile(ctx, reconcileRequest)
			Expect(err).NotTo(HaveOccurred())

			ctrl.Finish()
		})

		It("should not throw any errors if Cluster resource does not exists", func() {
			Expect(fakeSeedClient.Create(ctx, extension)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcileRequest)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func encode(obj runtime.Object) []byte {
	out, _ := json.Marshal(obj)
	return out
}
