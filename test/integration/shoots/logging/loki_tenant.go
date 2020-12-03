// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package logging

import (
	"context"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"

	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

const (
	tenantInitializationTimeout          = 2 * time.Minute
	tenantGetLogsFromLokiTimeout         = 5 * time.Minute
	tenantLoggerDeploymentCleanupTimeout = 5 * time.Minute

	randomLength           = 11
	userLoggerAppLabel     = "kube-logger"
	operatorLoggerAppLabel = "logger"
	tenantDeltaLogsCount   = 0
	tenantLogsCount        = 100
	tenantLogsDuration     = "20s"
)

var (
	userLoggerName     = "kube-apiserver-"
	operatorLoggerName = "logger-"
)
var _ = ginkgo.Describe("Seed logging testing", func() {

	f := framework.NewShootFramework(nil)
	grafanaOperatorsIngress := &extensionsv1beta1.Ingress{}
	grafanaUsersIngress := &extensionsv1beta1.Ingress{}

	framework.CBeforeEach(func(ctx context.Context) {
		checkRequiredResources(ctx, f.SeedClient)
		//Get the grafana-operators Ingress
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: f.ShootSeedNamespace(), Name: v1beta1constants.DeploymentNameGrafanaOperators}, grafanaOperatorsIngress))
		//Get the grafana-users Ingress
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: f.ShootSeedNamespace(), Name: v1beta1constants.DeploymentNameGrafanaUsers}, grafanaUsersIngress))
	}, tenantInitializationTimeout)

	f.Beta().CIt("should get container logs from loki by tenant", func(ctx context.Context) {
		userLoggerName = userLoggerName + utilrand.String(randomLength)
		userLoggerRegex := userLoggerName + "-.*"

		operatorLoggerName = operatorLoggerName + utilrand.String(randomLength)
		operatorLoggerRegex := operatorLoggerName + "-.*"

		ginkgo.By("Get Loki tenant IDs")
		userID := getXScopeOrgID(grafanaUsersIngress.Annotations)
		operatorID := getXScopeOrgID(grafanaOperatorsIngress.Annotations)

		ginkgo.By("Get existing user Loki tenant IDs")
		search, err := f.GetLokiLogs(ctx, userID, f.ShootSeedNamespace(), "pod_name", userLoggerRegex, f.SeedClient)
		framework.ExpectNoError(err)
		initialUsersLogs, err := getLogCountFromResult(search)
		framework.ExpectNoError(err)
		expectedUserLogs := tenantLogsCount + initialUsersLogs

		ginkgo.By("Get existing operator Loki tenant IDs")
		search, err = f.GetLokiLogs(ctx, operatorID, f.ShootSeedNamespace(), "pod_name", operatorLoggerRegex, f.SeedClient)
		framework.ExpectNoError(err)
		initialOperatorsLogs, err := getLogCountFromResult(search)
		framework.ExpectNoError(err)
		expectedOperatorLogs := tenantLogsCount + initialOperatorsLogs

		ginkgo.By("Deploy the user logger application")
		loggerParams := map[string]interface{}{
			"LoggerName":          userLoggerName,
			"HelmDeployNamespace": f.ShootSeedNamespace(),
			"AppLabel":            userLoggerAppLabel,
			"LogsCount":           tenantLogsCount,
			"LogsDuration":        tenantLogsDuration,
		}

		err = f.RenderAndDeployTemplate(ctx, f.SeedClient, templates.LoggerAppName, loggerParams)
		framework.ExpectNoError(err)

		ginkgo.By("Deploy the operator logger application")
		loggerParams["LoggerName"] = operatorLoggerName
		loggerParams["AppLabel"] = operatorLoggerAppLabel

		err = f.RenderAndDeployTemplate(ctx, f.SeedClient, templates.LoggerAppName, loggerParams)
		framework.ExpectNoError(err)

		ginkgo.By("Wait until user logger application is ready")
		loggerLabels := labels.SelectorFromSet(labels.Set(map[string]string{
			"app": userLoggerAppLabel,
		}))

		err = f.WaitUntilDeploymentsWithLabelsIsReady(ctx, loggerLabels, f.ShootSeedNamespace(), f.SeedClient)
		framework.ExpectNoError(err)

		ginkgo.By("Wait until operator logger application is ready")
		loggerLabels = labels.SelectorFromSet(labels.Set(map[string]string{
			"app": operatorLoggerAppLabel,
		}))

		err = f.WaitUntilDeploymentsWithLabelsIsReady(ctx, loggerLabels, f.ShootSeedNamespace(), f.SeedClient)
		framework.ExpectNoError(err)

		ginkgo.By("Verify loki received all user logger application logs")
		err = WaitUntilLokiReceivesLogs(ctx, 30*time.Second, f, userID, f.ShootSeedNamespace(), "pod_name", userLoggerRegex, expectedUserLogs, tenantDeltaLogsCount, f.SeedClient)
		framework.ExpectNoError(err)

		ginkgo.By("Verify loki received all user logger application logs as an operator")
		err = WaitUntilLokiReceivesLogs(ctx, 30*time.Second, f, operatorID, f.ShootSeedNamespace(), "pod_name", userLoggerRegex, expectedUserLogs, tenantDeltaLogsCount, f.SeedClient)
		framework.ExpectNoError(err)

		ginkgo.By("Verify loki received all operator logger application logs")
		err = WaitUntilLokiReceivesLogs(ctx, 30*time.Second, f, operatorID, f.ShootSeedNamespace(), "pod_name", operatorLoggerRegex, expectedOperatorLogs, tenantDeltaLogsCount, f.SeedClient)
		framework.ExpectNoError(err)

		ginkgo.By("Verify that loki will not show the operator logs to the user tenant")
		err = WaitUntilLokiReceivesLogs(ctx, 30*time.Second, f, userID, f.ShootSeedNamespace(), "pod_name", operatorLoggerRegex, 0, tenantDeltaLogsCount, f.SeedClient)
		framework.ExpectNoError(err)

	}, tenantGetLogsFromLokiTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		ginkgo.By("Cleaning up user logger app resources")
		loggerDeploymentToDelete := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: f.ShootSeedNamespace(),
				Name:      userLoggerName,
			},
		}
		err := kutil.DeleteObject(ctx, f.SeedClient.Client(), loggerDeploymentToDelete)
		framework.ExpectNoError(err)

		ginkgo.By("Cleaning up operator logger app resources")
		loggerDeploymentToDelete = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: f.ShootSeedNamespace(),
				Name:      operatorLoggerName,
			},
		}
		err = kutil.DeleteObject(ctx, f.SeedClient.Client(), loggerDeploymentToDelete)
		framework.ExpectNoError(err)
	}, tenantLoggerDeploymentCleanupTimeout))
})
