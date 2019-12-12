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
	"flag"
	"fmt"
	"path/filepath"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"
	"github.com/gardener/gardener/test/integration/framework/applications"
	. "github.com/gardener/gardener/test/integration/shoots"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	kubeconfig     = flag.String("kubecfg", "", "the path to the kubeconfig of the Garden cluster that will be used for integration tests")
	shootName      = flag.String("shoot-name", "", "the name of the shoot we want to test")
	shootNamespace = flag.String("shoot-namespace", "", "the namespace name that the shoot resides in")
	logLevel       = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
	downloadPath   = flag.String("download-path", "/tmp/test", "the path to which you download the kubeconfig")
)

const (
	GuestbookAppTimeout       = 1800 * time.Second
	DownloadKubeconfigTimeout = 600 * time.Second
	DashboardAvailableTimeout = 60 * time.Minute
	InitializationTimeout     = 600 * time.Second
	FinalizationTimeout       = 1800 * time.Second
	DumpStateTimeout          = 5 * time.Minute
)

func validateFlags() {
	if !StringSet(*shootName) {
		Fail("You should specify a shootName to test against")
	}

	if !StringSet(*kubeconfig) {
		Fail("you need to specify the correct path for the kubeconfig")
	}

	if !FileExists(*kubeconfig) {
		Fail("kubeconfig path does not exist")
	}
}

var _ = Describe("Shoot application testing", func() {
	var (
		shootGardenerTest   *ShootGardenerTest
		shootTestOperations *GardenerTestOperation
		shootAppTestLogger  *logrus.Logger
		guestBookTest       *applications.GuestBookTest

		resourcesDir = filepath.Join("..", "..", "resources")
	)

	CBeforeSuite(func(ctx context.Context) {
		// validate flags
		validateFlags()
		shootAppTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		var err error
		shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, nil, shootAppTestLogger)
		Expect(err).NotTo(HaveOccurred())

		shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: *shootNamespace, Name: *shootName}}
		shootTestOperations, err = NewGardenTestOperationWithShoot(ctx, shootGardenerTest.GardenClient, shootAppTestLogger, shoot)
		Expect(err).NotTo(HaveOccurred())

		guestBookTest, err = applications.NewGuestBookTest(resourcesDir)
		Expect(err).ToNot(HaveOccurred())
	}, InitializationTimeout)

	CAfterSuite(func(ctx context.Context) {
		guestBookTest.Cleanup(ctx, shootTestOperations)
	}, FinalizationTimeout)

	CAfterEach(func(ctx context.Context) {
		shootTestOperations.AfterEach(ctx)
	}, DumpStateTimeout)

	CIt("should download shoot kubeconfig successfully", func(ctx context.Context) {
		err := shootTestOperations.DownloadKubeconfig(ctx, shootTestOperations.SeedClient, shootTestOperations.ShootSeedNamespace(), gardencorev1beta1.GardenerName, *downloadPath)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Shoot Kubeconfig downloaded successfully to %s", *downloadPath))
	}, DownloadKubeconfigTimeout)

	CIt("should deploy guestbook app successfully", func(ctx context.Context) {
		guestBookTest.DeployGuestBookApp(ctx, shootTestOperations)
		guestBookTest.Test(ctx, shootTestOperations)
	}, GuestbookAppTimeout)

	CIt("Dashboard should be available", func(ctx context.Context) {
		shoot := shootTestOperations.Shoot
		if !shoot.Spec.Addons.KubernetesDashboard.Enabled {
			Fail("The test requires .spec.addons.kubernetesDashboard.enabled to be be true")
		}

		err := shootTestOperations.DashboardAvailable(ctx)
		Expect(err).NotTo(HaveOccurred())
	}, DashboardAvailableTimeout)

})
