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
	"flag"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/test/integration/framework"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	shootName        = flag.String("shoot-name", "", "name of the shoot")
	projectNamespace = flag.String("project-namespace", "", "project namespace of the shoot")
	kubeconfigPath   = flag.String("kubeconfig", "", "the path to the kubeconfig of Garden cluster that will be used for integration tests")
	testLogger       *logrus.Logger
)

func init() {
	testLogger = logger.NewLogger("debug")
	flag.Parse()

	if *shootName == "" {
		testLogger.Fatalf("flag '--shoot-name' needs to be specified")
	}
	if *projectNamespace == "" {
		testLogger.Fatalf("flag '--project-namespace' needs to be specified")
	}
	if *kubeconfigPath == "" {
		testLogger.Fatalf("flag '--kubeconfig' needs to be specified")
	}
}

func main() {
	gardenerConfigPath := *kubeconfigPath // fmt.Sprintf("%s/gardener.config", *kubeconfigPath)

	shoot := &v1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: *projectNamespace, Name: *shootName}}
	shootGardenerTest, err := framework.NewShootGardenerTest(gardenerConfigPath, shoot, testLogger)
	if err != nil {
		testLogger.Fatalf("Cannot create ShootGardenerTest %s", err.Error())
	}

	if err := shootGardenerTest.HibernateShoot(context.TODO()); err != nil && !errors.IsNotFound(err) {
		testLogger.Fatalf("Cannot hibernate shoot %s: %s", shootName, err.Error())
	}

}
