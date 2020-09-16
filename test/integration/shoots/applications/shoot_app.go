// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

/**
	Overview
		- Tests the workload deployment on top of a Shoot

	AfterSuite
		- Cleanup Workload in Shoot

	Test: Create Redis Deployment
	Expected Output
		- Redis Deployment is ready

	Test: Deploy Guestbook Application
	Expected Output
		- Guestbook application should be functioning
 **/

package applications

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/version"
	"github.com/gardener/gardener/test/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/test/framework/applications"

	"github.com/onsi/ginkgo"
)

const (
	guestbookAppTimeout       = 30 * time.Minute
	finalizationTimeout       = 15 * time.Minute
	downloadKubeconfigTimeout = 600 * time.Second
	dashboardAvailableTimeout = 60 * time.Minute
)

var _ = ginkgo.Describe("Shoot application testing", func() {

	f := framework.NewShootFramework(nil)

	f.Default().Release().CIt("should download shoot kubeconfig successfully", func(ctx context.Context) {
		err := framework.DownloadKubeconfig(ctx, f.SeedClient, f.ShootSeedNamespace(), gardencorev1beta1.GardenerName, "")
		framework.ExpectNoError(err)

		ginkgo.By("Shoot Kubeconfig downloaded successfully from seed")
	}, downloadKubeconfigTimeout)

	ginkgo.Context("GuestBook", func() {
		var (
			guestBookTest *applications.GuestBookTest
			err           error
		)

		f.Default().Release().CIt("should deploy guestbook app successfully", func(ctx context.Context) {
			guestBookTest, err = applications.NewGuestBookTest(f)
			framework.ExpectNoError(err)
			guestBookTest.DeployGuestBookApp(ctx)
			guestBookTest.Test(ctx)
		}, guestbookAppTimeout)

		framework.CAfterEach(func(ctx context.Context) {
			guestBookTest.Cleanup(ctx)
		}, finalizationTimeout)
	})

	f.Default().Release().CIt("Dashboard should be available", func(ctx context.Context) {
		shoot := f.Shoot
		if !shoot.Spec.Addons.KubernetesDashboard.Enabled {
			ginkgo.Fail("The test requires .spec.addons.kubernetesDashboard.enabled to be be true")
		}
		k8sVersionLessThan116, err := version.CompareVersions(f.Shoot.Spec.Kubernetes.Version, "<", "1.16")
		framework.ExpectNoError(err)

		k8sDashboardNamespace := metav1.NamespaceSystem
		if !k8sVersionLessThan116 {
			k8sDashboardNamespace = "kubernetes-dashboard"
		}

		url := fmt.Sprintf("https://api.%s/api/v1/namespaces/%s/services/https:kubernetes-dashboard:/proxy", *f.Shoot.Spec.DNS.Domain, k8sDashboardNamespace)
		dashboardToken, err := framework.GetObjectFromSecret(ctx, f.SeedClient, f.ShootSeedNamespace(), common.KubecfgSecretName, "token")
		framework.ExpectNoError(err)

		err = framework.TestHTTPEndpointWithToken(ctx, url, dashboardToken)
		framework.ExpectNoError(err)
	}, dashboardAvailableTimeout)

})
