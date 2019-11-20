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
		- Tests the communication between all nodes of the shoot

	AfterSuite
		- Cleanup Workload in Shoot

	Test: Create a nginx daemonset and test if it is reachable from each node.
	Expected Output
		- nginx's are reachable from each node
 **/

package networking_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"text/template"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	. "github.com/gardener/gardener/test/integration/shoots"

	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	kubeconfig     = flag.String("kubecfg", "", "the path to the kubeconfig  of the garden cluster that will be used for integration tests")
	shootName      = flag.String("shoot-name", "", "the name of the shoot we want to test")
	shootNamespace = flag.String("shoot-namespace", "", "the namespace name that the shoot resides in")
	logLevel       = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
)

const (
	NetworkTestTimeout    = 1800 * time.Second
	InitializationTimeout = 600 * time.Second
	FinalizationTimeout   = 1800 * time.Second
	DumpStateTimeout      = 5 * time.Minute
)

const (
	nginxTemplateName = "network-nginx-deamonset.yaml.tpl"
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

var _ = Describe("Shoot network testing", func() {
	var (
		shootGardenerTest      *ShootGardenerTest
		gardenerTestOperations *GardenerTestOperation
		networkTestLogger      *logrus.Logger

		resourcesDir = filepath.Join("..", "..", "resources")

		name      = "net-test"
		namespace = "default"
	)

	CBeforeSuite(func(ctx context.Context) {
		// validate flags
		validateFlags()
		networkTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		var err error
		shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, nil, networkTestLogger)
		Expect(err).NotTo(HaveOccurred())

		shoot := &gardencorev1alpha1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: *shootNamespace, Name: *shootName}}
		gardenerTestOperations, err = NewGardenTestOperationWithShoot(ctx, shootGardenerTest.GardenClient, networkTestLogger, shoot)
		Expect(err).NotTo(HaveOccurred())
	}, InitializationTimeout)

	CAfterSuite(func(ctx context.Context) {
		err := gardenerTestOperations.ShootClient.Client().Delete(ctx, &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				Expect(err).To(HaveOccurred())
			}
		}
	}, FinalizationTimeout)

	CAfterEach(func(ctx context.Context) {
		gardenerTestOperations.AfterEach(ctx)
	}, DumpStateTimeout)

	CIt("should reach all webservers on all nodes", func(ctx context.Context) {
		templateFilepath := filepath.Join(resourcesDir, "templates", nginxTemplateName)
		nettestTmpl := template.Must(template.ParseFiles(templateFilepath))

		By("Deploy the net test daemon set")
		var writer bytes.Buffer
		err := nettestTmpl.Execute(&writer, map[string]string{
			"name":      name,
			"namespace": namespace,
		})
		Expect(err).NotTo(HaveOccurred())

		manifestReader := kubernetes.NewManifestReader(writer.Bytes())
		err = gardenerTestOperations.ShootClient.Applier().ApplyManifest(ctx, manifestReader, kubernetes.DefaultApplierOptions)
		Expect(err).NotTo(HaveOccurred())

		err = gardenerTestOperations.WaitUntilDaemonSetIsRunning(ctx, name, namespace, gardenerTestOperations.ShootClient)
		Expect(err).NotTo(HaveOccurred())

		pods := &corev1.PodList{}
		err = gardenerTestOperations.ShootClient.Client().List(ctx, pods, client.MatchingLabels{"app": "net-nginx"})
		Expect(err).NotTo(HaveOccurred())

		// check if all webservers can be reached from all nodes
		By("test connectivity to webservers")
		shootRESTConfig := gardenerTestOperations.ShootClient.RESTConfig()
		var res error
		for _, from := range pods.Items {
			for _, to := range pods.Items {
				By(fmt.Sprintf("Testing %s to %s", from.GetName(), to.GetName()))
				reader, err := kubernetes.NewPodExecutor(shootRESTConfig).Execute(ctx, from.Namespace, from.Name, "net-curl", fmt.Sprintf("curl -L %s:80 --fail", to.Status.PodIP))
				if err != nil {
					res = multierror.Append(res, errors.Wrapf(err, "%s to %s", from.GetName(), to.GetName()))
					continue
				}
				data, err := ioutil.ReadAll(reader)
				if err != nil {
					networkTestLogger.Error(err)
					continue
				}
				networkTestLogger.Infof("%s to %s: %s", from.GetName(), to.GetName(), data)
			}
		}
		Expect(res).ToNot(HaveOccurred())
	}, NetworkTestTimeout)

})
