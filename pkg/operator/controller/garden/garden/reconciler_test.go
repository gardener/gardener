// SPDX-FileCopyrightText: Contributors to the Gardener project
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
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/features"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx             = context.TODO()
		gardenNamespace = "garden"

		runtimeClient client.Client
		sm            secretsmanager.Interface
		reconciler    *Reconciler
	)

	BeforeEach(func() {
		runtimeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
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

	Describe("#isLoggingEnabled", func() {
		It("should return true when logging.enabled is not set", func() {
			settings := &operatorv1alpha1.Logging{}
			Expect(isLoggingEnabled(settings)).To(BeTrue())
		})

		It("should return true when logging is enabled", func() {
			settings := &operatorv1alpha1.Logging{Enabled: new(true)}
			Expect(isLoggingEnabled(settings)).To(BeTrue())
		})

		It("should return false when logging is disabled", func() {
			settings := &operatorv1alpha1.Logging{Enabled: new(false)}
			Expect(isLoggingEnabled(settings)).To(BeFalse())
		})
	})

	Describe("#isValiEnabled", func() {
		It("should return true when vali.enabled is not set", func() {
			settings := &operatorv1alpha1.Logging{}
			Expect(isValiEnabled(settings)).To(BeTrue())
		})

		It("should return true when vali is enabled", func() {
			settings := &operatorv1alpha1.Logging{Vali: &operatorv1alpha1.Vali{Enabled: new(true)}}
			Expect(isValiEnabled(settings)).To(BeTrue())
		})

		It("should return false when vali is disabled", func() {
			settings := &operatorv1alpha1.Logging{Vali: &operatorv1alpha1.Vali{Enabled: new(false)}}
			Expect(isValiEnabled(settings)).To(BeFalse())
		})

		It("should return false when logging is disabled", func() {
			settings := &operatorv1alpha1.Logging{
				Enabled: new(false),
				Vali:    &operatorv1alpha1.Vali{Enabled: new(true)},
			}
			Expect(isValiEnabled(settings)).To(BeFalse())
		})

		It("should return false when the RemoveVali and VictoriaLogsBackend feature gates are enabled", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.VictoriaLogsBackend, true))
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.RemoveVali, true))

			settings := &operatorv1alpha1.Logging{Vali: &operatorv1alpha1.Vali{Enabled: new(true)}}
			Expect(isValiEnabled(settings)).To(BeFalse())
		})

		It("should return true when only the RemoveVali feature gate is enabled", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.VictoriaLogsBackend, false))
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.RemoveVali, true))

			settings := &operatorv1alpha1.Logging{Vali: &operatorv1alpha1.Vali{Enabled: new(true)}}
			Expect(isValiEnabled(settings)).To(BeTrue())
		})
	})

	Describe("#isVictoriaLogsEnabled", func() {
		It("should return false when the VictoriaLogsBackend feature gate is disabled", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.VictoriaLogsBackend, false))

			Expect(isVictoriaLogsEnabled(nil)).To(BeFalse())
			Expect(isVictoriaLogsEnabled(&operatorv1alpha1.Logging{})).To(BeFalse())
		})

		It("should return true when the VictoriaLogsBackend feature gate is enabled and victoriaLogs.enabled is not set", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.VictoriaLogsBackend, true))

			settings := &operatorv1alpha1.Logging{}
			Expect(isVictoriaLogsEnabled(settings)).To(BeTrue())
		})

		It("should return false when logging is disabled", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.VictoriaLogsBackend, true))

			settings := &operatorv1alpha1.Logging{
				Enabled:      new(false),
				VictoriaLogs: &operatorv1alpha1.VictoriaLogs{Enabled: new(true)},
			}
			Expect(isVictoriaLogsEnabled(settings)).To(BeFalse())
		})

		It("should return false when victoriaLogs is disabled", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.VictoriaLogsBackend, true))

			settings := &operatorv1alpha1.Logging{VictoriaLogs: &operatorv1alpha1.VictoriaLogs{Enabled: new(false)}}
			Expect(isVictoriaLogsEnabled(settings)).To(BeFalse())
		})

		It("should return true when victoriaLogs is enabled", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.VictoriaLogsBackend, true))

			settings := &operatorv1alpha1.Logging{VictoriaLogs: &operatorv1alpha1.VictoriaLogs{Enabled: new(true)}}
			Expect(isVictoriaLogsEnabled(settings)).To(BeTrue())
		})
	})
})
