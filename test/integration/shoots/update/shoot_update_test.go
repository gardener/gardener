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

package shootupdate

import (
	"context"
	"flag"
	"fmt"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"time"

	. "github.com/gardener/gardener/test/integration/shoots"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	kubeconfig        = flag.String("kubeconfig", "", "the path to the kubeconfig  of the garden cluster that will be used for integration tests")
	kubernetesVersion = flag.String("version", "", "the version to update the shoot")
	shootName         = flag.String("shootName", "", "the name of the shoot we want to test")
	shootNamespace    = flag.String("shootNamespace", "", "the namespace name that the shoot resides in")
	testShootsPrefix  = flag.String("prefix", "", "prefix to use for test shoots")
	logLevel          = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
	shootTestYamlPath = flag.String("shootpath", "", "the path to the shoot yaml that will be used for testing")
	cleanup           = flag.Bool("cleanup", false, "deletes the newly created / existing test shoot after the test suite is done")
)

const (
	UpdateKubernetesVersionTimeout = 600 * time.Second
	InitializationTimeout          = 600 * time.Second
	DumpStateTimeout               = 5 * time.Minute
)

func validateFlags() {
	if StringSet(*shootTestYamlPath) && StringSet(*shootName) {
		Fail("You can set either the shoot YAML path or specify a shootName to test against")
	}

	if !StringSet(*shootTestYamlPath) && !StringSet(*shootName) {
		Fail("You should either set the shoot YAML path or specify a shootName to test against")
	}

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

var _ = Describe("Shoot update testing", func() {
	var (
		shootGardenerTest   *ShootGardenerTest
		shootTestOperations *GardenerTestOperation
		shootTestLogger     *logrus.Logger
	)

	CBeforeSuite(func(ctx context.Context) {
		// validate flags
		validateFlags()
		shootTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		// check if a shoot spec is provided, if yes create a shoot object from it and use it for testing
		if StringSet(*shootTestYamlPath) {
			*cleanup = true
			// parse shoot yaml into shoot object and generate random test names for shoots
			_, shootObject, err := CreateShootTestArtifacts(*shootTestYamlPath, *testShootsPrefix, true)
			Expect(err).NotTo(HaveOccurred())

			shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, shootObject, shootTestLogger)
			Expect(err).NotTo(HaveOccurred())

			targetTestShoot, err := shootGardenerTest.CreateShoot(ctx)
			Expect(err).NotTo(HaveOccurred())

			shootTestOperations, err = NewGardenTestOperation(ctx, shootGardenerTest.GardenClient, shootTestLogger, targetTestShoot)
			Expect(err).NotTo(HaveOccurred())
		}

		if StringSet(*shootName) {
			var err error
			shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, nil, shootTestLogger)
			Expect(err).NotTo(HaveOccurred())

			shoot := &v1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: *shootNamespace, Name: *shootName}}
			shootTestOperations, err = NewGardenTestOperation(ctx, shootGardenerTest.GardenClient, shootTestLogger, shoot)
			Expect(err).NotTo(HaveOccurred())
		}

	}, InitializationTimeout)

	CAfterEach(func(ctx context.Context) {
		shootTestOperations.AfterEach(ctx)
	}, DumpStateTimeout)

	CIt("should update the kubernetes version of the shoot to the next available minor version", func(ctx context.Context) {
		currentVersion := shootTestOperations.Shoot.Spec.Kubernetes.Version
		newVersion := *kubernetesVersion
		if newVersion == "" {
			var (
				err error
				ok  bool
			)
			cloudprofile := shootTestOperations.SeedCloudProfile
			ok, newVersion, err = helper.DetermineNextKubernetesMinorVersion(*cloudprofile, currentVersion)
			Expect(err).ToNot(HaveOccurred())
			if !ok {
				Skip("no new version found")
			}
		}

		By(fmt.Sprintf("updating shoot %s to version %s", shootTestOperations.Shoot.Name, newVersion))
		shootTestOperations.Shoot.Spec.Kubernetes.Version = newVersion

		_, err := shootGardenerTest.UpdateShoot(ctx, shootTestOperations.Shoot)
		Expect(err).ToNot(HaveOccurred())

	}, UpdateKubernetesVersionTimeout)

})
