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

package operations_test

import (
	"context"
	"flag"
	"time"

	. "github.com/gardener/gardener/test/integration/shoots"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var (
	kubeconfig        = flag.String("kubeconfig", "", "the path to the kubeconfig  of Garden the cluster that will be used for integration tests")
	shootTestYamlPath = flag.String("shootpath", "", "the path to the shoot yaml that will be used for testing")
	testShootsPrefix  = flag.String("prefix", "", "prefix to use for test shoots")
	logLevel          = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
)

const (
	WaitForCreateDeleteTimeout = 7200 * time.Second
	InitializationTimeout      = 600 * time.Second
)

func validateFlags() {
	if StringSet(*shootTestYamlPath) {
		if !FileExists(*shootTestYamlPath) {
			Fail("shoot yaml path is set but invalid")
		}
	}

	if !StringSet(*kubeconfig) {
		Fail("you need to specify the correct path for the kubeconfig")
	}

	if !FileExists(*kubeconfig) {
		Fail("kubeconfig path does not exist")
	}
}

var _ = Describe("Shoot creation/deletion testing", func() {
	var (
		shootGardenerTest         *ShootGardenerTest
		shoot                     *v1beta1.Shoot
		shootOperationsTestLogger *logrus.Logger
	)

	CBeforeSuite(func(ctx context.Context) {
		validateFlags()

		// parse shoot yaml into shoot object and generate random test names for shoots
		_, shootObject, err := CreateShootTestArtifacts(*shootTestYamlPath, *testShootsPrefix)
		Expect(err).To(BeNil())

		shoot = shootObject
		shootOperationsTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, shoot, shootOperationsTestLogger)
		Expect(err).To(BeNil())
	}, InitializationTimeout)

	// This test creates and deletes shoots all together
	CIt("should create and delete shoot successfully", func(ctx context.Context) {
		// First we create the target shoot.
		shootCreateDelete := context.WithValue(ctx, "name", "create and delete shoot")

		_, err := shootGardenerTest.CreateShoot(shootCreateDelete)
		Expect(err).NotTo(HaveOccurred())
		// Now we should test shoot deletion
		shootOperationsTestLogger.Infof("Testing shoot deletion for %s", shoot.Name)
		// Then we delete the shoot
		err = shootGardenerTest.DeleteShoot(shootCreateDelete)
		Expect(err).NotTo(HaveOccurred())
	}, WaitForCreateDeleteTimeout)

})
