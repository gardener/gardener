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

package botanist_test

import (
	"context"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mock "github.com/gardener/gardener/pkg/mock/gardener/kubernetes"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/shoot"

	"github.com/golang/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("operatingsystemconfig", func() {
	var (
		ctrl                 *gomock.Controller
		k8sSeedClient        *mock.MockInterface
		k8sSeedRuntimeClient *mockclient.MockClient
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		k8sSeedClient = mock.NewMockInterface(ctrl)
		k8sSeedRuntimeClient = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#CleanupOperatingSystemConfigs", func() {
		It("should cleanup unused operating system configs", func() {
			var (
				ctx              = context.TODO()
				newDownloaderOsc = extensionsv1alpha1.OperatingSystemConfig{ObjectMeta: metav1.ObjectMeta{Name: "cloud-config-new-worker-9f0e7-downloader"}}
				newOriginalOsc   = extensionsv1alpha1.OperatingSystemConfig{ObjectMeta: metav1.ObjectMeta{Name: "cloud-config-new-worker-9f0e7-original"}}
				oldDownloaderOsc = extensionsv1alpha1.OperatingSystemConfig{ObjectMeta: metav1.ObjectMeta{Name: "cloud-config-old-worker-9f0e7-downloader"}}
				oldOriginalOsc   = extensionsv1alpha1.OperatingSystemConfig{ObjectMeta: metav1.ObjectMeta{Name: "cloud-config-old-worker-9f0e7-original"}}
			)

			k8sSeedClient.EXPECT().Client().Return(k8sSeedRuntimeClient)
			k8sSeedRuntimeClient.EXPECT().List(ctx, gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, list *extensionsv1alpha1.OperatingSystemConfigList, _ ...client.ListOptionFunc) error {
				list.Items = []extensionsv1alpha1.OperatingSystemConfig{newDownloaderOsc, newOriginalOsc, oldDownloaderOsc, oldOriginalOsc}
				return nil
			})

			// Expect that the old OperatingSystemConfigs will be cleaned up
			k8sSeedRuntimeClient.EXPECT().Delete(ctx, &oldDownloaderOsc)
			k8sSeedRuntimeClient.EXPECT().Delete(ctx, &oldOriginalOsc)

			op := &operation.Operation{
				K8sSeedClient: k8sSeedClient,
				Shoot: &shoot.Shoot{
					SeedNamespace: "shoot--foo--bar",
				},
			}
			botanist := botanist.Botanist{Operation: op}

			usedOscNames := map[string]string{
				newDownloaderOsc.Name: newDownloaderOsc.Name,
				newOriginalOsc.Name:   newOriginalOsc.Name,
			}
			err := botanist.CleanupOperatingSystemConfigs(ctx, usedOscNames)

			Expect(err).NotTo(HaveOccurred())
		})
	})
})
