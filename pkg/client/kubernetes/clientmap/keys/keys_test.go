// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package keys_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
)

var _ = Describe("Keys", func() {
	It("#ForGarden", func() {
		garden := &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name: "inception",
			},
		}
		key := keys.ForGarden(garden).(clientmap.GardenClientSetKey)
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
		key := keys.ForShoot(shoot).(clientmap.ShootClientSetKey)
		Expect(key.Key()).To(Equal(shoot.Namespace + "/" + shoot.Name))
		Expect(key.Namespace).To(Equal(shoot.Namespace))
		Expect(key.Name).To(Equal(shoot.Name))
	})

	It("#ForShootWithNamespacedName", func() {
		name := "inception"
		namespace := "core"
		key := keys.ForShootWithNamespacedName(namespace, name).(clientmap.ShootClientSetKey)
		Expect(key.Key()).To(Equal(namespace + "/" + name))
		Expect(key.Namespace).To(Equal(namespace))
		Expect(key.Name).To(Equal(name))
	})
})
