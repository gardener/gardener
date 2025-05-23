// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy"
)

var _ = Describe("Add", func() {
	Describe("#ClusterPredicate", func() {
		var (
			p       predicate.Predicate
			cluster *extensionsv1alpha1.Cluster
			shoot   *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			p = ClusterPredicate()
			shoot = &gardencorev1beta1.Shoot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.gardener.cloud/v1beta1",
					Kind:       "Shoot",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "garden-bar",
				},
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{},
					},
				},
			}
			cluster = &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: extensionsv1alpha1.ClusterSpec{
					Shoot: runtime.RawExtension{
						Raw: encode(shoot),
					},
				},
			}
		})

		It("should return false if networking is nil for workerless shoot", func() {
			newCluster := cluster.DeepCopy()

			Expect(p.Create(event.CreateEvent{Object: cluster})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newCluster, ObjectOld: cluster})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: cluster})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: cluster})).To(BeFalse())
		})

		It("should return true if networking is updated to non-nil for workerless shoot", func() {
			newCluster := cluster.DeepCopy()
			shoot.Spec.Networking = &gardencorev1beta1.Networking{}
			newCluster.Spec.Shoot.Raw = encode(shoot)

			Expect(p.Create(event.CreateEvent{Object: cluster})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newCluster, ObjectOld: cluster})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: cluster})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: cluster})).To(BeFalse())
		})

		It("should return true if service cidr is changed for workerless shoot", func() {
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Services: ptr.To("foo")}
			cluster.Spec.Shoot.Raw = encode(shoot)
			newCluster := cluster.DeepCopy()
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Services: ptr.To("bar")}
			newCluster.Spec.Shoot.Raw = encode(shoot)

			Expect(p.Create(event.CreateEvent{Object: cluster})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newCluster, ObjectOld: cluster})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: cluster})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: cluster})).To(BeFalse())
		})

		It("should return false if no change in networking for shoot with workers", func() {
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Pods: ptr.To("foo")}
			shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{Name: "test"}}
			cluster.Spec.Shoot.Raw = encode(shoot)
			newCluster := cluster.DeepCopy()

			Expect(p.Create(event.CreateEvent{Object: cluster})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newCluster, ObjectOld: cluster})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: cluster})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: cluster})).To(BeFalse())
		})

		It("should return true if pods cidr is changed for shoot with workers", func() {
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Pods: ptr.To("foo")}
			shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{Name: "test"}}
			cluster.Spec.Shoot.Raw = encode(shoot)
			newCluster := cluster.DeepCopy()
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Pods: ptr.To("bar")}
			newCluster.Spec.Shoot.Raw = encode(shoot)

			Expect(p.Create(event.CreateEvent{Object: cluster})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newCluster, ObjectOld: cluster})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: cluster})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: cluster})).To(BeFalse())
		})

		It("should return true if services cidr is changed for shoot with workers", func() {
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Services: ptr.To("foo")}
			shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{Name: "test"}}
			cluster.Spec.Shoot.Raw = encode(shoot)
			newCluster := cluster.DeepCopy()
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Services: ptr.To("bar")}
			newCluster.Spec.Shoot.Raw = encode(shoot)

			Expect(p.Create(event.CreateEvent{Object: cluster})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newCluster, ObjectOld: cluster})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: cluster})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: cluster})).To(BeFalse())
		})

		It("should return true if nodes cidr is changed for shoot with workers", func() {
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Nodes: ptr.To("foo")}
			shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{Name: "test"}}
			cluster.Spec.Shoot.Raw = encode(shoot)
			newCluster := cluster.DeepCopy()
			shoot.Spec.Networking = &gardencorev1beta1.Networking{Nodes: ptr.To("bar")}
			newCluster.Spec.Shoot.Raw = encode(shoot)

			Expect(p.Create(event.CreateEvent{Object: cluster})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newCluster, ObjectOld: cluster})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: cluster})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: cluster})).To(BeFalse())
		})

		It("should return true if status networking is changed for shoot with workers", func() {
			shoot.Spec.Networking = &gardencorev1beta1.Networking{}
			shoot.Status.Networking = &gardencorev1beta1.NetworkingStatus{
				Pods:     []string{"foo"},
				Services: []string{"bar"},
				Nodes:    []string{"baz"},
			}
			shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{Name: "test"}}
			cluster.Spec.Shoot.Raw = encode(shoot)
			newCluster := cluster.DeepCopy()
			shoot.Status.Networking = &gardencorev1beta1.NetworkingStatus{
				Pods:     []string{"bar"},
				Services: []string{"baz"},
				Nodes:    []string{"foo"},
			}
			newCluster.Spec.Shoot.Raw = encode(shoot)

			Expect(p.Create(event.CreateEvent{Object: cluster})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newCluster, ObjectOld: cluster})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: cluster})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: cluster})).To(BeFalse())
		})

		It("should return false if status networking is constant for shoot with workers", func() {
			shoot.Spec.Networking = &gardencorev1beta1.Networking{}
			shoot.Status.Networking = &gardencorev1beta1.NetworkingStatus{
				Pods:     []string{"foo", "bar"},
				Services: []string{"bar", "baz"},
				Nodes:    []string{"baz", "foo"},
			}
			shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{Name: "test"}}
			cluster.Spec.Shoot.Raw = encode(shoot)
			newCluster := cluster.DeepCopy()
			shoot.Status.Networking = &gardencorev1beta1.NetworkingStatus{
				Pods:     []string{"foo", "bar"},
				Services: []string{"bar", "baz"},
				Nodes:    []string{"baz", "foo"},
			}
			newCluster.Spec.Shoot.Raw = encode(shoot)

			Expect(p.Create(event.CreateEvent{Object: cluster})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newCluster, ObjectOld: cluster})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: cluster})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: cluster})).To(BeFalse())
		})
	})
})

func encode(shoot *gardencorev1beta1.Shoot) []byte {
	raw, err := json.Marshal(shoot)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return raw
}
