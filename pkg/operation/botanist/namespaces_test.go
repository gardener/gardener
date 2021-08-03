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

package botanist_test

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/operation/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

var _ = Describe("Namespaces", func() {
	var (
		ctrl                 *gomock.Controller
		seedKubernetesClient *mockkubernetes.MockInterface
		seedMockClient       *mockclient.MockClient

		gardenKubernetesClient *mockkubernetes.MockInterface
		gardenMockClient       *mockclient.MockClient

		botanist *Botanist

		ctx       = context.TODO()
		fakeErr   = fmt.Errorf("fake error")
		namespace = "shoot--foo--bar"

		obj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		extensionType1             = "shoot-custom-service-1"
		extensionType2             = "shoot-custom-service-2"
		extensionType3             = "shoot-custom-service-3"
		extensionType4             = "shoot-custom-service-4"
		controllerRegistrationList = &gardencorev1beta1.ControllerRegistrationList{
			Items: []gardencorev1beta1.ControllerRegistration{
				{
					Spec: gardencorev1beta1.ControllerRegistrationSpec{
						Resources: []gardencorev1beta1.ControllerResource{
							{
								Kind:            extensionsv1alpha1.ExtensionResource,
								Type:            extensionType3,
								GloballyEnabled: pointer.Bool(true),
							},
						},
					},
				},
				{
					Spec: gardencorev1beta1.ControllerRegistrationSpec{
						Resources: []gardencorev1beta1.ControllerResource{
							{
								Kind:            extensionsv1alpha1.ExtensionResource,
								Type:            extensionType4,
								GloballyEnabled: pointer.Bool(false),
							},
						},
					},
				},
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		seedKubernetesClient = mockkubernetes.NewMockInterface(ctrl)
		seedMockClient = mockclient.NewMockClient(ctrl)

		gardenKubernetesClient = mockkubernetes.NewMockInterface(ctrl)
		gardenMockClient = mockclient.NewMockClient(ctrl)

		botanist = &Botanist{Operation: &operation.Operation{
			K8sSeedClient:   seedKubernetesClient,
			K8sGardenClient: gardenKubernetesClient,
			Seed:            &seed.Seed{},
			Shoot:           &shoot.Shoot{SeedNamespace: namespace},
			Garden:          &garden.Garden{},
		}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeploySeedNamespace", func() {
		var (
			seedProviderType       = "seed-provider"
			backupProviderType     = "backup-provider"
			shootProviderType      = "shoot-provider"
			networkingProviderType = "networking-provider"
			uid                    = types.UID("12345")
			obj                    *corev1.Namespace
			defaultShootInfo       *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Provider: gardencorev1beta1.SeedProvider{
						Type: seedProviderType,
					},
					Settings: &gardencorev1beta1.SeedSettings{
						ShootDNS: &gardencorev1beta1.SeedSettingShootDNS{
							Enabled: true,
						},
					},
				},
			})

			defaultShootInfo = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Type: shootProviderType,
					},
					Networking: gardencorev1beta1.Networking{
						Type: networkingProviderType,
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					UID: uid,
				},
			}
			botanist.Shoot.SetInfo(defaultShootInfo)

			obj = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
					Annotations: map[string]string{
						"shoot.gardener.cloud/uid": string(uid),
					},
					Labels: map[string]string{
						"gardener.cloud/role":                      "shoot",
						"seed.gardener.cloud/provider":             seedProviderType,
						"shoot.gardener.cloud/provider":            shootProviderType,
						"networking.shoot.gardener.cloud/provider": networkingProviderType,
						"backup.gardener.cloud/provider":           seedProviderType,
					},
				},
			}
		})

		It("should fail to deploy the namespace", func() {
			gomock.InOrder(
				seedKubernetesClient.EXPECT().Client().Return(seedMockClient),
				seedMockClient.EXPECT().Get(ctx, kutil.Key(namespace), gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
				gardenKubernetesClient.EXPECT().Client().Return(gardenMockClient),
				gardenMockClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistrationList{})),
				seedMockClient.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(fakeErr),
			)

			Expect(botanist.DeploySeedNamespace(ctx)).To(MatchError(fakeErr))
			Expect(botanist.SeedNamespaceObject).To(BeNil())
		})

		It("should successfully deploy the namespace w/o dedicated backup provider", func() {
			gomock.InOrder(
				seedKubernetesClient.EXPECT().Client().Return(seedMockClient),
				seedMockClient.EXPECT().Get(ctx, kutil.Key(namespace), gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
				gardenKubernetesClient.EXPECT().Client().Return(gardenMockClient),
				gardenMockClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistrationList{})),
				seedMockClient.EXPECT().Create(ctx, obj),
			)

			Expect(botanist.DeploySeedNamespace(ctx)).To(Succeed())
			Expect(botanist.SeedNamespaceObject).To(Equal(obj))
		})

		It("should successfully deploy the namespace w/ dedicated backup provider", func() {
			botanist.Seed.GetInfo().Spec.Backup = &gardencorev1beta1.SeedBackup{Provider: backupProviderType}
			obj.Labels["backup.gardener.cloud/provider"] = backupProviderType

			gomock.InOrder(
				seedKubernetesClient.EXPECT().Client().Return(seedMockClient),
				seedMockClient.EXPECT().Get(ctx, kutil.Key(namespace), gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
				gardenKubernetesClient.EXPECT().Client().Return(gardenMockClient),
				gardenMockClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistrationList{})),
				seedMockClient.EXPECT().Create(ctx, obj),
			)

			Expect(botanist.DeploySeedNamespace(ctx)).To(Succeed())
			Expect(botanist.SeedNamespaceObject).To(Equal(obj))
		})

		It("should successfully deploy the namespace with enabled extension labels", func() {
			defaultShootInfo.Spec.Extensions = []gardencorev1beta1.Extension{
				{
					Type: extensionType1,
				},
				{
					Type: extensionType2,
				},
			}
			botanist.Shoot.SetInfo(defaultShootInfo)
			obj.Labels[v1beta1constants.LabelExtensionPrefix+extensionType1] = "true"
			obj.Labels[v1beta1constants.LabelExtensionPrefix+extensionType2] = "true"
			obj.Labels[v1beta1constants.LabelExtensionPrefix+extensionType3] = "true"

			gomock.InOrder(
				seedKubernetesClient.EXPECT().Client().Return(seedMockClient),
				seedMockClient.EXPECT().Get(ctx, kutil.Key(namespace), gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
				gardenKubernetesClient.EXPECT().Client().Return(gardenMockClient),
				gardenMockClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistrationList{})).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
					(controllerRegistrationList).DeepCopyInto(list.(*gardencorev1beta1.ControllerRegistrationList))
					return nil
				}),
				seedMockClient.EXPECT().Create(ctx, obj),
			)

			Expect(botanist.DeploySeedNamespace(ctx)).To(Succeed())
			Expect(botanist.SeedNamespaceObject).To(Equal(obj))
		})

		It("should successfully remove extension labels from the namespace when extensions are deleted from shoot spec or marked as disabled", func() {
			defaultShootInfo.Spec.Extensions = []gardencorev1beta1.Extension{
				{
					Type: extensionType1,
				},
				{
					Type:     extensionType3,
					Disabled: pointer.Bool(true),
				},
			}
			botanist.Shoot.SetInfo(defaultShootInfo)
			mockNamespace := obj.DeepCopy()
			mockNamespace.Labels[v1beta1constants.LabelExtensionPrefix+extensionType1] = "true"
			mockNamespace.Labels[v1beta1constants.LabelExtensionPrefix+extensionType2] = "true"
			mockNamespace.Labels[v1beta1constants.LabelExtensionPrefix+extensionType3] = "true"
			obj.Labels[v1beta1constants.LabelExtensionPrefix+extensionType1] = "true"

			gomock.InOrder(
				seedKubernetesClient.EXPECT().Client().Return(seedMockClient),
				seedMockClient.EXPECT().Get(ctx, kutil.Key(namespace), gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(_ context.Context, key client.ObjectKey, obj client.Object) error {
					(mockNamespace).DeepCopyInto(obj.(*corev1.Namespace))
					return nil
				}),
				gardenKubernetesClient.EXPECT().Client().Return(gardenMockClient),
				gardenMockClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistrationList{})).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
					(controllerRegistrationList).DeepCopyInto(list.(*gardencorev1beta1.ControllerRegistrationList))
					return nil
				}),
				seedMockClient.EXPECT().Patch(ctx, obj, gomock.AssignableToTypeOf(client.MergeFrom(&corev1.Namespace{}))).Return(nil),
			)

			Expect(botanist.DeploySeedNamespace(ctx)).To(Succeed())
			Expect(botanist.SeedNamespaceObject).To(Equal(obj))
		})
	})

	Describe("#DeleteSeedNamespace", func() {
		It("should fail to delete the namespace", func() {
			gomock.InOrder(
				seedKubernetesClient.EXPECT().Client().Return(seedMockClient),
				seedMockClient.EXPECT().Delete(ctx, obj, kubernetes.DefaultDeleteOptions).Return(fakeErr),
			)

			Expect(botanist.DeleteSeedNamespace(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully delete the namespace despite 'not found' error", func() {
			gomock.InOrder(
				seedKubernetesClient.EXPECT().Client().Return(seedMockClient),
				seedMockClient.EXPECT().Delete(ctx, obj, kubernetes.DefaultDeleteOptions).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
			)

			Expect(botanist.DeleteSeedNamespace(ctx)).To(Succeed())
		})

		It("should successfully delete the namespace despite 'conflict' error", func() {
			gomock.InOrder(
				seedKubernetesClient.EXPECT().Client().Return(seedMockClient),
				seedMockClient.EXPECT().Delete(ctx, obj, kubernetes.DefaultDeleteOptions).Return(apierrors.NewConflict(schema.GroupResource{}, "", fakeErr)),
			)

			Expect(botanist.DeleteSeedNamespace(ctx)).To(Succeed())
		})

		It("should successfully delete the namespace (no error)", func() {
			gomock.InOrder(
				seedKubernetesClient.EXPECT().Client().Return(seedMockClient),
				seedMockClient.EXPECT().Delete(ctx, obj, kubernetes.DefaultDeleteOptions),
			)

			Expect(botanist.DeleteSeedNamespace(ctx)).To(Succeed())
		})
	})
})
