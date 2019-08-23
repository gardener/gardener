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

package gardener_reconcile_test

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/test/integration/framework"
	"github.com/sirupsen/logrus"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/test/integration/shoots"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	kubeconfigPath  = flag.String("kubeconfig", "", "the path to the kubeconfig path  of the garden cluster that will be used for integration tests")
	gardenerVersion = flag.String("version", "", "current gardener version")
	logLevel        = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
)

const (
	ReconcileShootsTimeout = 1 * time.Hour
	InitializationTimeout  = 20 * time.Second
	DumpStateTimeout       = 5 * time.Minute
)

func validateFlags() {

	if !StringSet(*kubeconfigPath) {
		Fail("you need to specify the correct path for the kubeconfigpath")
	}

	if !FileExists(*kubeconfigPath) {
		Fail("kubeconfigpath path does not exist")
	}

	if !StringSet(*gardenerVersion) {
		Fail("you need to specify the current gardener version")
	}
}

var _ = Describe("Shoot reconciliation testing", func() {
	var (
		gardenerTestOperation *framework.GardenerTestOperation
		gardenClient          kubernetes.Interface
		testLogger            *logrus.Logger
	)

	CBeforeSuite(func(ctx context.Context) {
		validateFlags()
		testLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		var err error
		gardenClient, err = kubernetes.NewClientFromFile("", *kubeconfigPath, kubernetes.WithClientOptions(
			client.Options{
				Scheme: kubernetes.GardenScheme,
			}),
		)
		Expect(err).ToNot(HaveOccurred())

		gardenerTestOperation, err = framework.NewGardenTestOperation(ctx, gardenClient, testLogger, nil)
		Expect(err).ToNot(HaveOccurred())

	}, InitializationTimeout)

	CAfterEach(func(ctx context.Context) {
		gardenerTestOperation.AfterEach(ctx)
	}, DumpStateTimeout)

	CIt("Should reconcile all shoots", func(ctx context.Context) {
		err := retry.UntilTimeout(ctx, 30*time.Second, ReconcileShootsTimeout, func(ctx context.Context) (bool, error) {
			shoots := &v1beta1.ShootList{}
			err := gardenClient.Client().List(ctx, shoots)
			if err != nil {
				testLogger.Debug(err.Error())
				return retry.MinorError(err)
			}

			reconciledShoots := 0
			for _, shoot := range shoots.Items {
				// check if the last acted gardener version is the current version,
				// to determine if the updated gardener version reconciled the shoot.
				if shoot.Status.Gardener.Version != *gardenerVersion {
					testLogger.Debugf("last acted gardener version %s does not match current gardener version %s", shoot.Status.Gardener.Version, *gardenerVersion)
					continue
				}
				if framework.ShootCreationCompleted(&shoot) {
					reconciledShoots++
				}
			}

			if reconciledShoots != len(shoots.Items) {
				err := fmt.Errorf("Reconciled %d of %d shoots. Waiting ...", reconciledShoots, len(shoots.Items))
				testLogger.Info(err.Error())
				return retry.MinorError(err)
			}

			return retry.Ok()
		})
		Expect(err).ToNot(HaveOccurred())

	}, ReconcileShootsTimeout)

})
