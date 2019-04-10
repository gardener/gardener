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

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/test/integration/framework"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	shootName        string
	projectNamespace string
	kubeconfigPath   string
	testLogger       *logrus.Logger
)

func init() {
	testLogger = logger.NewLogger("debug")

	shootName = os.Getenv("SHOOT_NAME")
	if shootName == "" {
		testLogger.Fatalf("EnvVar 'SHOOT_NAME' needs to be specified")
	}
	projectNamespace = os.Getenv("PROJECT_NAMESPACE")
	if projectNamespace == "" {
		testLogger.Fatalf("EnvVar 'PROJECT_NAMESPACE' needs to be specified")
	}
	kubeconfigPath = os.Getenv("TM_KUBECONFIG_PATH")
	if kubeconfigPath == "" {
		testLogger.Fatalf("EnvVar 'TM_KUBECONFIG_PATH' needs to be specified")
	}
}

func main() {
	gardenerConfigPath := fmt.Sprintf("%s/gardener.config", kubeconfigPath)

	shoot := &v1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: projectNamespace, Name: shootName}}
	shootGardenerTest, err := framework.NewShootGardenerTest(gardenerConfigPath, shoot, testLogger)
	if err != nil {
		testLogger.Fatalf("Cannot create ShootGardenerTest %s", err.Error())
	}

	if err := shootGardenerTest.DeleteShoot(context.TODO()); err != nil && !errors.IsNotFound(err) {
		testLogger.Fatalf("Cannot delete shoot %s: %s", shootName, err.Error())
	}

}
