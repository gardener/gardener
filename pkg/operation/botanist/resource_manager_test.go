// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	mockresourcemanager "github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ResourceManager", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployGardenerResourceManager", func() {
		var (
			resourceManager  *mockresourcemanager.MockInterface
			kubernetesClient *mockkubernetes.MockInterface
			c                *mockclient.MockClient

			ctx           = context.TODO()
			fakeErr       = fmt.Errorf("fake err")
			secretName    = "gardener-resource-manager"
			seedNamespace = "fake-seed-ns"
			checksum      = "1234"
		)

		BeforeEach(func() {
			resourceManager = mockresourcemanager.NewMockInterface(ctrl)
			kubernetesClient = mockkubernetes.NewMockInterface(ctrl)
			c = mockclient.NewMockClient(ctrl)

			botanist.StoreCheckSum(secretName, checksum)
			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						ResourceManager: resourceManager,
					},
				},
				SeedNamespace: seedNamespace,
			}
			botanist.K8sSeedClient = kubernetesClient

			resourceManager.EXPECT().SetSecrets(resourcemanager.Secrets{
				Kubeconfig: component.Secret{Name: secretName, Checksum: checksum}})

			// Expecting the deletion of Deployments with the deprecated Role labels
			deploymentWithDeprecatedLabel := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      v1beta1constants.DeploymentNameGardenerResourceManager,
					Namespace: seedNamespace,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{v1beta1constants.DeprecatedGardenRole: "fake"},
					},
				},
			}
			key := client.ObjectKey{Namespace: seedNamespace, Name: v1beta1constants.DeploymentNameGardenerResourceManager}
			kubernetesClient.EXPECT().Client().Return(c)
			c.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(&appsv1.Deployment{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					(deploymentWithDeprecatedLabel).DeepCopyInto(obj.(*appsv1.Deployment))
					return nil
				})
			c.EXPECT().Delete(ctx, deploymentWithDeprecatedLabel)
			c.EXPECT().Get(ctx, key, deploymentWithDeprecatedLabel).Return(apierrors.NewNotFound(schema.GroupResource{}, "fake"))
		})

		It("should set the secrets and deploy", func() {
			resourceManager.EXPECT().Deploy(ctx)
			Expect(botanist.DeployGardenerResourceManager(ctx)).To(Succeed())
		})

		It("should fail when the deploy function fails", func() {
			resourceManager.EXPECT().Deploy(ctx).Return(fakeErr)
			Expect(botanist.DeployGardenerResourceManager(ctx)).To(Equal(fakeErr))
		})
	})
})
