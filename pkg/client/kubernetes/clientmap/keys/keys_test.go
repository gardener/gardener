// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package keys_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/internal"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
)

var _ = Describe("Keys", func() {
	It("#ForGarden", func() {
		garden := &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name: "inception",
			},
		}
		key := keys.ForGarden(garden).(internal.GardenClientSetKey)
		Expect(key.Key()).To(Equal(garden.Name))
		Expect(key.Name).To(Equal(garden.Name))
	})

	It("#ForShoot", func() {
		shoot := &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "inception",
				Namespace: "core",
			},
		}
		key := keys.ForShoot(shoot).(internal.ShootClientSetKey)
		Expect(key.Key()).To(Equal(shoot.Namespace + "/" + shoot.Name))
		Expect(key.Namespace).To(Equal(shoot.Namespace))
		Expect(key.Name).To(Equal(shoot.Name))
	})

	It("#ForShootWithNamespacedName", func() {
		name := "inception"
		namespace := "core"
		key := keys.ForShootWithNamespacedName(namespace, name).(internal.ShootClientSetKey)
		Expect(key.Key()).To(Equal(namespace + "/" + name))
		Expect(key.Namespace).To(Equal(namespace))
		Expect(key.Name).To(Equal(name))
	})
})
