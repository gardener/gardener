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
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mockkubernetes "github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/operation/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Namespaces", func() {
	var (
		ctrl             *gomock.Controller
		kubernetesClient *mockkubernetes.MockInterface
		c                *mockclient.MockClient

		botanist *Botanist

		ctx       = context.TODO()
		fakeErr   = fmt.Errorf("fake error")
		namespace = "shoot--foo--bar"

		obj = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		kubernetesClient = mockkubernetes.NewMockInterface(ctrl)
		c = mockclient.NewMockClient(ctrl)

		botanist = &Botanist{Operation: &operation.Operation{
			K8sSeedClient: kubernetesClient,
			Shoot:         &shoot.Shoot{SeedNamespace: namespace},
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
		)

		BeforeEach(func() {
			botanist.Seed = &seed.Seed{Info: &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Provider: gardencorev1beta1.SeedProvider{
						Type: seedProviderType,
					},
				},
			}}
			botanist.Shoot.Info = &gardencorev1beta1.Shoot{
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

			obj = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
					Annotations: map[string]string{
						"shoot.gardener.cloud/uid":     string(uid),
						"shoot.garden.sapcloud.io/uid": string(uid),
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
				kubernetesClient.EXPECT().Client().Return(c),
				c.EXPECT().Get(ctx, kutil.Key(namespace), gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
				c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(fakeErr),
			)

			Expect(botanist.DeploySeedNamespace(ctx)).To(MatchError(fakeErr))
			Expect(botanist.SeedNamespaceObject).To(BeNil())
		})

		It("should successfully deploy the namespace w/o dedicated backup provider", func() {
			gomock.InOrder(
				kubernetesClient.EXPECT().Client().Return(c),
				c.EXPECT().Get(ctx, kutil.Key(namespace), gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
				c.EXPECT().Create(ctx, obj),
			)

			Expect(botanist.DeploySeedNamespace(ctx)).To(Succeed())
			Expect(botanist.SeedNamespaceObject).To(Equal(obj))
		})

		It("should successfully deploy the namespace w/ dedicated backup provider", func() {
			botanist.Seed.Info.Spec.Backup = &gardencorev1beta1.SeedBackup{Provider: backupProviderType}
			obj.Labels["backup.gardener.cloud/provider"] = backupProviderType

			gomock.InOrder(
				kubernetesClient.EXPECT().Client().Return(c),
				c.EXPECT().Get(ctx, kutil.Key(namespace), gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
				c.EXPECT().Create(ctx, obj),
			)

			Expect(botanist.DeploySeedNamespace(ctx)).To(Succeed())
			Expect(botanist.SeedNamespaceObject).To(Equal(obj))
		})
	})

	Describe("#DeleteSeedNamespace", func() {
		It("should fail to delete the namespace", func() {
			gomock.InOrder(
				kubernetesClient.EXPECT().Client().Return(c),
				c.EXPECT().Delete(ctx, obj, kubernetes.DefaultDeleteOptions).Return(fakeErr),
			)

			Expect(botanist.DeleteSeedNamespace(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully delete the namespace despite 'not found' error", func() {
			gomock.InOrder(
				kubernetesClient.EXPECT().Client().Return(c),
				c.EXPECT().Delete(ctx, obj, kubernetes.DefaultDeleteOptions).Return(apierrors.NewNotFound(schema.GroupResource{}, "")),
			)

			Expect(botanist.DeleteSeedNamespace(ctx)).To(Succeed())
		})

		It("should successfully delete the namespace despite 'conflict' error", func() {
			gomock.InOrder(
				kubernetesClient.EXPECT().Client().Return(c),
				c.EXPECT().Delete(ctx, obj, kubernetes.DefaultDeleteOptions).Return(apierrors.NewConflict(schema.GroupResource{}, "", fakeErr)),
			)

			Expect(botanist.DeleteSeedNamespace(ctx)).To(Succeed())
		})

		It("should successfully delete the namespace (no error)", func() {
			gomock.InOrder(
				kubernetesClient.EXPECT().Client().Return(c),
				c.EXPECT().Delete(ctx, obj, kubernetes.DefaultDeleteOptions),
			)

			Expect(botanist.DeleteSeedNamespace(ctx)).To(Succeed())
		})
	})
})
