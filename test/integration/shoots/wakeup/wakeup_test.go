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
		- Tests the wakeup of a hibernated  shoot.

	Prerequisites
		- Hibernated Shoot exists.

	Test: Testing if Shoot can be woken-up from hibernation successfully.
	Expected Output
		- Successful reconciliation of the Shoot.
 **/

package wakeup

import (
	"context"
	"flag"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"time"

	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/shoots"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/test/integration/framework"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
)

var (
	kubeconfigPath   = flag.String("kubecfg", "", "the path to the kubeconfigPath of Garden cluster that will be used for integration tests")
	shootName        = flag.String("shoot-name", "", "name of the shoot")
	projectNamespace = flag.String("project-namespace", "", "project namespace of the shoot")
	testLogger       *logrus.Logger
)

func validateFlags() {
	if !StringSet(*shootName) {
		Fail("flag '--shoot-name' needs to be specified")
	}
	if !StringSet(*projectNamespace) {
		Fail("flag '--project-namespace' needs to be specified")
	}
	if !StringSet(*kubeconfigPath) {
		Fail("flag '--kubeconfigPath' needs to be specified")
	}
}

var _ = Describe("Shoot Wakeup testing", func() {
	CBeforeSuite(func(ctx context.Context) {
		testLogger = logger.NewLogger("debug")
		validateFlags()
	}, 5*time.Second)

	CIt("Testing if Shoot can be woken-up from hibernation successfully", func(ctx context.Context) {
		gardenerConfigPath := *kubeconfigPath

		shoot := &gardencorev1alpha1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: *projectNamespace, Name: *shootName}}
		shootGardenerTest, err := framework.NewShootGardenerTest(gardenerConfigPath, shoot, testLogger)
		Expect(err).To(BeNil())

		if err := shootGardenerTest.WakeUpShoot(context.TODO()); err != nil && !errors.IsNotFound(err) {
			Expect(err).To(BeNil())
		}
	}, 7000*time.Second)
})
