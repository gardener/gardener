// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	victoriametricsv1 "github.com/VictoriaMetrics/operator/api/operator/v1"
	victoriametricsv1beta1 "github.com/VictoriaMetrics/operator/api/operator/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	otelcollectorcomponent "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/opentelemetrycollector"
	otelcollectorconstants "github.com/gardener/gardener/pkg/component/observability/opentelemetry/collector/constants"
	"github.com/gardener/gardener/pkg/provider-local/controller/infrastructure"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/utils/logging"
)

// ItShouldHaveShootVictoriaLogsReady waits for the VLSingle CR in the shoot's control plane namespace to reach operational status.
func ItShouldHaveShootVictoriaLogsReady(tc *ShootContext) {
	GinkgoHelper()

	It("Wait for VictoriaLogs to be ready", func(ctx SpecContext) {
		vl := &victoriametricsv1.VLSingle{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.SeedClient.Get(ctx, types.NamespacedName{Name: "victoria-logs", Namespace: tc.ControlPlaneNamespace}, vl)).To(Succeed())
			g.Expect(vl.Status.UpdateStatus).To(Equal(victoriametricsv1beta1.UpdateStatusOperational))
		}).Should(Succeed())
	}, SpecTimeout(5*time.Minute))
}

// ItShouldHaveGardenVictoriaLogsReady waits for the VLSingle CR in the garden namespace to reach operational status.
func ItShouldHaveGardenVictoriaLogsReady(tc *SeedContext) {
	GinkgoHelper()

	It("Wait for garden VictoriaLogs to be ready", func(ctx SpecContext) {
		vl := &victoriametricsv1.VLSingle{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.SeedClient.Get(ctx, types.NamespacedName{Name: "victoria-logs", Namespace: "garden"}, vl)).To(Succeed())
			g.Expect(vl.Status.UpdateStatus).To(Equal(victoriametricsv1beta1.UpdateStatusOperational))
		}).Should(Succeed())
	}, SpecTimeout(5*time.Minute))
}

// ItShouldWaitForLogsCountByKeyToBeInShootVictoriaLogs waits for a specific number of logs matching key=~value to be present in shoot VictoriaLogs.
// The lookback is relative to when the step starts; use zero to require newly generated logs.
func ItShouldWaitForLogsCountByKeyToBeInShootVictoriaLogs(tc *ShootContext, key, value string, lookback time.Duration, expectedLogCount int) {
	GinkgoHelper()

	It("Wait for exact number of logs by key to appear in shoot VictoriaLogs", func(ctx SpecContext) {
		since := time.Now().UTC().Add(-lookback)

		Eventually(ctx, func() error {
			return checkLogsCount(ctx, since, tc.ControlPlaneNamespace, key, value, expectedLogCount, tc.SeedClientSet)
		}).WithTimeout(2 * time.Minute).WithPolling(time.Second).Should(Succeed())
	}, SpecTimeout(3*time.Minute))
}

// ItShouldWaitForLogsCountByKeyToBeInGardenVictoriaLogs waits for a specific number of logs matching key=~value to be present in garden VictoriaLogs.
// The lookback is relative to when the step starts; use zero to require newly generated logs.
func ItShouldWaitForLogsCountByKeyToBeInGardenVictoriaLogs(tc *SeedContext, key, value string, lookback time.Duration, expectedLogCount int) {
	GinkgoHelper()

	It("Wait for exact number of logs by key to appear in garden VictoriaLogs", func(ctx SpecContext) {
		since := time.Now().UTC().Add(-lookback)

		Eventually(ctx, func() error {
			return checkLogsCount(ctx, since, "garden", key, value, expectedLogCount, tc.SeedClientSet)
		}).WithTimeout(2 * time.Minute).WithPolling(time.Second).Should(Succeed())
	}, SpecTimeout(3*time.Minute))
}

// ItShouldWaitForLogsByKeyToBeInShootVictoriaLogs waits for any logs matching key=~value to be present in shoot VictoriaLogs.
// The lookback is relative to when the step starts; use zero to require newly generated logs.
func ItShouldWaitForLogsByKeyToBeInShootVictoriaLogs(tc *ShootContext, key, value string, lookback time.Duration) {
	GinkgoHelper()

	It("Wait for logs by key to appear in shoot VictoriaLogs", func(ctx SpecContext) {
		since := time.Now().UTC().Add(-lookback)

		Eventually(ctx, func() error {
			return checkLogsPresent(ctx, since, tc.ControlPlaneNamespace, key, value, tc.SeedClientSet)
		}).WithTimeout(2 * time.Minute).WithPolling(time.Second).Should(Succeed())
	}, SpecTimeout(3*time.Minute))
}

// ItShouldWaitForLogsByKeyNotToBeInShootVictoriaLogs checks that logs matching key=~value are NOT present in shoot VictoriaLogs.
// The cutoff is fixed when the step starts; a zero lookback excludes older logs.
func ItShouldWaitForLogsByKeyNotToBeInShootVictoriaLogs(tc *ShootContext, key, value string, lookback time.Duration) {
	GinkgoHelper()

	It("Ensure logs do not exist in shoot VictoriaLogs", func(ctx SpecContext) {
		since := time.Now().UTC().Add(-lookback)

		Consistently(ctx, func() error {
			queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			logCount, err := logging.GetVictoriaLogsCount(queryCtx, since, tc.ControlPlaneNamespace, key, value, tc.SeedClientSet)

			// Retry failed queries without treating them as evidence that logs are absent.
			if err != nil {
				return TryAgainAfter(time.Second).Wrap(err)
			}
			if logCount > 0 {
				StopTrying(fmt.Sprintf("found unexpected logs for %s=%s: got %d", key, value, logCount)).Now()
			}
			return nil
		}, time.Minute).WithPolling(5 * time.Second).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// ItShouldWaitForLogsByKeyNotToBeInGardenVictoriaLogs checks that logs matching key=~value are NOT present in garden VictoriaLogs.
// The cutoff is fixed when the step starts; a zero lookback excludes older logs.
func ItShouldWaitForLogsByKeyNotToBeInGardenVictoriaLogs(tc *SeedContext, key, value string, lookback time.Duration) {
	GinkgoHelper()

	It("Ensure logs do not exist in garden VictoriaLogs", func(ctx SpecContext) {
		since := time.Now().UTC().Add(-lookback)

		Consistently(ctx, func() error {
			queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			logCount, err := logging.GetVictoriaLogsCount(queryCtx, since, "garden", key, value, tc.SeedClientSet)

			// Retry failed queries without treating them as evidence that logs are absent.
			if err != nil {
				return TryAgainAfter(time.Second).Wrap(err)
			}
			if logCount > 0 {
				StopTrying(fmt.Sprintf("found unexpected logs for %s=%s: got %d", key, value, logCount)).Now()
			}
			return nil
		}, time.Minute).WithPolling(5 * time.Second).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// ItShouldRejectUnauthenticatedVictoriaLogsRequestInShoot asserts that a request to the shoot VictoriaLogs ingress without credentials is rejected with HTTP 401.
func ItShouldRejectUnauthenticatedVictoriaLogsRequestInShoot(tc *ShootContext) {
	GinkgoHelper()

	It("shoot VictoriaLogs ingress should reject unauthenticated request with 401", func(ctx SpecContext) {
		status, err := logging.QueryVictoriaLogsStatus(ctx, tc.ControlPlaneNamespace, "", "", tc.SeedClientSet)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal(http.StatusUnauthorized))
	}, SpecTimeout(time.Minute))
}

// ItShouldRejectUnauthenticatedVictoriaLogsRequestInGarden asserts that a request to the garden VictoriaLogs ingress without credentials is rejected with HTTP 401.
func ItShouldRejectUnauthenticatedVictoriaLogsRequestInGarden(tc *SeedContext) {
	GinkgoHelper()

	It("garden VictoriaLogs ingress should reject unauthenticated request with 401", func(ctx SpecContext) {
		status, err := logging.QueryVictoriaLogsStatus(ctx, "garden", "", "", tc.SeedClientSet)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal(http.StatusUnauthorized))
	}, SpecTimeout(time.Minute))
}

// ItShouldRejectWrongCredentialsVictoriaLogsRequestInShoot asserts that a request to the shoot VictoriaLogs ingress with invalid credentials is rejected with HTTP 401.
func ItShouldRejectWrongCredentialsVictoriaLogsRequestInShoot(tc *ShootContext) {
	GinkgoHelper()

	It("shoot VictoriaLogs ingress should reject wrong credentials with 401", func(ctx SpecContext) {
		status, err := logging.QueryVictoriaLogsStatus(ctx, tc.ControlPlaneNamespace, "wrong-user", "wrong-password", tc.SeedClientSet)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal(http.StatusUnauthorized))
	}, SpecTimeout(time.Minute))
}

// ItShouldRejectWrongCredentialsVictoriaLogsRequestInGarden asserts that a request to the garden VictoriaLogs ingress with invalid credentials is rejected with HTTP 401.
func ItShouldRejectWrongCredentialsVictoriaLogsRequestInGarden(tc *SeedContext) {
	GinkgoHelper()

	It("garden VictoriaLogs ingress should reject wrong credentials with 401", func(ctx SpecContext) {
		status, err := logging.QueryVictoriaLogsStatus(ctx, "garden", "wrong-user", "wrong-password", tc.SeedClientSet)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal(http.StatusUnauthorized))
	}, SpecTimeout(time.Minute))
}

const (
	annotationRestartSystemdServices       = "worker.gardener.cloud/restart-systemd-services"
	invalidToken                           = "invalid-token"
	openTelemetryCollectorAccessSecretName = "shoot-access-opentelemetry-collector"
)

// ItShouldBreakOpenTelemetryCollectorToken changes the renewal time via the seed-side access Secret,
// then corrupts the token in the shoot-side target Secret.
func ItShouldBreakOpenTelemetryCollectorToken(tc *ShootContext) {
	GinkgoHelper()

	It("Break the opentelemetry-collector auth token", func(ctx SpecContext) {
		// Postpone scheduled renewal until the restore step makes it due again.
		accessSecret := &corev1.Secret{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.SeedClient.Get(ctx, types.NamespacedName{
				Namespace: tc.ControlPlaneNamespace,
				Name:      openTelemetryCollectorAccessSecretName,
			}, accessSecret)).To(Succeed())

			patch := client.MergeFrom(accessSecret.DeepCopy())
			if accessSecret.Annotations == nil {
				accessSecret.Annotations = map[string]string{}
			}

			accessSecret.Annotations[resourcesv1alpha1.ServiceAccountTokenRenewTimestamp] = "9999-01-01T00:00:00Z"
			g.Expect(tc.SeedClient.Patch(ctx, accessSecret, patch)).To(Succeed())
		}).WithPolling(time.Second).Should(Succeed())

		secret := &corev1.Secret{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.ShootClient.Get(ctx, types.NamespacedName{
				Namespace: metav1.NamespaceSystem,
				Name:      otelcollectorconstants.OpenTelemetryCollectorSecretName,
			}, secret)).To(Succeed())

			patch := client.MergeFrom(secret.DeepCopy())
			if secret.Data == nil {
				secret.Data = map[string][]byte{}
			}

			secret.Data["token"] = []byte(invalidToken)
			g.Expect(tc.ShootClient.Patch(ctx, secret, patch)).To(Succeed())
		}).WithPolling(1 * time.Second).Should(Succeed())
	}, SpecTimeout(3*time.Minute))
}

// ItShouldWaitForInvalidTokenOnNodes execs into every machine pod and waits until
// the auth-token file on disk contains the invalidToken value.
// This checks file contents, not whether the collector has reloaded the token.
func ItShouldWaitForInvalidTokenOnNodes(tc *ShootContext) {
	GinkgoHelper()

	It("Wait for invalid token to be written to nodes", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			return checkTokenOnNodes(ctx, tc, func(got string) error {
				if got != invalidToken {
					return fmt.Errorf("auth-token file contains %q, want %q", got, invalidToken)
				}
				return nil
			})
		}).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// ItShouldWaitForValidTokenOnNodes execs into every machine pod and waits until
// the auth-token file on disk contains a non-empty value different from invalidToken.
// This does not validate the token or wait for the collector to reload it.
func ItShouldWaitForValidTokenOnNodes(tc *ShootContext) {
	GinkgoHelper()

	It("Wait for valid token to be written to nodes", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			return checkTokenOnNodes(ctx, tc, func(got string) error {
				if got == "" || got == invalidToken {
					return fmt.Errorf("auth-token file still invalid (%q)", got)
				}
				return nil
			})
		}).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// ItShouldRestoreOpenTelemetryCollectorToken forces the token-requestor to re-issue a fresh
// valid token into the shoot's kube-system/gardener-opentelemetry-collector secret.
func ItShouldRestoreOpenTelemetryCollectorToken(tc *ShootContext) {
	GinkgoHelper()

	It("Restore the opentelemetry-collector auth token", func(ctx SpecContext) {
		accessSecret := &corev1.Secret{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.SeedClient.Get(ctx, types.NamespacedName{
				Namespace: tc.ControlPlaneNamespace,
				Name:      openTelemetryCollectorAccessSecretName,
			}, accessSecret)).To(Succeed())

			patch := client.MergeFrom(accessSecret.DeepCopy())
			if accessSecret.Annotations == nil {
				accessSecret.Annotations = map[string]string{}
			}

			accessSecret.Annotations[resourcesv1alpha1.ServiceAccountTokenRenewTimestamp] = "1970-01-01T00:00:00Z"
			g.Expect(tc.SeedClient.Patch(ctx, accessSecret, patch)).To(Succeed())
		}).WithPolling(time.Second).Should(Succeed())

		Eventually(ctx, func(g Gomega) {
			freshSecret := &corev1.Secret{}
			g.Expect(tc.ShootClient.Get(ctx, types.NamespacedName{
				Namespace: metav1.NamespaceSystem,
				Name:      otelcollectorconstants.OpenTelemetryCollectorSecretName,
			}, freshSecret)).To(Succeed())

			g.Expect(freshSecret.Data).To(HaveKeyWithValue("token", Not(Or(
				BeEmpty(),
				Equal([]byte(invalidToken)),
			))), "the invalid token has not yet been replaced")
		}).WithPolling(time.Second).Should(Succeed())
	}, SpecTimeout(3*time.Minute))
}

// ItShouldRestartGNA annotates every shoot node with the restart-systemd-services annotation for
// gardener-node-agent and waits until GNA has consumed the annotation.
func ItShouldRestartGNA(tc *ShootContext) {
	GinkgoHelper()

	It("Restart gardener-node-agent on shoot nodes", func(ctx SpecContext) {
		restartServiceOnNodes(ctx, tc, "gardener-node-agent")
	}, SpecTimeout(3*time.Minute))
}

// checkTokenOnNodes reads the auth-token file on every machine pod and calls check on the trimmed content.
func checkTokenOnNodes(ctx SpecContext, tc *ShootContext, check func(token string) error) error {
	infraNamespace := infrastructure.NamespaceName(tc.Shoot.Status.TechnicalID)

	podList := &corev1.PodList{}
	if err := tc.SeedClient.List(ctx, podList,
		client.InNamespace(infraNamespace),
		client.MatchingLabels{"app": "machine", "machine-provider": "local"},
	); err != nil {
		return err
	}

	if len(podList.Items) == 0 {
		return fmt.Errorf("no machine pods found in %s", infraNamespace)
	}

	for _, pod := range podList.Items {
		stdout, _, err := tc.SeedClientSet.PodExecutor().Execute(ctx, infraNamespace, pod.Name, "node",
			"cat", otelcollectorcomponent.PathAuthToken,
		)
		if err != nil {
			return fmt.Errorf("exec on pod %s failed: %w", pod.Name, err)
		}

		content, err := io.ReadAll(stdout)
		if err != nil {
			return fmt.Errorf("reading stdout from pod %s: %w", pod.Name, err)
		}

		if err := check(strings.TrimSpace(string(content))); err != nil {
			return fmt.Errorf("pod %s: %w", pod.Name, err)
		}
	}
	return nil
}

// restartServiceOnNodes sets the restart-systemd-services annotation on every shoot node for the given service and
// waits until GNA has consumed it.
func restartServiceOnNodes(ctx SpecContext, tc *ShootContext, service string) {
	GinkgoHelper()

	Eventually(ctx, func(g Gomega) {
		nodeList := &corev1.NodeList{}
		g.Expect(tc.ShootClient.List(ctx, nodeList)).To(Succeed())
		g.Expect(nodeList.Items).NotTo(BeEmpty())
		for i := range nodeList.Items {
			patch := client.MergeFrom(nodeList.Items[i].DeepCopy())
			if nodeList.Items[i].Annotations == nil {
				nodeList.Items[i].Annotations = map[string]string{}
			}

			nodeList.Items[i].Annotations[annotationRestartSystemdServices] = service
			g.Expect(tc.ShootClient.Patch(ctx, &nodeList.Items[i], patch)).To(Succeed())
		}
	}).WithPolling(time.Second).Should(Succeed())

	Eventually(ctx, func(g Gomega) {
		nodeList := &corev1.NodeList{}
		g.Expect(tc.ShootClient.List(ctx, nodeList)).To(Succeed())

		for _, node := range nodeList.Items {
			g.Expect(node.Annotations[annotationRestartSystemdServices]).To(BeEmpty(),
				"restart annotation still present on node %s", node.Name)
		}
	}).WithPolling(time.Second).Should(Succeed())
}

func checkLogsCount(ctx SpecContext, since time.Time, namespace, key, value string, expectedLogCount int, k8sClient kubernetes.Interface) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	logCount, err := logging.GetVictoriaLogsCount(queryCtx, since, namespace, key, value, k8sClient)
	if err != nil {
		return err
	}

	if logCount > expectedLogCount {
		StopTrying(fmt.Sprintf("received too many logs for %s=%s: got %d, expected %d", key, value, logCount, expectedLogCount)).Now()
	}

	if logCount < expectedLogCount {
		return fmt.Errorf("waiting for logs for %s=%s: got %d, expected %d", key, value, logCount, expectedLogCount)
	}

	return nil
}

func checkLogsPresent(ctx SpecContext, since time.Time, namespace, key, value string, k8sClient kubernetes.Interface) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	logCount, err := logging.GetVictoriaLogsCount(queryCtx, since, namespace, key, value, k8sClient)
	if err != nil {
		return err
	}

	if logCount < 1 {
		return fmt.Errorf("no logs for %s=%s since %s", key, value, since.Format(time.RFC3339Nano))
	}
	return nil
}
