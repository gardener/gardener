// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx             = context.TODO()
		gardenNamespace = "garden"

		runtimeClient       client.Client
		virtualGardenClient client.Client
		sm                  secretsmanager.Interface
		reconciler          *Reconciler
	)

	BeforeEach(func() {
		runtimeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		virtualGardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		sm = fakesecretsmanager.New(runtimeClient, gardenNamespace)

		reconciler = &Reconciler{
			RuntimeClientSet: fakekubernetes.NewClientSetBuilder().WithClient(runtimeClient).Build(),
			GardenNamespace:  gardenNamespace,
		}
	})

	Describe("#generateGlobalObservabilityIngressPassword", func() {
		It("should generate the secret in the runtime cluster when none exists", func() {
			secret, err := reconciler.generateGlobalObservabilityIngressPassword(ctx, sm)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret).NotTo(BeNil())
			Expect(secret.Name).To(HavePrefix("global-" + v1beta1constants.SecretNameObservabilityIngress + "-"))
			Expect(secret.Data).To(And(
				HaveKeyWithValue("username", []byte("admin")),
				HaveKey("password"),
			))
		})

		It("should return the human-managed secret without generating a new one", func() {
			humanSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom-monitoring-secret",
					Namespace: gardenNamespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleGlobalMonitoring,
					},
				},
				Data: map[string][]byte{
					"username": []byte("custom-user"),
					"password": []byte("custom-pass"),
				},
			}
			Expect(runtimeClient.Create(ctx, humanSecret)).To(Succeed())

			secret, err := reconciler.generateGlobalObservabilityIngressPassword(ctx, sm)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Name).To(Equal("custom-monitoring-secret"))
			Expect(secret.Data).To(And(
				HaveKeyWithValue("username", []byte("custom-user")),
				HaveKeyWithValue("password", []byte("custom-pass")),
			))

			secretList := &corev1.SecretList{}
			Expect(runtimeClient.List(ctx, secretList,
				client.InNamespace(gardenNamespace),
				client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleGlobalMonitoring},
			)).To(Succeed())
			Expect(secretList.Items).To(HaveLen(1))

			// Assert the human-managed secret is not deleted.
			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(humanSecret), &corev1.Secret{})).To(Succeed())
		})

		It("should regenerate when only secrets-manager-managed secrets exist and delete the old one", func() {
			oldSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "global-observability-ingress-abc12345",
					Namespace: gardenNamespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole:      v1beta1constants.GardenRoleGlobalMonitoring,
						secretsmanager.LabelKeyManagedBy: secretsmanager.LabelValueSecretsManager,
					},
				},
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("existing-pass"),
				},
			}
			Expect(runtimeClient.Create(ctx, oldSecret)).To(Succeed())

			secret, err := reconciler.generateGlobalObservabilityIngressPassword(ctx, sm)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret).NotTo(BeNil())
			Expect(secret.Name).To(HavePrefix("global-" + v1beta1constants.SecretNameObservabilityIngress + "-"))

			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(oldSecret), &corev1.Secret{})).To(MatchError(ContainSubstring("not found")))
		})

		It("should fail when more than one global observability secret exists", func() {
			for _, name := range []string{"secret-1", "secret-2"} {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: gardenNamespace,
						Labels: map[string]string{
							v1beta1constants.GardenRole: v1beta1constants.GardenRoleGlobalMonitoring,
						},
					},
				}
				Expect(runtimeClient.Create(ctx, secret)).To(Succeed())
			}

			_, err := reconciler.generateGlobalObservabilityIngressPassword(ctx, sm)
			Expect(err).To(MatchError(ContainSubstring("there can be at most one global observability secret but found multiple")))
		})

		It("should ignore seed replicas when the runtime cluster is also a seed", func() {
			seedReplica := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "seed-observability-ingress-abc12345",
					Namespace: gardenNamespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole:      v1beta1constants.GardenRoleGlobalMonitoring,
						v1beta1constants.GardenerPurpose: gardenerutils.LabelPurposeGlobalMonitoringSecret,
					},
				},
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("seed-pass"),
				},
			}
			Expect(runtimeClient.Create(ctx, seedReplica)).To(Succeed())

			secret, err := reconciler.generateGlobalObservabilityIngressPassword(ctx, sm)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret).NotTo(BeNil())
			Expect(secret.Name).To(HavePrefix("global-" + v1beta1constants.SecretNameObservabilityIngress + "-"))

			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(seedReplica), &corev1.Secret{})).To(Succeed())
		})
	})

	Describe("#prepareGlobalMonitoringSecretMigration", func() {
		It("should fail when more than one virtual garden secret exists", func() {
			for _, name := range []string{"secret-1", "secret-2"} {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: gardenNamespace,
						Labels: map[string]string{
							v1beta1constants.GardenRole: v1beta1constants.GardenRoleGlobalMonitoring,
						},
					},
				}
				Expect(virtualGardenClient.Create(ctx, secret)).To(Succeed())
			}

			Expect(reconciler.prepareGlobalMonitoringSecretMigration(ctx, virtualGardenClient)).To(MatchError(ContainSubstring("more than one")))
		})

		It("should create migration secret and add purpose label to virtual garden secret for secrets-manager-managed secrets", func() {
			virtualSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "observability-ingress-abc123",
					Namespace: gardenNamespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole:      v1beta1constants.GardenRoleGlobalMonitoring,
						secretsmanager.LabelKeyManagedBy: secretsmanager.LabelValueSecretsManager,
					},
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("secret-pass"),
				},
			}
			Expect(virtualGardenClient.Create(ctx, virtualSecret)).To(Succeed())

			Expect(reconciler.prepareGlobalMonitoringSecretMigration(ctx, virtualGardenClient)).To(Succeed())

			Expect(virtualGardenClient.Get(ctx, client.ObjectKeyFromObject(virtualSecret), virtualSecret)).To(Succeed())
			Expect(virtualSecret.Labels).To(HaveKeyWithValue(v1beta1constants.GardenerPurpose, gardenerutils.LabelPurposeGlobalMonitoringSecret))

			migrationSecret := &corev1.Secret{}
			Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: "migrate-global-observability-ingress", Namespace: gardenNamespace}, migrationSecret)).To(Succeed())
			Expect(migrationSecret.Labels).To(HaveKeyWithValue(secretsmanager.LabelKeyUseDataForName, "global-observability-ingress"))
			Expect(migrationSecret.Data).To(And(
				HaveKeyWithValue("username", []byte("admin")),
				HaveKeyWithValue("password", []byte("secret-pass")),
			))
		})

		It("should create runtime secret and add purpose label for human-managed secrets", func() {
			virtualSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom-monitoring",
					Namespace: gardenNamespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleGlobalMonitoring,
						"custom-label":              "custom-value",
					},
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("secret-pass"),
				},
			}
			Expect(virtualGardenClient.Create(ctx, virtualSecret)).To(Succeed())

			Expect(reconciler.prepareGlobalMonitoringSecretMigration(ctx, virtualGardenClient)).To(Succeed())

			runtimeSecret := &corev1.Secret{}
			Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: "custom-monitoring", Namespace: gardenNamespace}, runtimeSecret)).To(Succeed())
			Expect(runtimeSecret.Labels).To(HaveKeyWithValue(v1beta1constants.GardenRole, v1beta1constants.GardenRoleGlobalMonitoring))
			Expect(runtimeSecret.Labels).To(HaveKeyWithValue("custom-label", "custom-value"))
			Expect(runtimeSecret.Labels).NotTo(HaveKey(v1beta1constants.GardenerPurpose))
			Expect(runtimeSecret.Data).To(And(
				HaveKeyWithValue("username", []byte("admin")),
				HaveKeyWithValue("password", []byte("secret-pass")),
				HaveKey("auth"),
			))

			Expect(virtualGardenClient.Get(ctx, client.ObjectKeyFromObject(virtualSecret), virtualSecret)).To(Succeed())
			Expect(virtualSecret.Labels).To(HaveKeyWithValue(v1beta1constants.GardenerPurpose, gardenerutils.LabelPurposeGlobalMonitoringSecret))
		})
	})

	Describe("#finalizeGlobalMonitoringSecretMigration", func() {
		It("should delete the migration secret from runtime cluster", func() {
			migrationSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "migrate-global-observability-ingress",
					Namespace: gardenNamespace,
					Labels: map[string]string{
						secretsmanager.LabelKeyUseDataForName: "global-observability-ingress",
					},
				},
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("secret-pass"),
				},
			}
			Expect(runtimeClient.Create(ctx, migrationSecret)).To(Succeed())

			Expect(reconciler.finalizeGlobalMonitoringSecretMigration(ctx)).To(Succeed())

			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(migrationSecret), &corev1.Secret{})).To(MatchError(ContainSubstring("not found")))
		})

		It("should delete runtime secrets with purpose label but not role label", func() {
			for _, name := range []string{"stale-replica-1", "stale-replica-2"} {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: gardenNamespace,
						Labels: map[string]string{
							v1beta1constants.GardenerPurpose: gardenerutils.LabelPurposeGlobalMonitoringSecret,
						},
					},
				}
				Expect(runtimeClient.Create(ctx, secret)).To(Succeed())
			}

			Expect(reconciler.finalizeGlobalMonitoringSecretMigration(ctx)).To(Succeed())

			secretList := &corev1.SecretList{}
			Expect(runtimeClient.List(ctx, secretList,
				client.InNamespace(gardenNamespace),
				client.MatchingLabels{v1beta1constants.GardenerPurpose: gardenerutils.LabelPurposeGlobalMonitoringSecret},
			)).To(Succeed())
			Expect(secretList.Items).To(BeEmpty())
		})

		It("should ignore seed replicas when the runtime cluster is also a seed", func() {
			seedReplica := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "seed-observability-ingress-abc12345",
					Namespace: gardenNamespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole:      v1beta1constants.GardenRoleGlobalMonitoring,
						v1beta1constants.GardenerPurpose: gardenerutils.LabelPurposeGlobalMonitoringSecret,
					},
				},
			}
			Expect(runtimeClient.Create(ctx, seedReplica)).To(Succeed())

			staleReplica := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "stale-replica",
					Namespace: gardenNamespace,
					Labels: map[string]string{
						v1beta1constants.GardenerPurpose: gardenerutils.LabelPurposeGlobalMonitoringSecret,
					},
				},
			}
			Expect(runtimeClient.Create(ctx, staleReplica)).To(Succeed())

			Expect(reconciler.finalizeGlobalMonitoringSecretMigration(ctx)).To(Succeed())

			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(seedReplica), &corev1.Secret{})).To(Succeed())
			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(staleReplica), &corev1.Secret{})).To(MatchError(ContainSubstring("not found")))
		})
	})
})
