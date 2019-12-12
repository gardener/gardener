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
		- Tests the hibernation of a  shoot.

	Prerequisites
		- A Shoot exists.

	Test:
		Deploys a default application and hibernates the cluster.
		When the cluster is successfully hibernated it is woken up and the deployed application is tested.
	Expected Output
		- Shoot and deployed app is fully functional after hibernation and wakeup.
 **/

package hibernation

import (
	"context"
	"flag"
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
	shootName      = flag.String("shoot-name", "", "name of the shoot")
	shootNamespace = flag.String("shoot-namespace", "", "project namespace of the shoot")
	kubeconfigPath = flag.String("kubecfg", "", "the path to the kubeconfig of Garden cluster that will be used for integration tests")
	logLevel       = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
)

const (
	initializationTimeout  = 1 * time.Minute
	hibernationTestTimeout = 1 * time.Hour
	cleanupTimeout         = 10 * time.Minute
	dumpStateTimeout       = 5 * time.Minute
)

func validateFlags() {
	if !StringSet(*shootName) {
		Fail("flag '--shoot-name' needs to be specified")
	}
	if !StringSet(*shootNamespace) {
		Fail("flag '--project-namespace' needs to be specified")
	}
	if !StringSet(*kubeconfigPath) {
		Fail("flag '--kubeconfig' needs to be specified")
	}
}

var _ = Describe("Shoot Hibernation testing", func() {

	var (
		shootGardenerTest      *ShootGardenerTest
		gardenerTestOperations *GardenerTestOperation
		shootAppTestLogger     *logrus.Logger
		guestBookTest          *applications.GuestBookTest

		resourcesDir = filepath.Join("..", "..", "resources")
	)

	CBeforeSuite(func(ctx context.Context) {
		shootAppTestLogger = logger.NewLogger(*logLevel)
		validateFlags()

		var err error
		shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: *shootNamespace, Name: *shootName}}
		shootGardenerTest, err = NewShootGardenerTest(*kubeconfigPath, shoot, shootAppTestLogger)
		Expect(err).NotTo(HaveOccurred())

		gardenerTestOperations, err = NewGardenTestOperationWithShoot(ctx, shootGardenerTest.GardenClient, shootAppTestLogger, shoot)
		Expect(err).NotTo(HaveOccurred())

		guestBookTest, err = applications.NewGuestBookTest(resourcesDir)
		Expect(err).ToNot(HaveOccurred())

	}, initializationTimeout)

	CAfterSuite(func(ctx context.Context) {
		guestBookTest.Cleanup(ctx, gardenerTestOperations)
	}, cleanupTimeout)

	CAfterEach(func(ctx context.Context) {
		gardenerTestOperations.AfterEach(ctx)
	}, dumpStateTimeout)

	CIt("Testing if Shoot can be hibernated successfully", func(ctx context.Context) {
		By("Deploy guestbook")
		guestBookTest.DeployGuestBookApp(ctx, gardenerTestOperations)
		guestBookTest.Test(ctx, gardenerTestOperations)

		By("Hibernate shoot")
		err := shootGardenerTest.HibernateShoot(ctx)
		Expect(err).ToNot(HaveOccurred())

		By("wake up shoot")
		err = shootGardenerTest.WakeUpShoot(ctx)
		Expect(err).ToNot(HaveOccurred())

		By("test guestbook")
		guestBookTest.WaitUntilPrerequisitesAreReady(ctx, gardenerTestOperations)
		guestBookTest.Test(ctx, gardenerTestOperations)

	}, hibernationTestTimeout)
})
