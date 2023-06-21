// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
)

var _ = Describe("botanist", func() {
	var (
		ctx          = context.TODO()
		shootState   *gardencorev1beta1.ShootState
		shoot        *gardencorev1beta1.Shoot
		gardenClient client.Client
		botanist     *Botanist
	)

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fakeShootName",
				Namespace: "fakeShootNS",
			},
		}
		shootState = &gardencorev1beta1.ShootState{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "ShootState",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            shoot.Name,
				Namespace:       shoot.Namespace,
				ResourceVersion: "1",
			},
		}

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		botanist = &Botanist{
			Operation: &operation.Operation{
				GardenClient: gardenClient,
				Shoot:        &shootpkg.Shoot{},
			},
		}
		botanist.Shoot.SetInfo(shoot)
	})

	Describe("#EnsureShootStateExists", func() {
		It("should create ShootState and add it to the Botanist object", func() {
			Expect(botanist.EnsureShootStateExists(ctx)).To(Succeed())

			Expect(botanist.Shoot.GetShootState()).To(Equal(shootState))
		})

		It("should succeed and update Botanist object if ShootState already exists", func() {
			shootState.SetAnnotations(map[string]string{"foo": "bar"})
			shootState.ResourceVersion = ""
			Expect(gardenClient.Create(ctx, shootState)).To(Succeed())

			Expect(botanist.EnsureShootStateExists(ctx)).To(Succeed())

			Expect(botanist.Shoot.GetShootState()).To(Equal(shootState))
		})
	})
})
