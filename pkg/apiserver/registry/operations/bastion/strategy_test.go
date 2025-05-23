// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/operations"
)

var _ = Describe("ToSelectableFields", func() {
	It("should return correct fields", func() {
		result := ToSelectableFields(newBastion("shoot", "foo"))

		Expect(result).To(HaveLen(4))
		Expect(result.Has("metadata.name")).To(BeTrue())
		Expect(result.Get("metadata.name")).To(Equal("test"))
		Expect(result.Has("metadata.namespace")).To(BeTrue())
		Expect(result.Get("metadata.namespace")).To(Equal("test-namespace"))
		Expect(result.Has(operations.BastionSeedName)).To(BeTrue())
		Expect(result.Get(operations.BastionSeedName)).To(Equal("foo"))
		Expect(result.Has(operations.BastionShootName)).To(BeTrue())
		Expect(result.Get(operations.BastionShootName)).To(Equal("shoot"))
	})
})

var _ = Describe("GetAttrs", func() {
	It("should return error when object is not Bastion", func() {
		_, _, err := GetAttrs(&gardencore.Seed{})
		Expect(err).To(HaveOccurred())
	})

	It("should return correct result", func() {
		ls, fs, err := GetAttrs(newBastion("shoot", "foo"))

		Expect(ls).To(HaveLen(1))
		Expect(ls.Get("foo")).To(Equal("bar"))
		Expect(fs.Get(operations.BastionSeedName)).To(Equal("foo"))
		Expect(fs.Get(operations.BastionShootName)).To(Equal("shoot"))
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("SeedNameTriggerFunc", func() {
	It("should return spec.seedName", func() {
		actual := SeedNameTriggerFunc(newBastion("shoot", "foo"))
		Expect(actual).To(Equal("foo"))
	})
})

var _ = Describe("MatchBastion", func() {
	It("should return correct predicate", func() {
		ls, _ := labels.Parse("app=test")
		fs := fields.OneTermEqualSelector(operations.BastionSeedName, "foo")

		result := MatchBastion(ls, fs)

		Expect(result.Label).To(Equal(ls))
		Expect(result.Field).To(Equal(fs))
		Expect(result.IndexFields).To(ConsistOf(operations.BastionSeedName))
	})
})

var _ = Describe("PrepareForCreate", func() {
	It("should perform an initial heartbeat", func() {
		bastion := operations.Bastion{}

		Strategy.PrepareForCreate(context.TODO(), &bastion)

		Expect(bastion.Generation).NotTo(BeZero())
		Expect(bastion.Status.LastHeartbeatTimestamp).NotTo(BeNil())
		Expect(bastion.Status.ExpirationTimestamp).NotTo(BeNil())
		Expect(bastion.Annotations[v1beta1constants.GardenerOperation]).To(BeEmpty())
	})

	It("should remove operation annotation even on creates", func() {
		bastion := operations.Bastion{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationKeepalive,
				},
			},
		}

		Strategy.PrepareForCreate(context.TODO(), &bastion)
		Expect(bastion.Annotations[v1beta1constants.GardenerOperation]).To(BeEmpty())
	})
})

var _ = Describe("PrepareForUpdate", func() {
	It("should not perform heartbeat if no annotation is set", func() {
		bastion := operations.Bastion{}

		Strategy.PrepareForUpdate(context.TODO(), &bastion, &bastion)

		Expect(bastion.Status.LastHeartbeatTimestamp).To(BeNil())
		Expect(bastion.Status.ExpirationTimestamp).To(BeNil())
	})

	It("should perform the heartbeat when the annotation is set", func() {
		bastion := operations.Bastion{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationKeepalive,
				},
			},
		}

		Strategy.PrepareForUpdate(context.TODO(), &bastion, &bastion)
		Expect(bastion.Status.LastHeartbeatTimestamp).NotTo(BeNil())
		Expect(bastion.Status.ExpirationTimestamp).NotTo(BeNil())
		Expect(bastion.Annotations[v1beta1constants.GardenerOperation]).To(BeEmpty())
	})

	Context("generation increment", func() {
		var (
			oldBastion *operations.Bastion
			newBastion *operations.Bastion
		)

		BeforeEach(func() {
			oldBastion = &operations.Bastion{}
			newBastion = oldBastion.DeepCopy()
		})

		DescribeTable("standard tests",
			func(mutateNewBastion func(*operations.Bastion), shouldIncreaseGeneration bool) {
				if mutateNewBastion != nil {
					mutateNewBastion(newBastion)
				}

				Strategy.PrepareForUpdate(context.TODO(), newBastion, oldBastion)

				expectedGeneration := oldBastion.Generation
				if shouldIncreaseGeneration {
					expectedGeneration++
				}

				Expect(newBastion.Generation).To(Equal(expectedGeneration))
			},

			Entry("no change",
				nil,
				false,
			),
			Entry("only label change",
				func(b *operations.Bastion) { b.Labels = map[string]string{"foo": "bar"} },
				false,
			),
			Entry("some spec change",
				func(b *operations.Bastion) { b.Spec.SSHPublicKey = "foo" },
				true,
			),
			Entry("deletion timestamp gets set",
				func(b *operations.Bastion) {
					deletionTimestamp := metav1.Now()
					b.DeletionTimestamp = &deletionTimestamp
				},
				true,
			),
			Entry("force-deletion annotation",
				func(b *operations.Bastion) {
					metav1.SetMetaDataAnnotation(&b.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "true")
				},
				true,
			),
		)
	})
})

var _ = Describe("heartbeat", func() {
	It("should delete keepalive annotation", func() {
		bastion := operations.Bastion{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationKeepalive,
				},
			},
		}

		Strategy.heartbeat(&bastion)

		Expect(bastion.Annotations[v1beta1constants.GardenerOperation]).To(BeEmpty())
	})

	It("should create expirations that are after the heartbeat", func() {
		bastion := operations.Bastion{}

		Strategy.heartbeat(&bastion)

		Expect(bastion.Status.LastHeartbeatTimestamp).NotTo(BeNil())
		Expect(bastion.Status.ExpirationTimestamp).NotTo(BeNil())

		heartbeat := bastion.Status.LastHeartbeatTimestamp.Time
		expires := bastion.Status.ExpirationTimestamp.Time

		Expect(expires).Should(BeTemporally(">", heartbeat))
	})
})

func newBastion(shootName string, seedName string) *operations.Bastion {
	return &operations.Bastion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec: operations.BastionSpec{
			ShootRef: corev1.LocalObjectReference{
				Name: shootName,
			},
			SeedName: &seedName,
		},
	}
}
