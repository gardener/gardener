// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package applications

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"
)

const (
	// GuestBook is the name of the guestbook k8s resources
	GuestBook = "guestbook"

	// RedisMaster is the name of the redis master deployed by the helm chart
	RedisMaster = "redis-master"
)

var (
	// Copied from https://github.com/helm/charts/tree/cb5f95d7453432e0ecd7adc60ea5965cf90adc28/stable/redis/templates
	//go:embed charts/redis
	chartRedis     embed.FS
	chartPathRedis = filepath.Join("charts", "redis")
)

// GuestBookTest is simple application tests.
// It deploys a guestbook application with a redis backend and a frontend
// that is exposed via an ingress.
type GuestBookTest struct {
	framework *framework.ShootFramework

	guestBookAppHost string
}

// NewGuestBookTest creates a new guestbook application test
// This test should run inside a testframework with a registered shoot test because otherwise created resources may leak.
func NewGuestBookTest(f *framework.ShootFramework) *GuestBookTest {
	return &GuestBookTest{
		framework: f,
	}
}

// WaitUntilRedisIsReady waits until the redis master is ready.
func (t *GuestBookTest) WaitUntilRedisIsReady(ctx context.Context) {
	Expect(t.framework.WaitUntilStatefulSetIsRunning(ctx, RedisMaster, t.framework.Namespace, t.framework.ShootClient)).To(Succeed())
}

// WaitUntilGuestbookDeploymentIsReady waits until the guestbook deployment is ready.
func (t *GuestBookTest) WaitUntilGuestbookDeploymentIsReady(ctx context.Context) {
	Expect(t.framework.WaitUntilDeploymentIsReady(ctx, GuestBook, t.framework.Namespace, t.framework.ShootClient)).To(Succeed())
}

// WaitUntilGuestbookIngressIsReady waits until the guestbook ingress is ready.
func (t *GuestBookTest) WaitUntilGuestbookIngressIsReady(ctx context.Context) {
	Expect(t.framework.WaitUntilIngressIsReady(ctx, GuestBook, t.framework.Namespace, t.framework.ShootClient)).To(Succeed())
}

// WaitUntilGuestbookLoadBalancerIsReady waits until the guestbook Service of type=LoadBalancer is ready.
func (t *GuestBookTest) WaitUntilGuestbookLoadBalancerIsReady(ctx context.Context) string {
	loadBalancerIngress, err := kubernetesutils.WaitUntilLoadBalancerIsReady(ctx, t.framework.Logger, t.framework.ShootClient.Client(), t.framework.Namespace, GuestBook, 10*time.Minute)
	Expect(err).NotTo(HaveOccurred())

	return loadBalancerIngress
}

// WaitUntilGuestbookURLsRespondOK waits until the deployed guestbook application can be reached via http
func (t *GuestBookTest) WaitUntilGuestbookURLsRespondOK(ctx context.Context, guestbookAppUrls []string) error {
	defaultPollInterval := time.Minute
	return retry.UntilTimeout(ctx, defaultPollInterval, 20*time.Minute, func(ctx context.Context) (done bool, err error) {
		for _, guestbookAppURL := range guestbookAppUrls {
			response, err := framework.HTTPGet(ctx, guestbookAppURL)
			if err != nil {
				t.framework.Logger.Info("Guestbook app is not available yet (call failed)", "url", guestbookAppURL, "reason", err.Error())
				return retry.MinorError(err)
			}

			if response.StatusCode != http.StatusOK {
				t.framework.Logger.Info("Guestbook app is not available yet (unexpected response)", "url", guestbookAppURL, "statusCode", response.StatusCode)
				return retry.MinorError(fmt.Errorf("guestbook app url %q returned status %s", guestbookAppURL, response.Status))
			}

			responseBytes, err := io.ReadAll(response.Body)
			if err != nil {
				return retry.SevereError(err)
			}

			bodyString := string(responseBytes)
			if strings.Contains(bodyString, "404") || strings.Contains(bodyString, "503") {
				t.framework.Logger.Info("Guestbook app is not ready yet")
				return retry.MinorError(errors.New("guestbook response body contained an error code"))
			}
		}
		t.framework.Logger.Info("Rejoice, the guestbook app urls are available now")
		return retry.Ok()
	})
}

// DeployGuestBookApp deploys the redis helmchart and the guestbook app application
func (t *GuestBookTest) DeployGuestBookApp(ctx context.Context) {
	if t.framework.Namespace == "" {
		_, err := t.framework.CreateNewNamespace(ctx)
		Expect(err).NotTo(HaveOccurred())
	}

	shoot := t.framework.Shoot
	k8sVersion, err := semver.NewVersion(shoot.Spec.Kubernetes.Version)
	Expect(err).NotTo(HaveOccurred())

	if versionutils.ConstraintK8sLess135.Check(k8sVersion) && !v1beta1helper.NginxIngressEnabled(shoot.Spec.Addons) {
		Fail("The test requires .spec.addons.nginxIngress.enabled to be true for Kubernetes versions less than 1.35")
	}

	By("Apply redis chart")
	masterValues := map[string]any{
		"command": "redis-server",
	}
	if shoot.Spec.Provider.Type == "alicloud" {
		// AliCloud requires a minimum of 20 GB for its PVCs
		masterValues["persistence"] = map[string]any{
			"size": "20Gi",
		}
	}

	hasARMWorkerPools := slices.ContainsFunc(shoot.Spec.Provider.Workers, func(worker gardencorev1beta1.Worker) bool {
		return worker.Machine.Architecture != nil && *worker.Machine.Architecture == v1beta1constants.ArchitectureARM64
	})

	// GCP requires a specific storage class for ARM worker pools
	if shoot.Spec.Provider.Type == "gcp" && hasARMWorkerPools {
		Expect(t.framework.ShootClient.Client().Create(ctx, &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gce-hd-balanced",
				Annotations: map[string]string{
					"resources.gardener.cloud/delete-on-invalid-update": "true",
				},
			},
			AllowVolumeExpansion: ptr.To(true),
			Provisioner:          "pd.csi.storage.gke.io",
			Parameters: map[string]string{
				"type": "hyperdisk-balanced",
			},
			VolumeBindingMode: ptr.To(storagev1.VolumeBindingWaitForFirstConsumer),
		})).To(Succeed())

		masterValues["persistence"] = map[string]any{
			"storageClass": "gce-hd-balanced",
		}
	}

	// redis-slaves are not required for test success
	values := map[string]any{
		"image": map[string]any{
			"registry":   "europe-docker.pkg.dev",
			"repository": "gardener-project/releases/3rd/redis",
			"tag":        "5.0.8",
		},
		"cluster": map[string]any{
			"enabled": false,
		},
		"rbac": map[string]any{
			"create": true,
		},
		"master": masterValues,
	}

	Expect(t.framework.ShootClient.ChartApplier().ApplyFromEmbeddedFS(ctx, chartRedis, chartPathRedis, t.framework.Namespace, "redis", kubernetes.Values(values), kubernetes.ForceNamespace)).To(Succeed())

	t.WaitUntilRedisIsReady(ctx)

	By("Deploy the guestbook application")
	guestBookValues := map[string]any{
		"HelmDeployNamespace": t.framework.Namespace,
		"KubeVersion":         shoot.Spec.Kubernetes.Version,
	}
	// TODO(ialidzhikov): Clean up the guestbook test not to use use Ingress when Kubernetes 1.34 is no longer supported.
	if versionutils.ConstraintK8sLess135.Check(k8sVersion) {
		allowedCharacters := "0123456789abcdefghijklmnopqrstuvwxyz"
		randURLSuffix, err := utils.GenerateRandomStringFromCharset(3, allowedCharacters)
		Expect(err).NotTo(HaveOccurred())

		t.guestBookAppHost = fmt.Sprintf("guestbook-%s.ingress.%s", randURLSuffix, *shoot.Spec.DNS.Domain)

		guestBookValues["ShootDNSHost"] = t.guestBookAppHost
	}

	Expect(t.framework.RenderAndDeployTemplate(ctx, t.framework.ShootClient, templates.GuestbookAppName, guestBookValues)).To(Succeed())

	t.WaitUntilGuestbookDeploymentIsReady(ctx)

	if versionutils.ConstraintK8sLess135.Check(k8sVersion) {
		t.WaitUntilGuestbookIngressIsReady(ctx)
	} else {
		loadBalancerIngress := t.WaitUntilGuestbookLoadBalancerIsReady(ctx)
		t.guestBookAppHost = loadBalancerIngress
	}

	By("Guestbook app was deployed successfully!")
}

// Test tests that a deployed guestbook application is working correctly
func (t *GuestBookTest) Test(ctx context.Context) {
	shoot := t.framework.Shoot
	// define guestbook app urls
	guestBookAppURL := "http://" + t.guestBookAppHost
	pushString := "foobar-" + shoot.Name
	pushURL := fmt.Sprintf("%s/rpush/guestbook/%s", guestBookAppURL, pushString)
	pullURL := fmt.Sprintf("%s/lrange/guestbook", guestBookAppURL)

	// Check availability of the guestbook app
	Expect(t.WaitUntilGuestbookURLsRespondOK(ctx, []string{guestBookAppURL, pushURL, pullURL})).To(Succeed())

	// Push foobar-<shoot-name> to the guestbook app
	_, err := framework.HTTPGet(ctx, pushURL)
	Expect(err).NotTo(HaveOccurred())

	// Pull foobar
	pullResponse, err := framework.HTTPGet(ctx, pullURL)
	Expect(err).NotTo(HaveOccurred())
	Expect(pullResponse.StatusCode).To(Equal(http.StatusOK))

	responseBytes, err := io.ReadAll(pullResponse.Body)
	Expect(err).NotTo(HaveOccurred())

	// test if foobar-<shoot-name> was pulled successfully
	bodyString := string(responseBytes)
	Expect(bodyString).To(ContainSubstring(fmt.Sprintf("foobar-%s", shoot.Name)))
}

// Dump logs the current state of all components of the guestbook test
// if the test has failed
func (t *GuestBookTest) dump(ctx context.Context) {
	if !CurrentSpecReport().Failed() {
		return
	}

	if err := t.framework.DumpDefaultResourcesInNamespace(ctx, t.framework.ShootClient, t.framework.Namespace); err != nil {
		t.framework.Logger.Error(err, "Unable to dump guestbook resources in namespace", "namespace", t.framework.Namespace)
	}

	labels := client.MatchingLabels{"app": "nginx-ingress", "component": "controller", "origin": "gardener"}
	if err := t.framework.DumpLogsForPodsWithLabelsInNamespace(ctx, t.framework.ShootClient, "kube-system", labels); err != nil {
		t.framework.Logger.Error(err, "Unable to dump nginx logs from pods with labels in kube-system namespace", "labels", labels)
	}
}

// Cleanup cleans up all resources deployed by the guestbook test
func (t *GuestBookTest) Cleanup(ctx context.Context) {
	// First dump all resources if the test has failed
	t.dump(ctx)

	// Clean up shoot
	By("Clean up guestbook app resources")

	Expect(kubernetesutils.DeleteObjects(ctx, t.framework.ShootClient.Client(),
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Namespace: t.framework.Namespace, Name: GuestBook}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: t.framework.Namespace, Name: GuestBook}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: t.framework.Namespace, Name: GuestBook}},
	)).To(Succeed())

	Expect(kubernetesutils.DeleteObjects(ctx, t.framework.ShootClient.Client(),
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: t.framework.Namespace, Name: RedisMaster}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: t.framework.Namespace, Name: RedisMaster}},
	)).To(Succeed())

	By("Redis and the guestbook app have been cleaned up!")
}
