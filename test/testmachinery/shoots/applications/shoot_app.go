// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/applications"
)

const (
	guestbookAppTimeout       = 30 * time.Minute
	finalizationTimeout       = 15 * time.Minute
	downloadKubeconfigTimeout = 600 * time.Second
	dashboardAvailableTimeout = 60 * time.Minute
)

var _ = ginkgo.Describe("Shoot application testing", func() {

	f := framework.NewShootFramework(nil)

	f.Default().Release().CIt("should fetch the shoot kubeconfig from the Seed cluster successfully", func(ctx context.Context) {
		kubeconfig, err := framework.GetObjectFromSecret(ctx, f.SeedClient, f.ShootSeedNamespace(), v1beta1constants.SecretNameGardener, framework.KubeconfigSecretKeyName)
		framework.ExpectNoError(err)
		gomega.Expect(kubeconfig).ToNot(gomega.BeNil())
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

	f.Default().Beta().CIt("Dashboard should be available", func(ctx context.Context) {
		shoot := f.Shoot
		if !shoot.Spec.Addons.KubernetesDashboard.Enabled {
			ginkgo.Fail("The test requires .spec.addons.kubernetesDashboard.enabled to be true")
		}

		url := fmt.Sprintf("https://api.%s/api/v1/namespaces/%s/services/https:kubernetes-dashboard:/proxy", *f.Shoot.Spec.DNS.Domain, "kubernetes-dashboard")
		serviceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameGardener,
				Namespace: metav1.NamespaceSystem,
			},
		}
		token, err := framework.CreateTokenForServiceAccount(ctx, f.ShootClient, serviceAccount, ptr.To[int64](3600))
		framework.ExpectNoError(err)

		err = framework.TestHTTPEndpointWithToken(ctx, url, token)
		framework.ExpectNoError(err)
	}, dashboardAvailableTimeout)

})
