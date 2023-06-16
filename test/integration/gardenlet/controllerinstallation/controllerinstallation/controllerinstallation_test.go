// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerinstallation_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/controllerinstallation"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ControllerInstallation controller tests", func() {
	var (
		controllerRegistration *gardencorev1beta1.ControllerRegistration
		controllerDeployment   *gardencorev1beta1.ControllerDeployment
		controllerInstallation *gardencorev1beta1.ControllerInstallation
	)

	BeforeEach(func() {
		controllerRegistration = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "registration-",
				Labels:       map[string]string{testID: testRunID},
			},
		}
		controllerDeployment = &gardencorev1beta1.ControllerDeployment{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "deploy-",
				Labels:       map[string]string{testID: testRunID},
			},
			Type: "helm",
			ProviderConfig: runtime.RawExtension{
				// created via the following commands in the ./testdata/chart directory:
				//   helm package . --version 0.1.0 --app-version 0.1.0 --destination /tmp/chart
				//   cat /tmp/chart/test-0.1.0.tgz | base64 | tr -d '\n'
				Raw: []byte(`{"chart": "H4sIFAAAAAAA/ykAK2FIUjBjSE02THk5NWIzVjBkUzVpWlM5Nk9WVjZNV2xqYW5keVRRbz1IZWxtAOyUz2rDMAzGc/ZT6AkcOXHb4WvP22GMwo6i0RbT/DGxWhhp3300XQcLjB22rozldxGSkW2Z77NwlHRZUif6heoquQSIiHNrh4iI44jGLhJjc5PnmZkvbIImM7N5AniR24zYRqEuwW+fNR7uj0DBr7iLvm0c7IyiEN5T1EajKjiuOx9kKD1wFFW2NTsoRUJ0abq5idq3aclVrRo6rhwlpXYfd7n2mBOfMPhfuA4VCcd03TZP/vmHv4Kv/J9lOPK/zWc4+f83GPl/45vCwXJQwS0FVbNQQUJOAZzcfVLIWxoDrdlB34O+54opsr47l+FwUOfWHVVbjg72qu9B2keqK9CroQh78E3BjYA9dlz7PSYmJib+C68BAAD//6xO2UUADAAA"}`),
			},
		}
		controllerInstallation = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "installation-",
				Labels:       map[string]string{testID: testRunID},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create ControllerRegistration")
		Expect(testClient.Create(ctx, controllerRegistration)).To(Succeed())
		log.Info("Created ControllerRegistration", "controllerRegistration", client.ObjectKeyFromObject(controllerRegistration))

		By("Wait until manager has observed ControllerRegistration")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)
		}).Should(Succeed())

		By("Create ControllerDeployment")
		Expect(testClient.Create(ctx, controllerDeployment)).To(Succeed())
		log.Info("Created ControllerDeployment", "controllerDeployment", client.ObjectKeyFromObject(controllerDeployment))

		By("Wait until manager has observed ControllerDeployment")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(controllerDeployment), controllerDeployment)
		}).Should(Succeed())

		By("Create ControllerInstallation")
		controllerInstallation.Spec.SeedRef = corev1.ObjectReference{Name: seed.Name}
		controllerInstallation.Spec.RegistrationRef = corev1.ObjectReference{Name: controllerRegistration.Name}
		controllerInstallation.Spec.DeploymentRef = &corev1.ObjectReference{Name: controllerDeployment.Name}
		Expect(testClient.Create(ctx, controllerInstallation)).To(Succeed())
		log.Info("Created ControllerInstallation", "controllerInstallation", client.ObjectKeyFromObject(controllerInstallation))

		By("Wait until manager has observed ControllerInstallation")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)
		}).Should(Succeed())

		DeferCleanup(func() {
			By("Delete ControllerInstallation")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, controllerInstallation))).To(Succeed())

			By("Wait for ControllerInstallation to be gone")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)
			}).Should(BeNotFoundError())

			By("Delete ControllerDeployment")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, controllerDeployment))).To(Succeed())

			By("Delete ControllerRegistration")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, controllerRegistration))).To(Succeed())

			By("Wait for ControllerDeployment to be gone")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(controllerDeployment), controllerDeployment)
			}).Should(BeNotFoundError())

			By("Wait for ControllerRegistration to be gone")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)
			}).Should(BeNotFoundError())
		})
	})

	Context("not responsible", func() {
		BeforeEach(func() {
			controllerDeployment.Type = "not-responsible"
		})

		It("should not reconcile", func() {
			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
				return controllerInstallation.Status.Conditions
			}).ShouldNot(ContainCondition(OfType(gardencorev1beta1.ControllerInstallationInstalled)))
		})
	})

	Context("responsible", func() {
		BeforeEach(func() {
			DeferCleanup(test.WithVar(&controllerinstallation.RequeueDurationWhenResourceDeletionStillPresent, 500*time.Millisecond))
		})

		JustBeforeEach(func() {
			By("Ensure finalizer got added")
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
				return controllerInstallation.Finalizers
			}).Should(ConsistOf("core.gardener.cloud/controllerinstallation"))
		})

		It("should create a namespace and deploy the chart", func() {
			By("Ensure namespace was created")
			namespace := &corev1.Namespace{}
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "extension-" + controllerInstallation.Name}, namespace)).To(Succeed())
				g.Expect(namespace.Labels).To(And(
					HaveKeyWithValue("gardener.cloud/role", "extension"),
					HaveKeyWithValue("controllerregistration.core.gardener.cloud/name", controllerRegistration.Name),
					HaveKeyWithValue("high-availability-config.resources.gardener.cloud/consider", "true"),
				))
				g.Expect(namespace.Annotations).To(And(
					HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "a,b,c"),
				))
			}).Should(Succeed())

			By("Ensure chart was deployed correctly")
			// Note that the list of feature gates is unexpectedly longer than in reality since the envtest starts
			// gardener-apiserver which adds its own as well as the default Kubernetes features gates to the same
			// map that is reused in gardenlet:
			// `features.DefaultFeatureGate` is the same as `utilfeature.DefaultMutableFeatureGate`
			Eventually(func(g Gomega) string {
				managedResource := &resourcesv1alpha1.ManagedResource{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: "garden", Name: controllerInstallation.Name}, managedResource)).To(Succeed())

				secret := &corev1.Secret{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: managedResource.Namespace, Name: managedResource.Spec.SecretRefs[0].Name}, secret)).To(Succeed())

				configMap := &corev1.ConfigMap{}
				Expect(runtime.DecodeInto(newCodec(), secret.Data["test_templates_config.yaml"], configMap)).To(Succeed())

				return configMap.Data["values"]
			}).Should(Equal(`gardener:
  garden:
    clusterIdentity: ` + gardenClusterIdentity + `
  gardenlet:
    featureGates:
      APIListChunking: true
      APIPriorityAndFairness: true
      APIResponseCompression: true
      APIServerIdentity: true
      APIServerTracing: false
      AdvancedAuditing: true
      AggregatedDiscoveryEndpoint: false
      AllAlpha: false
      AllBeta: false
      ComponentSLIs: false
      CoreDNSQueryRewriting: false
      CustomResourceValidationExpressions: true
      DefaultSeccompProfile: false
      DisableScalingClassesForShoots: false
      DryRun: true
      EfficientWatchResumption: true
      HVPA: false
      HVPAForShootedSeed: false
      IPv6SingleStack: false
      KMSv2: false
      MachineControllerManagerDeployment: false
      MutableShootSpecNetworkingNodes: false
      OpenAPIEnums: true
      OpenAPIV3: true
      RemainingItemCount: true
      RemoveSelfLink: true
      ServerSideApply: true
      ServerSideFieldValidation: true
      StorageVersionAPI: false
      StorageVersionHash: true
      ValidatingAdmissionPolicy: false
      WatchBookmark: true
      WorkerlessShoots: false
  seed:
    annotations: null
    blockCIDRs: null
    clusterIdentity: ` + seedClusterIdentity + `
    ingressDomain: ` + seed.Spec.Ingress.Domain + `
    labels:
      ` + testID + `: ` + testRunID + `
      dnsrecord.extensions.gardener.cloud/` + seed.Spec.DNS.Provider.Type + `: "true"
      provider.extensions.gardener.cloud/` + seed.Spec.Provider.Type + `: "true"
    name: ` + seed.Name + `
    networks:
      ipFamilies:
      - IPv4
      nodes: ` + *seed.Spec.Networks.Nodes + `
      pods: ` + seed.Spec.Networks.Pods + `
      services: ` + seed.Spec.Networks.Services + `
    protected: false
    provider: ` + seed.Spec.Provider.Type + `
    region: ` + seed.Spec.Provider.Region + `
    spec:
      dns:
        provider:
          secretRef:
            name: ` + seed.Spec.DNS.Provider.SecretRef.Name + `
            namespace: ` + seed.Spec.DNS.Provider.SecretRef.Namespace + `
          type: ` + seed.Spec.DNS.Provider.Type + `
      ingress:
        controller:
          kind: ` + seed.Spec.Ingress.Controller.Kind + `
        domain: ` + seed.Spec.Ingress.Domain + `
      networks:
        ipFamilies:
        - IPv4
        nodes: ` + *seed.Spec.Networks.Nodes + `
        pods: ` + seed.Spec.Networks.Pods + `
        services: ` + seed.Spec.Networks.Services + `
      provider:
        region: ` + seed.Spec.Provider.Region + `
        type: ` + seed.Spec.Provider.Type + `
        zones:
        - a
        - b
        - c
      settings:
        dependencyWatchdog:
          endpoint:
            enabled: true
          probe:
            enabled: true
          prober:
            enabled: true
          weeder:
            enabled: true
        excessCapacityReservation:
          enabled: true
        ownerChecks:
          enabled: false
        scheduling:
          visible: true
        topologyAwareRouting:
          enabled: false
        verticalPodAutoscaler:
          enabled: true
    taints: null
    visible: true
    volumeProvider: ""
    volumeProviders: null
  version: 1.2.3
`))

			By("Ensure conditions are maintained correctly")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
				return controllerInstallation.Status.Conditions
			}).Should(And(
				ContainCondition(OfType(gardencorev1beta1.ControllerInstallationValid), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("RegistrationValid")),
				ContainCondition(OfType(gardencorev1beta1.ControllerInstallationInstalled), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("InstallationPending")),
			))
		})

		It("should properly clean up on ControllerInstallation deletion", func() {
			var (
				namespace       = &corev1.Namespace{}
				managedResource = &resourcesv1alpha1.ManagedResource{}
				secret          = &corev1.Secret{}
			)

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "extension-" + controllerInstallation.Name}, namespace)).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: "garden", Name: controllerInstallation.Name}, managedResource)).To(Succeed())
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: managedResource.Namespace, Name: managedResource.Spec.SecretRefs[0].Name}, secret)).To(Succeed())
			}).Should(Succeed())

			By("Delete ControllerInstallation")
			Expect(testClient.Delete(ctx, controllerInstallation)).To(Succeed())

			By("Wait for ControllerInstallation to be gone")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)
			}).Should(BeNotFoundError())

			By("Verify controller artefacts were removed")
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespace), namespace)).To(BeNotFoundError())
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(BeNotFoundError())
		})

		It("should not overwrite the Installed condition when it is not 'Unknown'", func() {
			By("Wait for condition to be maintained initially")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
				return controllerInstallation.Status.Conditions
			}).Should(ContainCondition(OfType(gardencorev1beta1.ControllerInstallationInstalled), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("InstallationPending")))

			By("Overwrite condition with status 'True'")
			patch := client.StrategicMergeFrom(controllerInstallation.DeepCopy())
			controllerInstallation.Status.Conditions = helper.MergeConditions(controllerInstallation.Status.Conditions, gardencorev1beta1.Condition{Type: gardencorev1beta1.ControllerInstallationInstalled, Status: gardencorev1beta1.ConditionTrue})
			Expect(testClient.Status().Patch(ctx, controllerInstallation, patch)).To(Succeed())

			By("Ensure condition is not overwritten")
			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
				return controllerInstallation.Status.Conditions
			}).Should(ContainCondition(OfType(gardencorev1beta1.ControllerInstallationInstalled), WithStatus(gardencorev1beta1.ConditionTrue)))
		})
	})
})

func newCodec() runtime.Codec {
	var groupVersions []schema.GroupVersion
	for k := range kubernetes.SeedScheme.AllKnownTypes() {
		groupVersions = append(groupVersions, k.GroupVersion())
	}
	return kubernetes.SeedCodec.CodecForVersions(kubernetes.SeedSerializer, kubernetes.SeedSerializer, schema.GroupVersions(groupVersions), schema.GroupVersions(groupVersions))
}
