// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"
)

const (
	shootInitializationTimeout          = 2 * time.Minute
	shootGetLogsFromValiTimeout         = 5 * time.Minute
	shootLoggerDeploymentCleanupTimeout = 5 * time.Minute

	randomLength        = 11
	loggerAppLabel      = "logger"
	loggerName          = "logger"
	shootDeltaLogsCount = 0
	shootLogsCount      = 100
	shootLogsDuration   = "20s"
)

var (
	fullLoggerName = loggerName + "-" + utilrand.String(randomLength)
	valiLabels     = map[string]string{
		"app":  "vali",
		"role": "logging",
	}
)

var _ = ginkgo.Describe("Seed logging testing", func() {

	shootFramework := framework.NewShootFramework(nil)

	var (
		plutonoIngress client.Object = &networkingv1.Ingress{}

		shootNamespace           = &corev1.Namespace{}
		shootNamespaceLabelKey   = "gardener.cloud/test"
		shootNamespaceLabelValue = "logging"
	)

	framework.CBeforeEach(func(ctx context.Context) {

		checkRequiredResources(ctx, shootFramework.SeedClient)

		// Get shoot namespace name
		shootNamespace.Name = shootFramework.ShootSeedNamespace()

		seedClient := shootFramework.SeedClient.Client()
		// Get the plutono Ingress
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: shootFramework.ShootSeedNamespace(),
					Name:      v1beta1constants.DeploymentNamePlutono},
				plutonoIngress,
			),
		)
		// Set label to the testing namespace
		_, err := controllerutils.GetAndCreateOrMergePatch(ctx,
			seedClient, shootNamespace,
			func() error {
				metav1.SetMetaDataLabel(&shootNamespace.ObjectMeta, shootNamespaceLabelKey, shootNamespaceLabelValue)
				return nil
			},
		)
		framework.ExpectNoError(err)

		// spellchecker:off
		// Deploy Vali ValidatingWebhookConfiguration
		validatingWebhookParams := map[string]any{
			"NamespaceLabelKey":   shootNamespaceLabelKey,
			"NamespaceLabelValue": shootNamespaceLabelValue,
			"CABundle": utils.EncodeBase64([]byte(`-----BEGIN CERTIFICATE-----
MIIC+jCCAeKgAwIBAgIUTp3XvhrWOVM8ZGe86YoXMV/UJ7AwDQYJKoZIhvcNAQEL
BQAwFTETMBEGA1UEAxMKa3ViZXJuZXRlczAeFw0xOTAyMjcxNTM0MDBaFw0yNDAy
MjYxNTM0MDBaMBUxEzARBgNVBAMTCmt1YmVybmV0ZXMwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQCyi0QGOcv2bTf3N8OLN97RwsgH6QAr8wSpAOrttBJg
FnfnU2T1RHgxm7qd190WL8DChv0dZf76d6eSQ4ZrjjyArTzufb4DtPwg+VWq7XvF
BNyn+2hf4SySkwd6k7XLhUTRx048IbByC4v+FEvmoLAwrc0d0G14ec6snD+7jO7e
kykQ/NgAOL7P6kDs9z6+bOfgF0nGN+bmeWQqJejR0t+OyQDCx5/FMtUfEVR5QX80
aeefgp3JFZb6fAw9KhLtdRV3FP0tz6hS+e4Sg0mwAAOqijZsV87kP5GYzjtcfA12
lDYl/nb1GtVvvkQD49VnV7mDnl6mG3LCMNCNH6WlZNv3AgMBAAGjQjBAMA4GA1Ud
DwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBSFA3LvJM21d8qs
ZVVCe6RrTT9wiTANBgkqhkiG9w0BAQsFAAOCAQEAns/EJ3yKsjtISoteQ714r2Um
BMPyUYTTdRHD8LZMd3RykvsacF2l2y88Nz6wJcAuoUj1h8aBDP5oXVtNmFT9jybS
TXrRvWi+aeYdb556nEA5/a94e+cb+Ck3qy/1xgQoS457AZQODisDZNBYWkAFs2Lc
ucpcAtXJtIthVm7FjoAHYcsrY04yAiYEJLD02TjUDXg4iGOGMkVHdmhawBDBF3Aj
esfcqFwji6JyAKFRACPowykQONFwUSom89uYESSCJFvNCk9MJmjJ2PzDUt6CypR4
epFdd1fXLwuwn7fvPMmJqD3HtLalX1AZmPk+BI8ezfAiVcVqnTJQMXlYPpYe9A==
-----END CERTIFICATE-----`)),
		}
		// spellchecker:on

		err = shootFramework.RenderAndDeployTemplate(ctx,
			shootFramework.SeedClient, templates.BlockValiValidatingWebhookConfiguration, validatingWebhookParams,
		)
		framework.ExpectNoError(err)
	}, initializationTimeout)

	shootFramework.Beta().CIt("should get container logs from the shoot cluster", func(ctx context.Context) {
		loggerRegex := fullLoggerName + "-.*"

		ginkgo.By("Wait until Vali StatefulSet is ready")
		framework.ExpectNoError(
			shootFramework.WaitUntilStatefulSetIsRunning(ctx,
				valiName, shootFramework.ShootSeedNamespace(), shootFramework.SeedClient,
			),
		)

		ginkgo.By("Compute expected logs")
		search, err := shootFramework.GetValiLogs(ctx,
			valiLabels, shootFramework.ShootSeedNamespace(), "pod_name",
			loggerRegex, shootFramework.SeedClient,
		)
		framework.ExpectNoError(err)

		initialLogs, err := getLogCountFromResult(search)
		framework.ExpectNoError(err)
		expectedLogs := shootLogsCount + initialLogs

		ginkgo.By("Check again if Vali StatefulSet is ready")
		framework.ExpectNoError(
			shootFramework.WaitUntilStatefulSetIsRunning(ctx, valiName, shootFramework.ShootSeedNamespace(), shootFramework.SeedClient),
		)

		ginkgo.By("Deploy the logger application")
		loggerParams := map[string]any{
			"LoggerName":          fullLoggerName,
			"HelmDeployNamespace": shootFramework.ShootSeedNamespace(),
			"AppLabel":            loggerAppLabel,
			"LogsCount":           shootLogsCount,
			"LogsDuration":        shootLogsDuration,
		}

		err = shootFramework.RenderAndDeployTemplate(ctx, shootFramework.SeedClient, templates.LoggerAppName, loggerParams)
		framework.ExpectNoError(err)

		ginkgo.By("Wait until logger application is ready")
		loggerLabels := labels.SelectorFromSet(map[string]string{
			"app": loggerAppLabel,
		})

		err = shootFramework.WaitUntilDeploymentsWithLabelsIsReady(ctx,
			loggerLabels, shootFramework.ShootSeedNamespace(), shootFramework.SeedClient,
		)
		framework.ExpectNoError(err)

		ginkgo.By("Verify vali received all logger application logs")
		err = WaitUntilValiReceivesLogs(ctx,
			30*time.Second, shootFramework, valiLabels, shootFramework.ShootSeedNamespace(),
			"pod_name", loggerRegex, expectedLogs, shootDeltaLogsCount, shootFramework.SeedClient,
		)
		framework.ExpectNoError(err)

	}, shootGetLogsFromValiTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		ginkgo.By("Cleanup logger app resources")
		loggerDeploymentToDelete := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: shootFramework.ShootSeedNamespace(),
				Name:      fullLoggerName,
			},
		}
		err := kubernetesutils.DeleteObject(ctx, shootFramework.SeedClient.Client(), loggerDeploymentToDelete)
		framework.ExpectNoError(err)

		ginkgo.By("Cleanup vali's MutatingWebhook and the additional label")
		webhookToDelete := &admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "block-vali-updates",
			},
		}
		err = kubernetesutils.DeleteObject(ctx, shootFramework.SeedClient.Client(), webhookToDelete)
		framework.ExpectNoError(err)

		_, err = controllerutils.GetAndCreateOrMergePatch(ctx, shootFramework.SeedClient.Client(), shootNamespace,
			func() error {
				delete(shootNamespace.Labels, shootNamespaceLabelKey)
				return nil
			},
		)
		framework.ExpectNoError(err)

	}, shootLoggerDeploymentCleanupTimeout))
})
