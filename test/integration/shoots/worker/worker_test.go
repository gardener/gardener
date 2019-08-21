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

package worker

import (
	"context"
	"flag"
	"fmt"
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
	kubeconfig        = flag.String("kubeconfig", "", "the path to the kubeconfig of Garden cluster that will be used for integration tests")
	testShootsPrefix  = flag.String("prefix", "", "prefix to use for test shoots")
	shootTestYamlPath = flag.String("shootpath", "", "the path to the shoot yaml that will be used for testing")
	shootName         = flag.String("shootName", "", "the name of the shoot we want to test")
	shootNamespace    = flag.String("shootNamespace", "", "the namespace name that the shoot resides in")
	logLevel          = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
	cleanup           = flag.Bool("cleanup", true, "deletes the newly created / existing test shoot after the test suite is done")
)

const (
	InitializationTimeout = 15 * time.Minute
	TearDownTimeout       = 5 * time.Minute
	DumpStateTimeout      = 5 * time.Minute
)

func xOR(arg1, arg2 bool) bool {
	return (arg1 || arg2) && !(arg1 && arg2)
}

func validateFlags() {

	if !xOR(StringSet(*shootTestYamlPath), StringSet(*shootName)) {
		Fail("You should set either shootName or shootpath")
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

var _ = Describe("Worker Suite", func() {
	var (
		gardenTestOperation         *GardenerTestOperation
		workerGardenerTest          *WorkerGardenerTest
		shootGardenerTest           *ShootGardenerTest
		workerTestLogger            *logrus.Logger
		areThereEnoughMachineImages bool
	)

	areThereMoreThanOneMachineImageInShoot := func(s *WorkerGardenerTest) bool {
		machineImages, err := s.GetMachineImagesFromShoot()
		Expect(err).NotTo(HaveOccurred())

		firstMachineImage := machineImages[0]
		for i := 1; i < len(machineImages); i++ {
			if firstMachineImage.Name != machineImages[i].Name {
				return true
			}
		}

		return false
	}

	CBeforeSuite(func(ctx context.Context) {
		validateFlags()

		workerTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		var (
			shoot *v1beta1.Shoot
			err   error
		)

		if StringSet(*shootTestYamlPath) {
			_, shootObject, err := CreateShootTestArtifacts(*shootTestYamlPath, *testShootsPrefix, false)
			Expect(err).NotTo(HaveOccurred())

			shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, shootObject, workerTestLogger)
			Expect(err).NotTo(HaveOccurred())

			By("Creating shoot...")
			shoot, err = shootGardenerTest.CreateShoot(ctx)
			Expect(err).NotTo(HaveOccurred())
		}

		if StringSet(*shootName) {
			shoot = &v1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: *shootNamespace, Name: *shootName}}

			shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, shoot, workerTestLogger)
			Expect(err).NotTo(HaveOccurred())
		}

		gardenTestOperation, err = NewGardenTestOperation(ctx, shootGardenerTest.GardenClient, workerTestLogger, shoot)
		Expect(err).NotTo(HaveOccurred())

		workerGardenerTest, err = NewWorkerGardenerTest(ctx, shootGardenerTest, gardenTestOperation.ShootClient)
		Expect(err).NotTo(HaveOccurred())

		areThereEnoughMachineImages = areThereMoreThanOneMachineImageInShoot(workerGardenerTest)
	}, InitializationTimeout)

	CAfterSuite(func(ctx context.Context) {
		By("Deleting the shoot")
		if workerGardenerTest == nil {
			return
		}

		if *cleanup {
			err := workerGardenerTest.ShootGardenerTest.DeleteShoot(ctx)
			Expect(err).NotTo(HaveOccurred())
		}
	}, InitializationTimeout)

	CAfterEach(func(ctx context.Context) {
		gardenTestOperation.AfterEach(ctx)
	}, DumpStateTimeout)

	// This test creates and deletes shoots all together
	CIt("should create a shoot with two diffent nodes", func(ctx context.Context) {
		By(fmt.Sprintf("Checking if shoot is compatible for testing"))
		if !areThereEnoughMachineImages {
			Skip("For the purpose of this test, you must provide a shoot with at least two workers with two MachineImages")
		}

		nodesList, err := workerGardenerTest.ShootClient.Kubernetes().CoreV1().Nodes().List(metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		areThereTwoDifferentNodes := false
		firstNode := nodesList.Items[0]

		for i := 1; i < len(nodesList.Items); i++ {
			if firstNode.Status.NodeInfo.OSImage != nodesList.Items[i].Status.NodeInfo.OSImage {
				areThereTwoDifferentNodes = true
				break
			}
		}

		Expect(areThereTwoDifferentNodes).To(Equal(true))
	}, TearDownTimeout)
})
