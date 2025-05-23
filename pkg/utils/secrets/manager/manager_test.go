// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/utils"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

var _ = Describe("Manager", func() {
	Describe("#New", func() {
		var (
			ctx       = context.TODO()
			namespace = "some-namespace"
			identity  = "test"

			m          *manager
			fakeClient client.Client
			fakeClock  = testclock.NewFakeClock(time.Time{})
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		})

		It("should create a new instance w/ empty last rotation initiation times map", func() {
			mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{})
			Expect(err).NotTo(HaveOccurred())
			m = mgr.(*manager)

			Expect(m.lastRotationInitiationTimes).To(BeEmpty())
		})

		It("should create a new instance w/ provided last rotation initiation times", func() {
			mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{SecretNamesToTimes: map[string]time.Time{"foo": fakeClock.Now()}})
			Expect(err).NotTo(HaveOccurred())
			m = mgr.(*manager)

			Expect(m.lastRotationInitiationTimes).To(Equal(nameToUnixTime{"foo": "-62135596800"}))
		})

		It("should create a new instance w/ overwritten last rotation initiation times", func() {
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret1",
					Namespace: namespace,
					Labels: map[string]string{
						"name":                          "secret1",
						"managed-by":                    "secrets-manager",
						"manager-identity":              identity,
						"last-rotation-initiation-time": "-100",
					},
				},
			}
			Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed())

			mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{SecretNamesToTimes: map[string]time.Time{"secret1": fakeClock.Now()}})
			Expect(err).NotTo(HaveOccurred())
			m = mgr.(*manager)

			Expect(m.lastRotationInitiationTimes).To(Equal(nameToUnixTime{"secret1": "-62135596800"}))
		})

		It("should create a new instance w/ both existing and provided last rotation initiation times", func() {
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret1",
					Namespace: namespace,
					Labels: map[string]string{
						"name":                          "secret1",
						"managed-by":                    "secrets-manager",
						"manager-identity":              identity,
						"last-rotation-initiation-time": "-100",
					},
				},
			}
			Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed())

			mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{SecretNamesToTimes: map[string]time.Time{"foo": fakeClock.Now()}})
			Expect(err).NotTo(HaveOccurred())
			m = mgr.(*manager)

			Expect(m.lastRotationInitiationTimes).To(Equal(nameToUnixTime{
				"foo":     "-62135596800",
				"secret1": "-100",
			}))
		})

		It("should create a new instance and auto-renew a secret which is about to expire (at least 80% validity reached)", func() {
			fakeClock = testclock.NewFakeClock(time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC))

			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret1",
					Namespace: namespace,
					Labels: map[string]string{
						"name":                          "secret1",
						"managed-by":                    "secrets-manager",
						"manager-identity":              identity,
						"last-rotation-initiation-time": "-100",
						"issued-at-time":                strconv.FormatInt(fakeClock.Now().Add(-24*time.Hour).Unix(), 10),
						"valid-until-time":              strconv.FormatInt(fakeClock.Now().Add(time.Hour).Unix(), 10),
					},
				},
			}
			Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed())

			mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{})
			Expect(err).NotTo(HaveOccurred())
			m = mgr.(*manager)

			Expect(m.lastRotationInitiationTimes).To(Equal(nameToUnixTime{"secret1": strconv.FormatInt(fakeClock.Now().Unix(), 10)}))
		})

		It("should create a new instance and auto-renew a secret which is about to expire (at most 10d left until expiration)", func() {
			fakeClock = testclock.NewFakeClock(time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC))

			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret1",
					Namespace: namespace,
					Labels: map[string]string{
						"name":                          "secret1",
						"managed-by":                    "secrets-manager",
						"manager-identity":              identity,
						"last-rotation-initiation-time": "-100",
						"issued-at-time":                strconv.FormatInt(fakeClock.Now().Add(-24*time.Hour).Unix(), 10),
						"valid-until-time":              strconv.FormatInt(fakeClock.Now().Add(24*time.Hour).Unix(), 10),
					},
				},
			}
			Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed())

			mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{})
			Expect(err).NotTo(HaveOccurred())
			m = mgr.(*manager)

			Expect(m.lastRotationInitiationTimes).To(Equal(nameToUnixTime{"secret1": strconv.FormatInt(fakeClock.Now().Unix(), 10)}))
		})

		It("should create a new instance and NOT auto-renew a secret since it's still valid for a longer time", func() {
			fakeClock = testclock.NewFakeClock(time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC))

			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret1",
					Namespace: namespace,
					Labels: map[string]string{
						"name":                          "secret1",
						"managed-by":                    "secrets-manager",
						"manager-identity":              identity,
						"last-rotation-initiation-time": "-100",
						"issued-at-time":                strconv.FormatInt(fakeClock.Now().Add(-24*time.Hour).Unix(), 10),
						"valid-until-time":              strconv.FormatInt(fakeClock.Now().Add(15*365*24*time.Hour).Unix(), 10),
					},
				},
			}
			Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed())

			mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{})
			Expect(err).NotTo(HaveOccurred())
			m = mgr.(*manager)

			Expect(m.lastRotationInitiationTimes).To(Equal(nameToUnixTime{"secret1": "-100"}))
		})

		It("should only consider the last rotation initiation time for the newest secret", func() {
			secrets := []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "secret1-1",
						Namespace:         namespace,
						CreationTimestamp: metav1.Time{Time: time.Date(2000, 1, 3, 1, 1, 1, 1, time.UTC)},
						Labels: map[string]string{
							"name":                          "secret1",
							"managed-by":                    "secrets-manager",
							"manager-identity":              identity,
							"last-rotation-initiation-time": "24",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "secret1-2",
						Namespace:         namespace,
						CreationTimestamp: metav1.Time{Time: time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC)},
						Labels: map[string]string{
							"name":                          "secret1",
							"managed-by":                    "secrets-manager",
							"manager-identity":              identity,
							"last-rotation-initiation-time": "12",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "secret1-3",
						Namespace:         namespace,
						CreationTimestamp: metav1.Time{Time: time.Date(2000, 1, 2, 1, 1, 1, 1, time.UTC)},
						Labels: map[string]string{
							"name":                          "secret1",
							"managed-by":                    "secrets-manager",
							"manager-identity":              identity,
							"last-rotation-initiation-time": "16",
						},
					},
				},
			}

			for _, secret := range secrets {
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())
			}

			mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{})
			Expect(err).NotTo(HaveOccurred())
			m = mgr.(*manager)

			Expect(m.lastRotationInitiationTimes).To(Equal(nameToUnixTime{"secret1": "24"}))
		})

		Context("ca secrets which are about to expire (at most 10d left until expiration)", func() {
			var existingSecret *corev1.Secret

			BeforeEach(func() {
				fakeClock = testclock.NewFakeClock(time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC))

				existingSecret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret1",
						Namespace: namespace,
						Labels: map[string]string{
							"name":                          "secret1",
							"managed-by":                    "secrets-manager",
							"manager-identity":              identity,
							"last-rotation-initiation-time": "-100",
							"issued-at-time":                strconv.FormatInt(fakeClock.Now().Add(-24*time.Hour).Unix(), 10),
							"valid-until-time":              strconv.FormatInt(fakeClock.Now().Add(24*time.Hour).Unix(), 10),
						},
					},
					Data: map[string][]byte{
						"ca.crt": []byte("foo"),
						"ca.key": []byte("foo"),
					},
				}
				Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed())
			})

			It("should create a new instance and auto-renew the CA secret because CASecretAutoRotation=true", func() {
				fakeClock = testclock.NewFakeClock(time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC))

				mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{CASecretAutoRotation: true})
				Expect(err).NotTo(HaveOccurred())
				m = mgr.(*manager)

				Expect(m.lastRotationInitiationTimes).To(Equal(nameToUnixTime{"secret1": strconv.FormatInt(fakeClock.Now().Unix(), 10)}))
			})

			It("should create a new instance and NOT auto-renew the CA secret because CASecretAutoRotation=false", func() {
				mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{CASecretAutoRotation: false})
				Expect(err).NotTo(HaveOccurred())
				m = mgr.(*manager)

				Expect(m.lastRotationInitiationTimes).To(Equal(nameToUnixTime{"secret1": "-100"}))
			})
		})
	})

	Describe("#ObjectMeta", func() {
		var (
			configName                 = "config-name"
			namespace                  = "some-namespace"
			lastRotationInitiationTime = "1646060228"
		)

		DescribeTable("check different label options",
			func(ignoreChecksum bool, expectedName string, lastRotationInitiationTime string) {
				config := &secretsutils.CertificateSecretConfig{Name: configName}

				meta, err := ObjectMeta(namespace, "test", config, ignoreChecksum, lastRotationInitiationTime, nil, nil, nil)
				Expect(err).NotTo(HaveOccurred())

				Expect(meta).To(Equal(metav1.ObjectMeta{
					Name:      expectedName,
					Namespace: namespace,
					Labels: map[string]string{
						"name":                          configName,
						"managed-by":                    "secrets-manager",
						"manager-identity":              "test",
						"checksum-of-config":            "1645436262831067767",
						"last-rotation-initiation-time": lastRotationInitiationTime,
					},
				}))
			},

			Entry("config checksum ignored, no rotation", true, configName, ""),
			Entry("config checksum ignored, rotation", true, configName+"-76711", lastRotationInitiationTime),
			Entry("config checksum considered, no rotation", false, configName+"-fd0a3f24", ""),
			Entry("config checksum considered, rotation", false, configName+"-fd0a3f24-76711", lastRotationInitiationTime),
		)

		DescribeTable("check different label options",
			func(nameInfix string, signingCAChecksum *string, persist *bool, bundleFor *string, extraLabels map[string]string) {
				config := &secretsutils.CertificateSecretConfig{
					Name:      configName,
					SigningCA: &secretsutils.Certificate{},
				}

				meta, err := ObjectMeta(namespace, "test", config, false, lastRotationInitiationTime, signingCAChecksum, persist, bundleFor)
				Expect(err).NotTo(HaveOccurred())

				labels := map[string]string{
					"name":                          configName,
					"managed-by":                    "secrets-manager",
					"manager-identity":              "test",
					"checksum-of-config":            "17861245496710117091",
					"last-rotation-initiation-time": "1646060228",
				}

				Expect(meta).To(Equal(metav1.ObjectMeta{
					Name:      configName + "-" + nameInfix + "-76711",
					Namespace: namespace,
					Labels:    utils.MergeStringMaps(labels, extraLabels),
				}))
			},

			Entry("no extras", "a9c2fcb9", nil, nil, nil, nil),
			Entry("with signing ca checksum", "a11a0b2d", ptr.To("checksum"), nil, nil, map[string]string{"checksum-of-signing-ca": "checksum"}),
			Entry("with persist", "a9c2fcb9", nil, ptr.To(true), nil, map[string]string{"persist": "true"}),
			Entry("with bundleFor", "a9c2fcb9", nil, nil, ptr.To("bundle-origin"), map[string]string{"bundle-for": "bundle-origin"}),
		)
	})

	DescribeTable("#Secret",
		func(data map[string][]byte, expectedType corev1.SecretType) {
			objectMeta := metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			}

			Expect(Secret(objectMeta, data)).To(Equal(&corev1.Secret{
				ObjectMeta: objectMeta,
				Data:       data,
				Type:       expectedType,
				Immutable:  ptr.To(true),
			}))
		},

		Entry("regular secret", map[string][]byte{"some": []byte("data")}, corev1.SecretTypeOpaque),
		Entry("tls secret", map[string][]byte{"tls.key": []byte("key"), "tls.crt": []byte("cert")}, corev1.SecretTypeTLS),
	)
})
