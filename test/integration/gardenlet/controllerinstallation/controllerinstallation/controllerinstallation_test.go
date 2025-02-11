// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerinstallation_test

import (
	"encoding/base64"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/controllerinstallation"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ControllerInstallation controller tests", func() {
	var (
		controllerRegistration *gardencorev1beta1.ControllerRegistration
		controllerDeployment   *gardencorev1.ControllerDeployment
		controllerInstallation *gardencorev1beta1.ControllerInstallation

		chartWithGardenKubeconfig    []byte
		chartWithoutGardenKubeconfig []byte
	)

	BeforeEach(func() {
		controllerRegistration = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "registration-",
				Labels:       map[string]string{testID: testRunID},
			},
		}

		// created via the following commands in the ./testdata/chart-* directories:
		//   helm package . --version 0.1.0 --app-version 0.1.0 --destination /tmp/chart
		//   cat /tmp/chart/test-0.1.0.tgz | base64 | tr -d '\n'
		var err error
		chartWithGardenKubeconfig, err = base64.StdEncoding.DecodeString(`H4sIFAAAAAAA/ykAK2FIUjBjSE02THk5NWIzVjBkUzVpWlM5Nk9WVjZNV2xqYW5keVRRbz1IZWxtAOyWX2uzMBSHvc6nOF/g1cRG++Jtd7tdjFHY5UHPZqjGYNJCsf3uQzu7TjY6WP+wzecmzSlRE37PIY6sC2Y51s5fY1l454BzzmMpu5FzPhy5kFNPyGkc8ngiYulxEYp44gE/y9cMWFqHtce//a7h5n4IaNScaqsqncBKMDRmP+W+8DnLyKa1Mq4rPZB1LK9KSiB3ztgkCBb/ra+qIKeiZBrbf9pIsdX7p1x7myOf0PnvqDQFOrJBWukn9XziVnDM/zDkA//lJIpG/y/BwP+F0lkCsy4Ft2hYSQ4zdJgwgJ3du4S8Tq3BlBJoGvDvqSC05N/1ZdhuWb90hcWSbAIb1jTgqkcsC/DnXRE2oHRG2oFsV1z7PP4aA/8zMkW1Lkmf8jpw1P9IDPyPwqkc/b8Eh/6jMTbYN4GbfRQ+6AJvOfliJ7CG0nZ5H7X2N0BfbUkr7VBpqm1f+QcHF4prH9TIyMjIL+MlAAD//xIdYCMAEAAA`)
		Expect(err).NotTo(HaveOccurred())
		chartWithoutGardenKubeconfig, err = base64.StdEncoding.DecodeString(`H4sIFAAAAAAA/ykAK2FIUjBjSE02THk5NWIzVjBkUzVpWlM5Nk9WVjZNV2xqYW5keVRRbz1IZWxtAOyWQW+cMBCFOftXjHJfYxYKFdf02h6qKlKPE5hk3TXGYmZXjTb57xVQtilqlUpNdluF72L5IY+x9d7IQizx5QY70XfYuOglMMaYPMuG0RgzH02SFVGSFfna5GlSFJFJ1kleRGBe5G9m7Fiwi8xf7zU/3H8CBntFHdvWl7BPFIZwnBqdaKNq4qqzQQbpE7GoTdtQCRuRwGUcb9+ytm28Idcoj/2X3lJq/3OVcx9z4TcM+RdqgkMhjqvW39jbZ24FT+V/vTaz/Gdpniz5PwWz/G+tr0u4HFzwHoNqSLBGwVIBjOkeHfJ9ygErKuFwAP2RHCGT/jDJ8PCgpqV7dDviEu7V4QDSfsbGgb4aRLgH62vyAlm/4tz38dqY5b+m4Nq7hvxzPgeeyn+ap7P8v0lNtuT/FDzOP4bA8bEJvDta4Rdd4IdP/rATADi8Jsd9CQD6KuT7TVnfYleTp05Xrt3VsfVfqJLVqK62u2saG04JFzfomC4UB6r6KpNpx4qT2lO1XtB66nhSVvDoaXLuK19YWFj4J/gWAAD//1y8BI4AEAAA`)
		Expect(err).NotTo(HaveOccurred())

		controllerDeployment = &gardencorev1.ControllerDeployment{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "deploy-",
				Labels:       map[string]string{testID: testRunID},
			},
			Helm: &gardencorev1.HelmControllerDeployment{
				RawChart: chartWithGardenKubeconfig,
			},
			InjectGardenKubeconfig: ptr.To(true),
		}
		controllerInstallation = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "installation-",
				Labels:       map[string]string{testID: testRunID},
				Annotations:  map[string]string{"security.gardener.cloud/pod-security-enforce": "privileged"},
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
			controllerDeployment.Helm = nil
			metav1.SetMetaDataAnnotation(&controllerDeployment.ObjectMeta, gardencorev1.MigrationControllerDeploymentType, "not-responsible")
		})

		It("should not reconcile", func() {
			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
				return controllerInstallation.Status.Conditions
			}).ShouldNot(ContainCondition(OfType(gardencorev1beta1.ControllerInstallationInstalled)))
		})
	})

	Context("responsible with OCI", func() {
		BeforeEach(func() {
			DeferCleanup(test.WithVar(&controllerinstallation.RequeueDurationWhenResourceDeletionStillPresent, 500*time.Millisecond))

			oci := &gardencorev1.OCIRepository{
				Repository: ptr.To("test"),
				Tag:        ptr.To("0.1.0"),
			}
			controllerDeployment.Helm = &gardencorev1.HelmControllerDeployment{
				OCIRepository: oci,
			}
			fakeRegistry.AddArtifact(oci, chartWithGardenKubeconfig)
			fakeRegistry.SetExpectedPullSecretNamespace(gardenerutils.ComputeGardenNamespace(seed.Name))
		})

		It("should deploy the chart", func() {
			By("Ensure chart was deployed correctly")
			values := make(map[string]any)
			Eventually(func(g Gomega) {
				managedResource := &resourcesv1alpha1.ManagedResource{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: "garden", Name: controllerInstallation.Name}, managedResource)).To(Succeed())

				secret := &corev1.Secret{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: managedResource.Namespace, Name: managedResource.Spec.SecretRefs[0].Name}, secret)).To(Succeed())

				configMap := &corev1.ConfigMap{}
				Expect(runtime.DecodeInto(newCodec(), secret.Data["test_templates_config.yaml"], configMap)).To(Succeed())
				Expect(yaml.Unmarshal([]byte(configMap.Data["values"]), &values)).To(Succeed())

				deployment := &appsv1.Deployment{}
				Expect(runtime.DecodeInto(newCodec(), secret.Data["test_templates_deployment.yaml"], deployment)).To(Succeed())
				Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal("test"))
			}).Should(Succeed())

			By("Ensure conditions are maintained correctly")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
				return controllerInstallation.Status.Conditions
			}).Should(And(
				ContainCondition(OfType(gardencorev1beta1.ControllerInstallationValid), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("RegistrationValid")),
				ContainCondition(OfType(gardencorev1beta1.ControllerInstallationInstalled), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("InstallationPending")),
			))
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
					HaveKeyWithValue("pod-security.kubernetes.io/enforce", "privileged"),
					HaveKeyWithValue("high-availability-config.resources.gardener.cloud/consider", "true"),
				))
				g.Expect(namespace.Annotations).To(And(
					HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "a,b,c"),
				))
			}).Should(Succeed())

			By("Ensure generic garden kubeconfig was created")
			var genericKubeconfigSecret *corev1.Secret
			Eventually(func(g Gomega) {
				secretList := &corev1.SecretList{}
				g.Expect(testClient.List(ctx, secretList, client.InNamespace(namespace.Name))).To(Succeed())

				for _, secret := range secretList.Items {
					if strings.HasPrefix(secret.Name, "generic-garden-kubeconfig-") {
						genericKubeconfigSecret = secret.DeepCopy()
						break
					}
				}
				g.Expect(genericKubeconfigSecret).NotTo(BeNil())
				g.Expect(genericKubeconfigSecret.Data).To(HaveKeyWithValue("kubeconfig", Not(BeEmpty())))
			}).Should(Succeed())

			By("Ensure garden access secret was created")
			Eventually(func(g Gomega) {
				secret := &corev1.Secret{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: namespace.Name, Name: "garden-access-extension"}, secret)).To(Succeed())
				g.Expect(secret.Labels).To(And(
					HaveKeyWithValue("resources.gardener.cloud/class", "garden"),
					HaveKeyWithValue("resources.gardener.cloud/purpose", "token-requestor"),
				))
				g.Expect(secret.Annotations).To(
					HaveKeyWithValue("serviceaccount.resources.gardener.cloud/name", "extension-"+controllerInstallation.Name),
				)
			}).Should(Succeed())

			By("Ensure chart was deployed correctly")
			values := make(map[string]any)
			Eventually(func(g Gomega) {
				managedResource := &resourcesv1alpha1.ManagedResource{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: "garden", Name: controllerInstallation.Name}, managedResource)).To(Succeed())

				secret := &corev1.Secret{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: managedResource.Namespace, Name: managedResource.Spec.SecretRefs[0].Name}, secret)).To(Succeed())

				configMap := &corev1.ConfigMap{}
				Expect(runtime.DecodeInto(newCodec(), secret.Data["test_templates_config.yaml"], configMap)).To(Succeed())
				Expect(yaml.Unmarshal([]byte(configMap.Data["values"]), &values)).To(Succeed())

				deployment := &appsv1.Deployment{}
				Expect(runtime.DecodeInto(newCodec(), secret.Data["test_templates_deployment.yaml"], deployment)).To(Succeed())
				Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal("test"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(HaveExactElements(
					corev1.EnvVar{Name: "GARDEN_KUBECONFIG", Value: "/var/run/secrets/gardener.cloud/garden/generic-kubeconfig/kubeconfig"},
					corev1.EnvVar{Name: "SEED_NAME", Value: seed.Name},
				))
			}).Should(Succeed())

			// Our envtest setup starts gardener-apiserver in-process which adds its own feature gates as well as the default
			// Kubernetes features gates to the same map that is reused in the tested gardenlet controller:
			// `features.DefaultFeatureGate` is the same as `utilfeature.DefaultMutableFeatureGate`
			// Hence, these feature gates are also mixed into the helm values.
			// Here we assert that all known gardenlet features are correctly passed to the helm values but ignore the rest.
			gardenletValues := (values["gardener"].(map[string]any))["gardenlet"].(map[string]any)
			for _, feature := range gardenletfeatures.GetFeatures() {
				Expect(gardenletValues["featureGates"]).To(HaveKeyWithValue(string(feature), features.DefaultFeatureGate.Enabled(feature)))
			}

			delete(gardenletValues, "featureGates")
			(values["gardener"].(map[string]any))["gardenlet"] = gardenletValues

			valuesBytes, err := yaml.Marshal(values)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(valuesBytes)).To(Equal(`gardener:
  garden:
    clusterIdentity: ` + gardenClusterIdentity + `
    genericKubeconfigSecretName: ` + genericKubeconfigSecret.Name + `
  gardenlet: {}
  seed:
    annotations: null
    blockCIDRs: null
    clusterIdentity: ` + seedClusterIdentity + `
    ingressDomain: ` + seed.Spec.Ingress.Domain + `
    labels:
      ` + testID + `: ` + testRunID + `
      dnsrecord.extensions.gardener.cloud/` + seed.Spec.DNS.Provider.Type + `: "true"
      name.seed.gardener.cloud/` + seed.Name + `: "true"
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
          prober:
            enabled: true
          weeder:
            enabled: true
        excessCapacityReservation:
          configs:
          - resources:
              cpu: "2"
              memory: 6Gi
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

		When("garden kubeconfig injection is undesired", func() {
			var namespace *corev1.Namespace

			JustBeforeEach(func() {
				By("Ensure namespace was created")
				namespace = &corev1.Namespace{}
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "extension-" + controllerInstallation.Name}, namespace)).To(Succeed())
					g.Expect(namespace.Labels).To(And(
						HaveKeyWithValue("gardener.cloud/role", "extension"),
						HaveKeyWithValue("controllerregistration.core.gardener.cloud/name", controllerRegistration.Name),
						HaveKeyWithValue("pod-security.kubernetes.io/enforce", "privileged"),
						HaveKeyWithValue("high-availability-config.resources.gardener.cloud/consider", "true"),
					))
					g.Expect(namespace.Annotations).To(And(
						HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "a,b,c"),
					))
				}).Should(Succeed())

				By("Ensure generic garden kubeconfig was created")
				var genericKubeconfigSecret *corev1.Secret
				Eventually(func(g Gomega) {
					secretList := &corev1.SecretList{}
					g.Expect(testClient.List(ctx, secretList, client.InNamespace(namespace.Name))).To(Succeed())

					for _, secret := range secretList.Items {
						if strings.HasPrefix(secret.Name, "generic-garden-kubeconfig-") {
							genericKubeconfigSecret = secret.DeepCopy()
							break
						}
					}
					g.Expect(genericKubeconfigSecret).NotTo(BeNil())
					g.Expect(genericKubeconfigSecret.Data).To(HaveKeyWithValue("kubeconfig", Not(BeEmpty())))
				}).Should(Succeed())

				By("Ensure garden access secret was created")
				Eventually(func(g Gomega) {
					secret := &corev1.Secret{}
					g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: namespace.Name, Name: "garden-access-extension"}, secret)).To(Succeed())
					g.Expect(secret.Labels).To(And(
						HaveKeyWithValue("resources.gardener.cloud/class", "garden"),
						HaveKeyWithValue("resources.gardener.cloud/purpose", "token-requestor"),
					))
					g.Expect(secret.Annotations).To(
						HaveKeyWithValue("serviceaccount.resources.gardener.cloud/name", "extension-"+controllerInstallation.Name),
					)
				}).Should(Succeed())

				By("Ensure chart was deployed correctly")
				values := make(map[string]any)
				Eventually(func(g Gomega) {
					managedResource := &resourcesv1alpha1.ManagedResource{}
					g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: "garden", Name: controllerInstallation.Name}, managedResource)).To(Succeed())

					secret := &corev1.Secret{}
					g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: managedResource.Namespace, Name: managedResource.Spec.SecretRefs[0].Name}, secret)).To(Succeed())

					configMap := &corev1.ConfigMap{}
					Expect(runtime.DecodeInto(newCodec(), secret.Data["test_templates_config.yaml"], configMap)).To(Succeed())
					Expect(yaml.Unmarshal([]byte(configMap.Data["values"]), &values)).To(Succeed())

					deployment := &appsv1.Deployment{}
					Expect(runtime.DecodeInto(newCodec(), secret.Data["test_templates_deployment.yaml"], deployment)).To(Succeed())
					Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal("test"))
					Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(HaveExactElements(
						corev1.EnvVar{Name: "GARDEN_KUBECONFIG", Value: "/var/run/secrets/gardener.cloud/garden/generic-kubeconfig/kubeconfig"},
						corev1.EnvVar{Name: "SEED_NAME", Value: seed.Name},
					))
				}).Should(Succeed())

				By("Ensure conditions are maintained correctly")
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
					return controllerInstallation.Status.Conditions
				}).Should(And(
					ContainCondition(OfType(gardencorev1beta1.ControllerInstallationValid), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("RegistrationValid")),
					ContainCondition(OfType(gardencorev1beta1.ControllerInstallationInstalled), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("InstallationPending")),
				))
			})

			test := func() {
				By("Retrigger reconciliation")
				patch := client.MergeFrom(controllerInstallation.DeepCopy())
				controllerInstallation.Spec.DeploymentRef.ResourceVersion = "reconcile-again-please"
				Expect(testClient.Patch(ctx, controllerInstallation, patch)).To(Succeed())

				By("Ensure garden access secret was deleted")
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKey{Namespace: namespace.Name, Name: "garden-access-extension"}, &corev1.Secret{})
				}).Should(BeNotFoundError())

				By("Ensure chart was redeployed without kubeconfig injection")
				Eventually(func(g Gomega) {
					managedResource := &resourcesv1alpha1.ManagedResource{}
					g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: "garden", Name: controllerInstallation.Name}, managedResource)).To(Succeed())

					secret := &corev1.Secret{}
					g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: managedResource.Namespace, Name: managedResource.Spec.SecretRefs[0].Name}, secret)).To(Succeed())

					deployment := &appsv1.Deployment{}
					Expect(runtime.DecodeInto(newCodec(), secret.Data["test_templates_deployment.yaml"], deployment)).To(Succeed())
					Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal("test"))
					Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(HaveExactElements(
						corev1.EnvVar{Name: "SEED_NAME", Value: seed.Name},
					))
				}).Should(Succeed())

				By("Ensure conditions are maintained correctly")
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
					return controllerInstallation.Status.Conditions
				}).Should(And(
					ContainCondition(OfType(gardencorev1beta1.ControllerInstallationValid), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("RegistrationValid")),
					ContainCondition(OfType(gardencorev1beta1.ControllerInstallationInstalled), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("InstallationPending")),
				))
			}

			It("should remove the garden access secret and kubeconfig injection when disabled via ControllerDeployment", func() {
				By("Disable garden kubeconfig injection")
				controllerDeployment.InjectGardenKubeconfig = ptr.To(false)
				Expect(testClient.Update(ctx, controllerDeployment)).To(Succeed())

				test()
			})

			It("should remove the garden access secret and kubeconfig injection when disabled via label", func() {
				By("Disable garden kubeconfig injection")
				controllerDeployment.Helm.RawChart = chartWithoutGardenKubeconfig
				Expect(testClient.Update(ctx, controllerDeployment)).To(Succeed())

				test()
			})
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

			By("Create ServiceAccount for garden access secret")
			// This ServiceAccount is typically created by the token-requestor controller which does not run in this
			// integration test, so let's fake it here.
			gardenClusterServiceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
				Name:      "extension-" + controllerInstallation.Name,
				Namespace: seedNamespace.Name,
			}}
			Expect(testClient.Create(ctx, gardenClusterServiceAccount)).To(Succeed())

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
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenClusterServiceAccount), gardenClusterServiceAccount)).To(BeNotFoundError())
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

		Context("when seed is garden at the same time", func() {
			BeforeEach(func() {
				garden := &operatorv1alpha1.Garden{
					ObjectMeta: metav1.ObjectMeta{GenerateName: "garden-"},
					Spec: operatorv1alpha1.GardenSpec{
						RuntimeCluster: operatorv1alpha1.RuntimeCluster{
							Networking: operatorv1alpha1.RuntimeNetworking{
								Pods:     []string{"10.1.0.0/16"},
								Services: []string{"10.2.0.0/16"},
							},
							Ingress: operatorv1alpha1.Ingress{
								Domains: []operatorv1alpha1.DNSDomain{{Name: "ingress.dev.seed.example.com"}},
								Controller: gardencorev1beta1.IngressController{
									Kind: "nginx",
								},
							},
						},
						VirtualCluster: operatorv1alpha1.VirtualCluster{
							DNS: operatorv1alpha1.DNS{
								Domains: []operatorv1alpha1.DNSDomain{{Name: "virtual-garden.local.gardener.cloud"}},
							},
							Gardener: operatorv1alpha1.Gardener{
								ClusterIdentity: "test",
							},
							Kubernetes: operatorv1alpha1.Kubernetes{
								Version: "1.31.1",
							},
							Maintenance: operatorv1alpha1.Maintenance{
								TimeWindow: gardencorev1beta1.MaintenanceTimeWindow{
									Begin: "220000+0100",
									End:   "230000+0100",
								},
							},
							Networking: operatorv1alpha1.Networking{
								Services: []string{"100.64.0.0/13"},
							},
						},
					},
				}
				Expect(testClient.Create(ctx, garden)).To(Succeed())
				DeferCleanup(func() { Expect(testClient.Delete(ctx, garden)).To(Succeed()) })
			})

			It("should properly label the namespace with the network policy label", func() {
				By("Ensure namespace was created and labeled correctly")
				Eventually(func(g Gomega) {
					namespace := &corev1.Namespace{}
					g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "extension-" + controllerInstallation.Name}, namespace)).To(Succeed())
					g.Expect(namespace.Labels).To(HaveKeyWithValue("networking.gardener.cloud/access-target-apiserver", "allowed"))
				}).Should(Succeed())
			})
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
