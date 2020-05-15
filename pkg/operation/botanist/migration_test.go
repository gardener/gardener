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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsclient "github.com/gardener/gardener/pkg/client/extensions/clientset/versioned/scheme"
	mock "github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/shoot"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("control plane migration", func() {
	var (
		ctrl              *gomock.Controller
		k8sSeedClient     *mock.MockInterface
		testSeedNamespace = "test-seed-namespace"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		k8sSeedClient = mock.NewMockInterface(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#AnnotateExtensionCRDsForMigration()", func() {
		It("should annotate all extension objects", func() {
			var (
				ctx   = context.TODO()
				shoot = &shoot.Shoot{
					SeedNamespace: testSeedNamespace,
				}
				extensionClientFake = fake.NewFakeClientWithScheme(extensionsclient.Scheme, &extensionsv1alpha1.Worker{ObjectMeta: metav1.ObjectMeta{
					Name:      "test-worker",
					Namespace: testSeedNamespace,
				}})
			)

			k8sSeedClient.EXPECT().Client().Return(extensionClientFake).AnyTimes()
			op := &operation.Operation{
				K8sSeedClient: k8sSeedClient,
				Shoot:         shoot,
			}

			botanist := botanist.Botanist{Operation: op}
			err := botanist.AnnotateExtensionCRsForMigration(ctx)
			Expect(err).NotTo(HaveOccurred())

			actualWorker := &extensionsv1alpha1.Worker{}
			err = extensionClientFake.Get(ctx, types.NamespacedName{
				Name:      "test-worker",
				Namespace: testSeedNamespace,
			}, actualWorker)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualWorker.GetAnnotations()).NotTo(BeNil())
			Expect(actualWorker.GetAnnotations()[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationMigrate))
		})
	})
})
