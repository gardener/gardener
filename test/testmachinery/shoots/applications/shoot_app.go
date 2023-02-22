// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

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

	f.Default().Release().CIt("should download shoot kubeconfig successfully", func(ctx context.Context) {
		err := framework.DownloadKubeconfig(ctx, f.SeedClient, f.ShootSeedNamespace(), v1beta1constants.SecretNameGardener, "test-kubecfg")
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
			ginkgo.Fail("The test requires .spec.addons.kubernetesDashboard.enabled to be true")
		}

		url := fmt.Sprintf("https://api.%s/api/v1/namespaces/%s/services/https:kubernetes-dashboard:/proxy", *f.Shoot.Spec.DNS.Domain, "kubernetes-dashboard")
		serviceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameGardener,
				Namespace: metav1.NamespaceSystem,
			},
		}
		token, err := framework.CreateTokenForServiceAccount(ctx, f.ShootClient, serviceAccount, pointer.Int64(3600))
		framework.ExpectNoError(err)

		err = framework.TestHTTPEndpointWithToken(ctx, url, token)
		framework.ExpectNoError(err)
	}, dashboardAvailableTimeout)

})
