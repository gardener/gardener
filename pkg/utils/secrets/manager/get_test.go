// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Get", func() {
	var m *manager

	BeforeEach(func() {
		m = &manager{store: make(secretStore)}
	})

	Describe("#Get", func() {
		name := "some-name"

		It("should return an error because no secrets found for name", func() {
			result, found := m.Get(name)
			Expect(found).To(BeFalse())
			Expect(result).To(BeNil())
		})

		Context("bundle", func() {
			It("should return an error since there is no bundle secret in the internal store", func() {
				currentSecret := secretForClass(current)
				Expect(m.addToStore(name, currentSecret, current)).To(Succeed())

				result, found := m.Get(name, Bundle)
				Expect(found).To(BeFalse())
				Expect(result).To(BeNil())
			})

			secret := secretForClass(bundle)

			It("should get the bundle secret from the internal store", func() {
				Expect(m.addToStore(name, secret, bundle)).To(Succeed())

				result, found := m.Get(name, Bundle)
				Expect(found).To(BeTrue())
				Expect(result).To(Equal(secret))
			})

			It("should get the bundle secret from the internal store (w/o explicit option)", func() {
				Expect(m.addToStore(name, secret, bundle)).To(Succeed())

				result, found := m.Get(name)
				Expect(found).To(BeTrue())
				Expect(result).To(Equal(secret))
			})

		})

		Context("current", func() {
			var (
				currentSecret = secretForClass(current)
				bundleSecret  = secretForClass(bundle)
			)

			BeforeEach(func() {
				Expect(m.addToStore(name, currentSecret, current)).To(Succeed())
			})

			It("should get the bundle secret from the internal store (default behaviour w/o options)", func() {
				Expect(m.addToStore(name, bundleSecret, bundle)).To(Succeed())

				result, found := m.Get(name)
				Expect(found).To(BeTrue())
				Expect(result).To(Equal(bundleSecret))
			})

			It("should get the current secret from the internal store since there is no bundle secret", func() {
				result, found := m.Get(name)
				Expect(found).To(BeTrue())
				Expect(result).To(Equal(currentSecret))
			})

			It("should get the current secret from the internal store despite a bundle secret (w/ explicit option)", func() {
				Expect(m.addToStore(name, bundleSecret, bundle)).To(Succeed())

				result, found := m.Get(name, Current)
				Expect(found).To(BeTrue())
				Expect(result).To(Equal(currentSecret))
			})
		})

		Context("old", func() {
			It("should return an error since there is no old secret in the internal store", func() {
				currentSecret := secretForClass(current)
				Expect(m.addToStore(name, currentSecret, current)).To(Succeed())

				result, found := m.Get(name, Old)
				Expect(found).To(BeFalse())
				Expect(result).To(BeNil())
			})

			It("should get the old secret from the internal store", func() {
				oldSecret := secretForClass(old)
				Expect(m.addToStore(name, oldSecret, old)).To(Succeed())

				result, found := m.Get(name, Old)
				Expect(found).To(BeTrue())
				Expect(result).To(Equal(oldSecret))
			})
		})

		When("the secret must be loaded from the cluster", func() {
			var (
				ctx        = context.Background()
				namespace  = "shoot--foo--bar"
				identity   = "test"
				fakeClient client.Client
			)

			secretInCluster := func(suffix string) *corev1.Secret {
				return &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name + "-" + suffix,
						Namespace: namespace,
						Labels: map[string]string{
							"managed-by":       "secrets-manager",
							"manager-identity": identity,
							"name":             name,
						},
					},
				}
			}

			BeforeEach(func() {
				fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()

				mgr, err := New(ctx, logr.Discard(), testclock.NewFakeClock(time.Time{}), fakeClient, identity, WithNamespaces(namespace))
				Expect(err).NotTo(HaveOccurred())
				m = mgr.(*manager)
			})

			It("should not load from the cluster when the option is not set", func() {
				Expect(fakeClient.Create(ctx, secretInCluster("current"))).To(Succeed())

				result, found := m.Get(name)
				Expect(found).To(BeFalse())
				Expect(result).To(BeNil())
			})

			It("should load the secret from the cluster into the internal store", func() {
				secret := secretInCluster("current")
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())

				result, found := m.Get(name, LoadMissingSecretFromCluster(ctx))
				Expect(found).To(BeTrue())
				Expect(result.Name).To(Equal(name + "-current"))

				By("Verify the secret was added to the internal store")
				_, found = m.getFromStore(name)
				Expect(found).To(BeTrue())
			})

			It("should return the old secret when both current and old exist in the cluster", func() {
				oldSecret := secretInCluster("old")
				oldSecret.CreationTimestamp = metav1.NewTime(time.Unix(1, 0))
				currentSecret := secretInCluster("current")
				currentSecret.CreationTimestamp = metav1.NewTime(time.Unix(2, 0))
				Expect(fakeClient.Create(ctx, oldSecret)).To(Succeed())
				Expect(fakeClient.Create(ctx, currentSecret)).To(Succeed())

				result, found := m.Get(name, Old, LoadMissingSecretFromCluster(ctx))
				Expect(found).To(BeTrue())
				Expect(result.Name).To(Equal(name + "-old"))
			})

			It("should return that no secret was found when it does not exist in the cluster", func() {
				result, found := m.Get(name, LoadMissingSecretFromCluster(ctx))
				Expect(found).To(BeFalse())
				Expect(result).To(BeNil())
			})
		})
	})
})

func secretForClass(class secretClass) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(class) + "secret",
		},
	}
}
