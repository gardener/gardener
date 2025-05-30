// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mediumtouch

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/provider-local/local"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("gardenadm medium-touch scenario tests", Label("gardenadm", "medium-touch"), func() {
	BeforeEach(OncePerOrdered, func(SpecContext) {
		PrepareBinary()
	}, NodeTimeout(5*time.Minute))

	Describe("Prepare infrastructure and machines", Ordered, func() {
		const (
			shootName   = "root"
			technicalID = "shoot--garden--" + shootName
		)

		var session *gexec.Session

		It("should start the bootstrap flow", func() {
			// Start the gardenadm process but don't wait for it to complete so that we can asynchronously perform assertions
			// on individual steps in the test specs below.
			session = Run("bootstrap", "-d", "../../../example/gardenadm-local/resources/medium-touch")
		})

		It("should find the cloud provider secret", func(ctx SpecContext) {
			cloudProviderSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloudprovider", Namespace: technicalID}}
			Eventually(ctx, Object(cloudProviderSecret)).Should(HaveField("ObjectMeta.Labels", HaveKeyWithValue("gardener.cloud/purpose", "cloudprovider")))
		}, SpecTimeout(time.Minute))

		It("should deploy gardener-resource-manager", func(ctx SpecContext) {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameGardenerResourceManager, Namespace: technicalID}}
			Eventually(ctx, Object(deployment)).Should(BeHealthy(health.CheckDeployment))
		}, SpecTimeout(time.Minute))

		It("should deploy the provider extension", func(ctx SpecContext) {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-extension-" + local.Name, Namespace: "extension-" + local.Name}}
			Eventually(ctx, Object(deployment)).Should(BeHealthy(health.CheckDeployment))
		}, SpecTimeout(time.Minute))

		It("should deploy the infrastructure", func(ctx SpecContext) {
			infra := &extensionsv1alpha1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: technicalID}}
			Eventually(ctx, Object(infra)).Should(BeHealthy(health.CheckExtensionObject))
		}, SpecTimeout(time.Minute))

		It("should deploy machine-controller-manager", func(ctx SpecContext) {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameMachineControllerManager, Namespace: technicalID}}
			Eventually(ctx, Object(deployment)).Should(BeHealthy(health.CheckDeployment))
		}, SpecTimeout(time.Minute))

		It("should deploy a control plane machine", func(ctx SpecContext) {
			podList := &corev1.PodList{}
			Eventually(ctx, ObjectList(podList, client.InNamespace(technicalID), client.MatchingLabels{"app": "machine"})).
				Should(HaveField("Items", ConsistOf(HaveField("Status.Phase", corev1.PodRunning))))
		}, SpecTimeout(time.Minute))

		It("should finish successfully", func(ctx SpecContext) {
			Wait(ctx, session)
			Eventually(ctx, session.Err).Should(gbytes.Say("work in progress"))
		}, SpecTimeout(time.Minute))

		It("should run successfully a second time (should be idempotent)", func(ctx SpecContext) {
			RunAndWait(ctx, "bootstrap", "-d", "../../../example/gardenadm-local/resources/medium-touch")
		}, SpecTimeout(2*time.Minute))
	})
})
