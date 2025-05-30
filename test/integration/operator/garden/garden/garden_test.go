// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden_test

import (
	"context"
	"fmt"
	"time"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	extensioncomponent "github.com/gardener/gardener/pkg/component/extensions/extension"
	gardeneraccess "github.com/gardener/gardener/pkg/component/gardener/access"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	kubeapiserverexposure "github.com/gardener/gardener/pkg/component/kubernetes/apiserverexposure"
	kubecontrollermanager "github.com/gardener/gardener/pkg/component/kubernetes/controllermanager"
	"github.com/gardener/gardener/pkg/component/networking/istio"
	"github.com/gardener/gardener/pkg/component/networking/nginxingress"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
	gardencontroller "github.com/gardener/gardener/pkg/operator/controller/garden/garden"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/test/utils/namespacefinalizer"
	"github.com/gardener/gardener/test/utils/operationannotation"
)

var _ = Describe("Garden controller tests", func() {
	var (
		loadBalancerServiceAnnotations = map[string]string{"foo": "bar"}
		garden                         *operatorv1alpha1.Garden
		testRunID                      string
		testNamespace                  *corev1.Namespace

		extension                *operatorv1alpha1.Extension
		extensionManagedResource *resourcesv1alpha1.ManagedResource
		extensionType            string
		extensionTypeBeforeKAS   string
		extensionTypeAfterWorker string

		gardenletNameWithAutoUpdate    = "gardenlet-auto-update"
		gardenletNameWithoutAutoUpdate = "gardenlet-no-auto-update"
		noAutoUpdateRef                = "do-not-update-me"
	)

	BeforeEach(func() {
		DeferCleanup(test.WithVar(&secretsutils.GenerateKey, secretsutils.FakeGenerateKey))
		DeferCleanup(test.WithVars(
			&etcd.DefaultInterval, 100*time.Millisecond,
			&etcd.DefaultTimeout, 500*time.Millisecond,
			&gardeneraccess.TimeoutWaitForManagedResource, 500*time.Millisecond,
			&gardencontroller.IntervalWaitUntilExtensionReady, 100*time.Millisecond,
			&istio.TimeoutWaitForManagedResource, 500*time.Millisecond,
			&extensioncomponent.DefaultInterval, 100*time.Millisecond,
			&extensioncomponent.DefaultTimeout, 500*time.Millisecond,
			&kubeapiserverexposure.DefaultInterval, 100*time.Millisecond,
			&kubeapiserverexposure.DefaultTimeout, 500*time.Millisecond,
			&kubeapiserver.IntervalWaitForDeployment, 100*time.Millisecond,
			&kubeapiserver.TimeoutWaitForDeployment, 500*time.Millisecond,
			&kubeapiserver.Until, untilInTest,
			&kubecontrollermanager.IntervalWaitForDeployment, 100*time.Millisecond,
			&kubecontrollermanager.TimeoutWaitForDeployment, 500*time.Millisecond,
			&kubecontrollermanager.Until, untilInTest,
			&nginxingress.TimeoutWaitForManagedResource, 500*time.Millisecond,
			&resourcemanager.SkipWebhookDeployment, true,
			&resourcemanager.IntervalWaitForDeployment, 100*time.Millisecond,
			&resourcemanager.TimeoutWaitForDeployment, 500*time.Millisecond,
			&resourcemanager.Until, untilInTest,
			&shared.IntervalWaitForGardenerResourceManagerBootstrapping, 500*time.Millisecond,
			&managedresources.IntervalWait, 100*time.Millisecond,
		))

		By("Create test Namespace")
		testNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "garden-",
			},
		}
		Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
		log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)
		testRunID = testNamespace.Name

		DeferCleanup(func() {
			By("Delete test Namespace")
			Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Setup manager")
		mgr, err := manager.New(restConfig, manager.Options{
			Scheme:  scheme,
			Metrics: metricsserver.Options{BindAddress: "0"},
			Cache: cache.Options{
				ByObject: map[client.Object]cache.ByObject{
					&operatorv1alpha1.Garden{}: {
						Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
					},
				},
			},
			Client: client.Options{
				Cache: &client.CacheOptions{
					DisableFor: []client.Object{
						&corev1.Secret{}, // applied because of operations on managed resources and their secrets
					},
				},
			},
			Controller: controllerconfig.Controller{
				SkipNameValidation: ptr.To(true),
			},
		})
		Expect(err).NotTo(HaveOccurred())
		mgrClient = mgr.GetClient()

		// The controller waits for the operation annotation to be removed from certain resources, so we need to add a
		// reconciler for it since envtest does not run the responsible controller (e.g. etcd-druid).
		Expect((&operationannotation.Reconciler{ForObject: func() client.Object { return &druidcorev1alpha1.Etcd{} }}).AddToManager(mgr)).To(Succeed())
		Expect((&operationannotation.Reconciler{ForObject: func() client.Object { return &extensionsv1alpha1.Extension{} }}).AddToManager(mgr)).To(Succeed())

		Expect((&namespacefinalizer.Reconciler{}).AddToManager(mgr)).To(Succeed())

		By("Register controller")
		extensionType = "test-extension"
		extensionTypeBeforeKAS = "test-extension-before-kube-api-server"
		extensionTypeAfterWorker = "test-extension-after-worker"

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "garden-" + testRunID,
				Labels: map[string]string{testID: testRunID},
			},
			Spec: operatorv1alpha1.GardenSpec{
				Extensions: []operatorv1alpha1.GardenExtension{
					{Type: extensionType},
					{Type: extensionTypeBeforeKAS},
					{Type: extensionTypeAfterWorker},
				},
				RuntimeCluster: operatorv1alpha1.RuntimeCluster{
					Networking: operatorv1alpha1.RuntimeNetworking{
						Pods:     []string{"10.1.0.0/16"},
						Services: []string{"10.2.0.0/16"},
					},
					Ingress: operatorv1alpha1.Ingress{
						Domains: []operatorv1alpha1.DNSDomain{{Name: "ingress.runtime-garden.local.gardener.cloud"}},
						Controller: gardencorev1beta1.IngressController{
							Kind: "nginx",
						},
					},
					Provider: operatorv1alpha1.Provider{
						Zones: []string{"a", "b", "c"},
					},
					Settings: &operatorv1alpha1.Settings{
						LoadBalancerServices: &operatorv1alpha1.SettingLoadBalancerServices{
							Annotations: loadBalancerServiceAnnotations,
						},
						VerticalPodAutoscaler: &operatorv1alpha1.SettingVerticalPodAutoscaler{
							Enabled: ptr.To(true),
						},
					},
				},
				VirtualCluster: operatorv1alpha1.VirtualCluster{
					DNS: operatorv1alpha1.DNS{
						Domains: []operatorv1alpha1.DNSDomain{{Name: "virtual-garden.local.gardener.cloud"}},
					},
					Gardener: operatorv1alpha1.Gardener{
						ClusterIdentity: "test",
						Dashboard: &operatorv1alpha1.GardenerDashboardConfig{
							Terminal: &operatorv1alpha1.DashboardTerminal{
								Container: operatorv1alpha1.DashboardTerminalContainer{Image: "busybox:latest"},
							},
						},
					},
					Kubernetes: operatorv1alpha1.Kubernetes{
						KubeAPIServer: &operatorv1alpha1.KubeAPIServerConfig{
							SNI: &operatorv1alpha1.SNI{
								DomainPatterns: []string{"api.garden.local-test.gardener.cloud"},
							},
						},
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

		gardenClientMap := fakeclientmap.NewClientMapBuilder().WithRuntimeClientForKey(keys.ForGarden(garden), mgr.GetClient(), mgr.GetConfig()).Build()

		Expect((&gardencontroller.Reconciler{
			Config: operatorconfigv1alpha1.OperatorConfiguration{
				Controllers: operatorconfigv1alpha1.ControllerConfiguration{
					Garden: operatorconfigv1alpha1.GardenControllerConfig{
						ConcurrentSyncs: ptr.To(5),
						SyncPeriod:      &metav1.Duration{Duration: time.Minute},
						ETCDConfig: &gardenletconfigv1alpha1.ETCDConfig{
							ETCDController:      &gardenletconfigv1alpha1.ETCDController{Workers: ptr.To[int64](5)},
							CustodianController: &gardenletconfigv1alpha1.CustodianController{Workers: ptr.To[int64](5)},
							BackupCompactionController: &gardenletconfigv1alpha1.BackupCompactionController{
								EnableBackupCompaction: ptr.To(false),
								Workers:                ptr.To[int64](5),
								EventsThreshold:        ptr.To[int64](100),
							},
						},
					},
				},
			},
			Identity:        &gardencorev1beta1.Gardener{Name: "test-gardener"},
			GardenNamespace: testNamespace.Name,
		}).AddToManager(mgr, gardenClientMap)).To(Succeed())

		By("Start manager")
		mgrContext, mgrCancel := context.WithCancel(ctx)

		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(mgrContext)).To(Succeed())
		}()

		DeferCleanup(func() {
			By("Stop manager")
			mgrCancel()
		})

		// When the gardener-{apiserver,admission-controller} deployments are not found, the Garden controller will
		// trigger another reconciliation to enable the SeedAuthorizer feature. Since gardener-resource-manager does not
		// run in this test to create them, let's create them manually here to prevent the controller from looping
		// endlessly. We create them before the Garden resource to prevent that the test runs into a timeout.
		// The gardener-system-public namespace is also expected to be created by gardener-resource-manager, we create
		// the namespace to prevent controller failing to deploy the gardener-info configmap due to non-existing namespace.
		By("Create gardener-{apiserver,admission-controller} deployments to prevent infinite reconciliation loops")
		gardenerAPIServerDeployment := newDeployment("gardener-apiserver", testNamespace.Name)
		gardenerAdmissionControllerDeployment := newDeployment("gardener-admission-controller", testNamespace.Name)
		gardenerSystemPublicNamespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "gardener-system-public"}}
		Expect(testClient.Create(ctx, gardenerAPIServerDeployment)).To(Succeed())
		Expect(testClient.Create(ctx, gardenerAdmissionControllerDeployment)).To(Succeed())
		Expect(testClient.Create(ctx, &gardenerSystemPublicNamespace)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, gardenerAPIServerDeployment)).To(Or(Succeed(), BeNotFoundError()))
			Expect(testClient.Delete(ctx, gardenerAdmissionControllerDeployment)).To(Or(Succeed(), BeNotFoundError()))
			Expect(testClient.Delete(ctx, &gardenerSystemPublicNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create Extension", func() {
			extension = &operatorv1alpha1.Extension{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testRunID,
					Namespace: testNamespace.Name,
				},
				Spec: operatorv1alpha1.ExtensionSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "Extension", Type: extensionType},
						{Kind: "Extension", Type: extensionTypeBeforeKAS, Lifecycle: &gardencorev1beta1.ControllerResourceLifecycle{Reconcile: ptr.To[gardencorev1beta1.ControllerResourceLifecycleStrategy]("BeforeKubeAPIServer")}},
						{Kind: "Extension", Type: extensionTypeAfterWorker, Lifecycle: &gardencorev1beta1.ControllerResourceLifecycle{Reconcile: ptr.To[gardencorev1beta1.ControllerResourceLifecycleStrategy]("AfterWorker")}},
					},
				},
			}

			extensionManagedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "extension-" + extension.Name + "-garden",
					Namespace: testNamespace.Name,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs: []corev1.LocalObjectReference{
						{Name: "extension-" + extension.Name + "-garden"},
					},
				},
			}

			Expect(testClient.Create(ctx, extension)).To(Succeed())
			Expect(testClient.Create(ctx, extensionManagedResource)).To(Succeed())

			log.Info("Created Extension for test", "extension", client.ObjectKeyFromObject(extension))
		})

		DeferCleanup(func() {
			By("Delete Extension")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, extension))).To(Succeed())

			By("Ensure Garden is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)
			}).Should(BeNotFoundError())
		})

		By("Create wildcard certificate secret")
		Expect(testClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dummy-garden-wildcard-cert",
				Namespace: testNamespace.Name,
				Labels: map[string]string{
					"gardener.cloud/role": "garden-cert",
				},
			},
			StringData: map[string]string{
				"tls.crt": "dummy-cert",
				"tls.key": "dummy-key",
			},
		})).To(Succeed())

		By("Create Garden")
		Expect(testClient.Create(ctx, garden)).To(Succeed())
		log.Info("Created Garden for test", "garden", garden.Name)

		DeferCleanup(func() {
			By("Delete Garden")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, garden))).To(Succeed())

			By("Forcefully remove finalizers")
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, garden))).To(Succeed())

			By("Ensure Garden is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)
			}).Should(BeNotFoundError())
		})

		By("Create Gardenlets")
		gardenletWithAutoUpdate, err := kubernetes.NewManifestReader([]byte(`apiVersion: seedmanagement.gardener.cloud/v1alpha1
kind: Gardenlet
metadata:
  name: ` + gardenletNameWithAutoUpdate + `
  namespace: ` + testNamespace.Name + `
  labels:
    operator.gardener.cloud/auto-update-gardenlet-helm-chart-ref: "true"
spec:
  deployment:
    helm:
      ociRepository:
        repository: please-update
        tag: me
`)).Read()
		Expect(err).NotTo(HaveOccurred())

		gardenletWithoutAutoUpdate, err := kubernetes.NewManifestReader([]byte(`apiVersion: seedmanagement.gardener.cloud/v1alpha1
kind: Gardenlet
metadata:
  name: ` + gardenletNameWithoutAutoUpdate + `
  namespace: ` + testNamespace.Name + `
spec:
  deployment:
    helm:
      ociRepository:
        ref: ` + noAutoUpdateRef + `
`)).Read()
		Expect(err).NotTo(HaveOccurred())

		Expect(testClient.Create(ctx, gardenletWithAutoUpdate)).To(Succeed())
		Expect(testClient.Create(ctx, gardenletWithoutAutoUpdate)).To(Succeed())

		DeferCleanup(func() {
			By("Delete Gardenlets")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, gardenletWithAutoUpdate))).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, gardenletWithoutAutoUpdate))).To(Succeed())

			By("Ensure Gardenlets are gone")
			Eventually(func(g Gomega) {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(gardenletWithAutoUpdate), gardenletWithAutoUpdate)).To(BeNotFoundError())
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(gardenletWithoutAutoUpdate), gardenletWithoutAutoUpdate)).To(BeNotFoundError())
			}).Should(Succeed())
		})
	})

	It("should successfully reconcile and delete a Garden", func() {
		By("Wait for Garden to have finalizer")
		Eventually(func(g Gomega) []string {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
			return garden.Finalizers
		}).Should(ConsistOf("gardener.cloud/operator"))

		By("Wait for last operation state to be set to Progressing")
		Eventually(func(g Gomega) gardencorev1beta1.LastOperationState {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
			if garden.Status.LastOperation == nil {
				return ""
			}
			return garden.Status.LastOperation.State
		}).Should(Equal(gardencorev1beta1.LastOperationStateProcessing))
		Expect(garden.Status.Gardener).NotTo(BeNil())

		By("Verify that the custom resource definitions have been created")
		Eventually(func(g Gomega) []string {
			crdList := &apiextensionsv1.CustomResourceDefinitionList{}
			g.Expect(testClient.List(ctx, crdList)).To(Succeed())
			return test.ObjectNames(crdList)
		}).WithTimeout(kubernetesutils.WaitTimeout).Should(ContainElements(
			"etcds.druid.gardener.cloud",
			"etcdcopybackupstasks.druid.gardener.cloud",
			"managedresources.resources.gardener.cloud",
			"verticalpodautoscalers.autoscaling.k8s.io",
			"verticalpodautoscalercheckpoints.autoscaling.k8s.io",
			"authorizationpolicies.security.istio.io",
			"destinationrules.networking.istio.io",
			"envoyfilters.networking.istio.io",
			"gateways.networking.istio.io",
			"peerauthentications.security.istio.io",
			"proxyconfigs.networking.istio.io",
			"requestauthentications.security.istio.io",
			"serviceentries.networking.istio.io",
			"sidecars.networking.istio.io",
			"telemetries.telemetry.istio.io",
			"virtualservices.networking.istio.io",
			"wasmplugins.extensions.istio.io",
			"workloadentries.networking.istio.io",
			"workloadgroups.networking.istio.io",
			// fluent-operator
			"clusterfilters.fluentbit.fluent.io",
			"clusterfluentbitconfigs.fluentbit.fluent.io",
			"clusterinputs.fluentbit.fluent.io",
			"clusteroutputs.fluentbit.fluent.io",
			"clusterparsers.fluentbit.fluent.io",
			"fluentbits.fluentbit.fluent.io",
			"collectors.fluentbit.fluent.io",
			"fluentbitconfigs.fluentbit.fluent.io",
			"filters.fluentbit.fluent.io",
			"parsers.fluentbit.fluent.io",
			"outputs.fluentbit.fluent.io",
			// prometheus-operator
			"alertmanagerconfigs.monitoring.coreos.com",
			"alertmanagers.monitoring.coreos.com",
			"podmonitors.monitoring.coreos.com",
			"probes.monitoring.coreos.com",
			"prometheusagents.monitoring.coreos.com",
			"prometheuses.monitoring.coreos.com",
			"prometheusrules.monitoring.coreos.com",
			"scrapeconfigs.monitoring.coreos.com",
			"servicemonitors.monitoring.coreos.com",
			"thanosrulers.monitoring.coreos.com",
			// extensions
			"backupbuckets.extensions.gardener.cloud",
			"dnsrecords.extensions.gardener.cloud",
			"extensions.extensions.gardener.cloud",
			// perses-operator
			"perses.perses.dev",
			"persesdashboards.perses.dev",
			"persesdatasources.perses.dev",
		))

		// The garden controller waits for the gardener-resource-manager Deployment to be healthy, so let's fake this here.
		By("Patch gardener-resource-manager deployment to report healthiness")
		Eventually(func(g Gomega) {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			patch := client.MergeFrom(deployment.DeepCopy())
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}
			g.Expect(testClient.Status().Patch(ctx, deployment, patch)).To(Succeed())
		}).Should(Succeed())

		By("Patch extension managed resource")
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionManagedResource), extensionManagedResource)).To(Succeed())
		extensionManagedResource.Status.ObservedGeneration = extensionManagedResource.Generation
		extensionManagedResource.Status.Conditions = []gardencorev1beta1.Condition{
			{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
			{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
			{Type: resourcesv1alpha1.ResourcesProgressing, Status: gardencorev1beta1.ConditionFalse, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
		}
		Expect(testClient.Status().Update(ctx, extensionManagedResource)).To(Succeed())

		By("Verify and patch extension before kube-api-server")
		patchExtensionStatus(testClient, extensionTypeBeforeKAS, testNamespace.Name, gardencorev1beta1.LastOperationStateSucceeded)

		By("Verify that garden runtime CA secret was generated")
		Eventually(func(g Gomega) []corev1.Secret {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name), client.MatchingLabels{"name": "ca-garden-runtime", "managed-by": "secrets-manager", "manager-identity": "gardener-operator"})).To(Succeed())
			return secretList.Items
		}).Should(HaveLen(1))

		By("Verify that garden namespace was labeled and annotated appropriately")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(testNamespace), testNamespace)).To(Succeed())
			g.Expect(testNamespace.Labels).To(HaveKeyWithValue("pod-security.kubernetes.io/enforce", "privileged"))
			g.Expect(testNamespace.Labels).To(HaveKeyWithValue("high-availability-config.resources.gardener.cloud/consider", "true"))
			g.Expect(testNamespace.Annotations).To(HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "a,b,c"))
		}).Should(Succeed())

		By("Verify that garden has generic token kubeconfig annotation")
		Eventually(func(g Gomega) map[string]string {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
			return garden.Annotations
		}).Should(HaveKey("generic-token-kubeconfig.secret.gardener.cloud/name"))

		By("Verify that VPA was created for gardener-operator")
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKey{Name: "gardener-operator-vpa", Namespace: testNamespace.Name}, &vpaautoscalingv1.VerticalPodAutoscaler{})
		}).Should(Succeed())

		By("Verify that ServiceMonitor was created for gardener-operator")
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKey{Name: "garden-gardener-operator", Namespace: testNamespace.Name}, &monitoringv1.ServiceMonitor{})
		}).Should(Succeed())

		By("Verify that the ManagedResources related to runtime components have been deployed")
		Eventually(func(g Gomega) []string {
			managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
			g.Expect(testClient.List(ctx, managedResourceList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return test.ObjectNames(managedResourceList)
		}).Should(ContainElements(
			"garden-system",
			"vpa",
			"etcd-druid",
			"nginx-ingress",
			"fluent-operator",
			"fluent-bit",
			"fluent-operator-custom-resources-garden",
			"vali",
			"plutono",
			"prometheus-operator",
			"alertmanager-garden",
			"perses-operator",
		))

		// The garden controller waits for the Istio ManagedResources to be healthy, but Istio is not really running in
		// this test, so let's fake this here.
		By("Patch Istio ManagedResources to report healthiness")
		for _, name := range []string{"istio-system", "virtual-garden-istio"} {
			Eventually(makeManagedResourceHealthy(name, "istio-system")).Should(Succeed())
		}

		// The garden controller waits for the etcd-druid ManagedResources to be healthy, but it is not really running
		// in this test, so let's fake this here.
		By("Patch etcd-druid ManagedResources to report healthiness")
		Eventually(makeManagedResourceHealthy("etcd-druid", testNamespace.Name)).Should(Succeed())

		By("Verify that the virtual garden control plane components have been deployed")
		Eventually(func(g Gomega) []string {
			etcdList := &druidcorev1alpha1.EtcdList{}
			g.Expect(testClient.List(ctx, etcdList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return test.ObjectNames(etcdList)
		}).Should(ConsistOf(
			"virtual-garden-etcd-main",
			"virtual-garden-etcd-events",
		))

		Eventually(func(g Gomega) map[string]string {
			service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "virtual-garden-kube-apiserver", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(service), service)).To(Succeed())
			return service.Annotations
		}).Should(Equal(map[string]string{
			"networking.istio.io/exportTo": "*",
			"networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":443}]`,
			"networking.resources.gardener.cloud/namespace-selectors":                          `[{"matchLabels":{"gardener.cloud/role":"istio-ingress"}},{"matchLabels":{"networking.gardener.cloud/access-target-apiserver":"allowed"}}]`,
		}))

		// The garden controller waits for the Etcd resources to be healthy, but etcd-druid is not really running in
		// this test, so let's fake this here.
		By("Patch Etcd resources to report healthiness")
		Eventually(func(g Gomega) {
			for _, suffix := range []string{"main", "events"} {
				etcd := &druidcorev1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Name: "virtual-garden-etcd-" + suffix, Namespace: testNamespace.Name}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(etcd), etcd)).To(Succeed(), "for "+etcd.Name)

				patch := client.MergeFrom(etcd.DeepCopy())
				etcd.Status.ObservedGeneration = &etcd.Generation
				etcd.Status.Conditions = []druidcorev1alpha1.Condition{{Type: druidcorev1alpha1.ConditionTypeAllMembersUpdated, Status: druidcorev1alpha1.ConditionTrue, LastUpdateTime: metav1.Now(), LastTransitionTime: metav1.Now()}}
				etcd.Status.Ready = ptr.To(true)
				g.Expect(testClient.Status().Patch(ctx, etcd, patch)).To(Succeed(), "for "+etcd.Name)
			}
		}).Should(Succeed())

		// The garden controller waits for the istio-ingress Service resource to be ready, but there is
		// no service controller or GRM running in this test which would make it ready, so let's fake this here.
		By("Create and patch istio-ingress Service resource to report readiness")
		var istioService *corev1.Service
		Eventually(func(g Gomega) {
			istioService = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istio-ingressgateway",
					Namespace: "virtual-garden-istio-ingress",
				},
				Spec: corev1.ServiceSpec{
					Type:  corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{{Protocol: corev1.ProtocolTCP, Port: 443}},
				},
			}
			g.Expect(testClient.Create(ctx, istioService)).To(Succeed())

			patch := client.MergeFrom(istioService.DeepCopy())
			istioService.Status.LoadBalancer.Ingress = append(istioService.Status.LoadBalancer.Ingress, corev1.LoadBalancerIngress{Hostname: "localhost"})
			g.Expect(testClient.Status().Patch(ctx, istioService, patch)).To(Succeed())
		}).Should(Succeed())

		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, istioService)).To(Succeed())
		})

		// The garden controller waits for the virtual-garden-kube-apiserver Deployment to be healthy, so let's fake
		// this here.
		By("Patch virtual-garden-kube-apiserver deployment to report healthiness")
		Eventually(func(g Gomega) []string {
			deploymentList := &appsv1.DeploymentList{}
			g.Expect(testClient.List(ctx, deploymentList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return test.ObjectNames(deploymentList)
		}).Should(ContainElements(
			"virtual-garden-kube-apiserver",
		))

		Eventually(func(g Gomega) {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "virtual-garden-kube-apiserver", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			podList := &corev1.PodList{}
			g.Expect(testClient.List(ctx, podList, client.InNamespace(testNamespace.Name), client.MatchingLabels(kubeapiserver.GetLabels()))).To(Succeed())

			if desiredReplicas := int(ptr.Deref(deployment.Spec.Replicas, 1)); len(podList.Items) != desiredReplicas {
				g.Expect(testClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(testNamespace.Name), client.MatchingLabels(kubeapiserver.GetLabels()))).To(Succeed())
				for i := 0; i < desiredReplicas; i++ {
					g.Expect(testClient.Create(ctx, &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      fmt.Sprintf("virtual-garden-kube-apiserver-%d", i),
							Namespace: testNamespace.Name,
							Labels:    kubeapiserver.GetLabels(),
						},
						Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "app"}}},
					})).To(Succeed(), fmt.Sprintf("create virtual-garden-kube-apiserver pod number %d", i))
				}
			}

			patch := client.MergeFrom(deployment.DeepCopy())
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Status.Conditions = []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue},
				{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionTrue, Reason: "NewReplicaSetAvailable"},
			}
			g.Expect(testClient.Status().Patch(ctx, deployment, patch)).To(Succeed())
		}).Should(Succeed())

		By("Bootstrapping virtual-garden-gardener-resource-manager")
		Eventually(func(g Gomega) []string {
			deploymentList := &appsv1.DeploymentList{}
			g.Expect(testClient.List(ctx, deploymentList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return test.ObjectNames(deploymentList)
		}).Should(ContainElements(
			"virtual-garden-gardener-resource-manager",
		))

		// The secret with the bootstrap certificate indicates that the bootstrapping of virtual-garden-gardener-resource-manager started.
		Eventually(func(g Gomega) []string {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return test.ObjectNames(secretList)
		}).Should(ContainElements(
			ContainSubstring("shoot-access-gardener-resource-manager-bootstrap-"),
		))

		// virtual-garden-gardener-resource manager usually sets the token-renew-timestamp when it reconciled the secret.
		// It is not running here, so we have to patch the secret by ourselves.
		Eventually(func(g Gomega) {
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "shoot-access-gardener-resource-manager", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())

			patch := client.MergeFrom(secret.DeepCopy())
			secret.Annotations["serviceaccount.resources.gardener.cloud/token-renew-timestamp"] = "2999-01-01T00:00:00Z"
			g.Expect(testClient.Patch(ctx, secret, patch)).To(Succeed())
		}).Should(Succeed())

		Eventually(func(g Gomega) []string {
			managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
			g.Expect(testClient.List(ctx, managedResourceList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return test.ObjectNames(managedResourceList)
		}).Should(ContainElements(
			"shoot-core-gardener-resource-manager",
		))

		// The garden controller waits for the shoot-core-gardener-resource-manager ManagedResource to be healthy, but virtual-garden-gardener-resource-manager is not really running in
		// this test, so let's fake this here.
		By("Patch shoot-core-gardener-resource-manager ManagedResource to report healthiness")
		Eventually(makeManagedResourceHealthy("shoot-core-gardener-resource-manager", testNamespace.Name)).Should(Succeed())

		// The secret with the bootstrap certificate should be gone when virtual-garden-gardener-resource-manager was bootstrapped.
		Eventually(func(g Gomega) []string {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return test.ObjectNames(secretList)
		}).ShouldNot(ContainElements(
			ContainSubstring("shoot-access-gardener-resource-manager-bootstrap-"),
		))

		// The garden controller waits for the virtual-garden-gardener-resource-manager Deployment to be healthy, so let's fake this here.
		By("Patch virtual-garden-gardener-resource-manager deployment to report healthiness")
		Eventually(func(g Gomega) {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "virtual-garden-gardener-resource-manager", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			// Don't patch bootstrapping deployment but wait for final deployment
			g.Expect(deployment.Spec.Template.Spec.Volumes).ShouldNot(ContainElements(
				MatchFields(IgnoreExtras, Fields{"Name": Equal("kubeconfig-bootstrap")}),
			))

			patch := client.MergeFrom(deployment.DeepCopy())
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}
			g.Expect(testClient.Status().Patch(ctx, deployment, patch)).To(Succeed())
		}).Should(Succeed())

		By("Patch gardener-internal kubeconfig secret to add the token usually added by virtual-garden-gardener-resource-manager")
		Eventually(func(g Gomega) {
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "gardener-internal", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())

			kubeconfigRaw, ok := secret.Data["kubeconfig"]
			g.Expect(ok).To(BeTrue())

			existingKubeconfig := &clientcmdv1.Config{}
			_, _, err := clientcmdlatest.Codec.Decode(kubeconfigRaw, nil, existingKubeconfig)
			g.Expect(err).NotTo(HaveOccurred())

			existingKubeconfig.AuthInfos[0].AuthInfo.Token = "foobar"

			kubeconfigRaw, err = runtime.Encode(clientcmdlatest.Codec, existingKubeconfig)
			g.Expect(err).NotTo(HaveOccurred())

			patch := client.MergeFrom(secret.DeepCopy())
			secret.Data["kubeconfig"] = kubeconfigRaw
			g.Expect(testClient.Patch(ctx, secret, patch)).To(Succeed())
		}).Should(Succeed())

		// The garden controller waits for the shoot-core-gardeneraccess ManagedResource to be healthy, but virtual-garden-gardener-resource-manager is not really running in
		// this test, so let's fake this here.
		By("Patch shoot-core-gardeneraccess ManagedResource to report healthiness")
		Eventually(func(g Gomega) {
			mr := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: "shoot-core-gardeneraccess", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(Succeed())

			patch := client.MergeFrom(mr.DeepCopy())
			mr.Status.ObservedGeneration = mr.Generation
			mr.Status.Conditions = []gardencorev1beta1.Condition{
				{
					Type:               "ResourcesHealthy",
					Status:             "True",
					LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
					LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
				},
				{
					Type:               "ResourcesApplied",
					Status:             "True",
					LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
					LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
				},
			}
			g.Expect(testClient.Status().Patch(ctx, mr, patch)).To(Succeed())
		}).Should(Succeed())

		By("Ensure virtual-garden-kube-controller-manager was deployed")
		Eventually(func(g Gomega) []string {
			deploymentList := &appsv1.DeploymentList{}
			g.Expect(testClient.List(ctx, deploymentList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return test.ObjectNames(deploymentList)
		}).Should(ContainElements(
			"virtual-garden-kube-controller-manager",
		))

		// The garden controller waits for the virtual-garden-kube-controller-manager Deployment to be healthy, so let's fake this here.
		By("Patch virtual-garden-kube-controller-manager deployment to report healthiness")
		Eventually(func(g Gomega) {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "virtual-garden-kube-controller-manager", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			podList := &corev1.PodList{}
			g.Expect(testClient.List(ctx, podList, client.InNamespace(testNamespace.Name), client.MatchingLabels(map[string]string{"app": "kubernetes", "role": "controller-manager"}))).To(Succeed())

			if desiredReplicas := int(ptr.Deref(deployment.Spec.Replicas, 1)); len(podList.Items) != desiredReplicas {
				g.Expect(testClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(testNamespace.Name), client.MatchingLabels(map[string]string{"app": "kubernetes", "role": "controller-manager"}))).To(Succeed())
				for i := 0; i < desiredReplicas; i++ {
					g.Expect(testClient.Create(ctx, &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      fmt.Sprintf("virtual-garden-kube-controller-manager-%d", i),
							Namespace: testNamespace.Name,
							Labels:    map[string]string{"app": "kubernetes", "role": "controller-manager"},
						},
						Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "app"}}},
					})).To(Succeed(), fmt.Sprintf("create virtual-garden-kube-apiserver pod number %d", i))
				}
			}

			patch := client.MergeFrom(deployment.DeepCopy())
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Status.Conditions = []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue},
				{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionTrue, Reason: "NewReplicaSetAvailable"},
			}
			g.Expect(testClient.Status().Patch(ctx, deployment, patch)).To(Succeed())
		}).Should(Succeed())

		By("Create gardener-apiserver Service in runtime cluster")
		// The garden controller requires the existence of the `gardener-apiserver` Service in the runtime cluster (in
		// reality, this is created asynchronously by gardener-resource-manager which is not running in this test).
		// Hence, let's manually create it to satisfy the reconciliation flow.
		gardenerAPIServerService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-apiserver",
				Namespace: testNamespace.Name,
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Ports:    []corev1.ServicePort{{Port: 443, TargetPort: intstr.FromInt32(443)}},
				Selector: map[string]string{"foo": "bar"},
			},
		}
		Expect(testClient.Create(ctx, gardenerAPIServerService)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, gardenerAPIServerService)).To(Or(Succeed(), BeNotFoundError()))
		})

		for _, name := range []string{"apiserver", "admission-controller", "controller-manager", "scheduler", "dashboard"} {
			By("Verify that the ManagedResources related to gardener-" + name + " have been deployed")
			Eventually(func(g Gomega) []string {
				managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
				g.Expect(testClient.List(ctx, managedResourceList, client.InNamespace(testNamespace.Name))).To(Succeed())
				return test.ObjectNames(managedResourceList)
			}).Should(ContainElements(
				"gardener-"+name+"-runtime",
				"gardener-"+name+"-virtual",
			), "for gardener-"+name)

			// The garden controller waits for the Gardener-related ManagedResources to be healthy, but no
			// gardener-resource-manager is running in this test, so let's fake this here.
			By("Patch gardener-" + name + "-related ManagedResources to report healthiness")
			Eventually(makeManagedResourceHealthy("gardener-"+name+"-runtime", testNamespace.Name)).Should(Succeed(), "for gardener-"+name)
			Eventually(makeManagedResourceHealthy("gardener-"+name+"-virtual", testNamespace.Name)).Should(Succeed(), "for gardener-"+name)
		}

		By("Verify that the ManagedResources related to other components have been deployed")
		Eventually(func(g Gomega) []string {
			managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
			g.Expect(testClient.List(ctx, managedResourceList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return test.ObjectNames(managedResourceList)
		}).Should(ContainElements(
			"terminal-runtime",
			"terminal-virtual",
			"garden-system-virtual",
			"kube-state-metrics-runtime",
			"gardener-metrics-exporter-runtime",
			"gardener-metrics-exporter-virtual",
			"prometheus-garden",
			"prometheus-longterm",
			"blackbox-exporter",
		))

		By("Verify and patch extensions")
		patchExtensionStatus(testClient, extensionType, testNamespace.Name, gardencorev1beta1.LastOperationStateSucceeded)
		patchExtensionStatus(testClient, extensionTypeAfterWorker, testNamespace.Name, gardencorev1beta1.LastOperationStateSucceeded)

		By("Wait for last operation state to be set to Succeeded")
		Eventually(func(g Gomega) gardencorev1beta1.LastOperationState {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
			if garden.Status.LastOperation == nil {
				return ""
			}
			return garden.Status.LastOperation.State
		}).Should(Equal(gardencorev1beta1.LastOperationStateSucceeded))

		By("Ensure relevant Gardenlet resources get auto-updated")
		Eventually(func(g Gomega) gardencorev1.OCIRepository {
			gardenlet := &seedmanagementv1alpha1.Gardenlet{ObjectMeta: metav1.ObjectMeta{Name: gardenletNameWithAutoUpdate, Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), gardenlet)).To(Succeed())
			return gardenlet.Spec.Deployment.Helm.OCIRepository
		}).Should(Equal(gardencorev1.OCIRepository{Ref: ptr.To("europe-docker.pkg.dev/gardener-project/releases/charts/gardener/gardenlet:v0.0.0-master+$Format:%H$")}))

		Consistently(func(g Gomega) gardencorev1.OCIRepository {
			gardenlet := &seedmanagementv1alpha1.Gardenlet{ObjectMeta: metav1.ObjectMeta{Name: gardenletNameWithoutAutoUpdate, Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), gardenlet)).To(Succeed())
			return gardenlet.Spec.Deployment.Helm.OCIRepository
		}).Should(Equal(gardencorev1.OCIRepository{Ref: &noAutoUpdateRef}))

		By("Delete Garden")
		Expect(testClient.Delete(ctx, garden)).To(Succeed())

		By("Delete Extension ManagedResource")
		Expect(testClient.Delete(ctx, extensionManagedResource)).To(Succeed())

		By("Verify that the virtual garden control plane components have been deleted")
		Eventually(func(g Gomega) []string {
			deploymentList := &appsv1.DeploymentList{}
			g.Expect(testClient.List(ctx, deploymentList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return test.ObjectNames(deploymentList)
		}).ShouldNot(ContainElements(
			"virtual-garden-kube-apiserver",
			"virtual-garden-kube-controller-manager",
			"virtual-garden-gardener-resource-manager",
		))

		Eventually(func(g Gomega) []string {
			etcdList := &druidcorev1alpha1.EtcdList{}
			g.Expect(testClient.List(ctx, etcdList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return test.ObjectNames(etcdList)
		}).ShouldNot(ContainElements(
			"virtual-garden-etcd-main",
			"virtual-garden-etcd-events",
		))

		By("Verify that the garden system components have been deleted")
		// When the controller succeeds then it deletes the `ManagedResource` CRD, so we only need to ensure here that
		// the `ManagedResource` API is no longer available.
		Eventually(func() error {
			return testClient.List(ctx, &resourcesv1alpha1.ManagedResourceList{}, client.InNamespace(testNamespace.Name))
		}).Should(BeNotFoundError())

		By("Verify that the custom resource definitions have been deleted")
		Eventually(func(g Gomega) []string {
			crdList := &apiextensionsv1.CustomResourceDefinitionList{}
			g.Expect(testClient.List(ctx, crdList)).To(Succeed())
			return test.ObjectNames(crdList)
		}).ShouldNot(ContainElements(
			"etcds.druid.gardener.cloud",
			"etcdcopybackupstasks.druid.gardener.cloud",
			"managedresources.resources.gardener.cloud",
			"verticalpodautoscalers.autoscaling.k8s.io",
			"verticalpodautoscalercheckpoints.autoscaling.k8s.io",
			"authorizationpolicies.security.istio.io",
			"destinationrules.networking.istio.io",
			"envoyfilters.networking.istio.io",
			"gateways.networking.istio.io",
			"peerauthentications.security.istio.io",
			"proxyconfigs.networking.istio.io",
			"requestauthentications.security.istio.io",
			"serviceentries.networking.istio.io",
			"sidecars.networking.istio.io",
			"telemetries.telemetry.istio.io",
			"virtualservices.networking.istio.io",
			"wasmplugins.extensions.istio.io",
			"workloadentries.networking.istio.io",
			"workloadgroups.networking.istio.io",
			// fluent-operator
			"clusterfilters.fluentbit.fluent.io",
			"clusterfluentbitconfigs.fluentbit.fluent.io",
			"clusterinputs.fluentbit.fluent.io",
			"clusteroutputs.fluentbit.fluent.io",
			"clusterparsers.fluentbit.fluent.io",
			"fluentbits.fluentbit.fluent.io",
			"collectors.fluentbit.fluent.io",
			"fluentbitconfigs.fluentbit.fluent.io",
			"filters.fluentbit.fluent.io",
			"parsers.fluentbit.fluent.io",
			"outputs.fluentbit.fluent.io",
			// prometheus-operator
			"alertmanagerconfigs.monitoring.coreos.com",
			"alertmanagers.monitoring.coreos.com",
			"podmonitors.monitoring.coreos.com",
			"probes.monitoring.coreos.com",
			"prometheusagents.monitoring.coreos.com",
			"prometheuses.monitoring.coreos.com",
			"prometheusrules.monitoring.coreos.com",
			"scrapeconfigs.monitoring.coreos.com",
			"servicemonitors.monitoring.coreos.com",
			"thanosrulers.monitoring.coreos.com",
		))

		By("Verify that gardener-resource-manager has been deleted")
		Eventually(func() error {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: testNamespace.Name}}
			return testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
		}).Should(BeNotFoundError())

		By("Verify that secrets have been deleted")
		Eventually(func(g Gomega) []corev1.Secret {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name), client.MatchingLabels{"managed-by": "secrets-manager", "manager-identity": "gardener-operator"})).To(Succeed())
			return secretList.Items
		}).Should(BeEmpty())

		By("Verify that garbage-collectable resources have been deleted")
		Eventually(func(g Gomega) {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name), client.MatchingLabels{"resources.gardener.cloud/garbage-collectable-reference": "true"})).To(Succeed())
			g.Expect(secretList.Items).To(BeEmpty())

			configMapList := &corev1.ConfigMapList{}
			g.Expect(testClient.List(ctx, configMapList, client.InNamespace(testNamespace.Name), client.MatchingLabels{"resources.gardener.cloud/garbage-collectable-reference": "true"})).To(Succeed())
			g.Expect(configMapList.Items).To(BeEmpty())
		}).Should(Succeed())

		By("Ensure Garden is gone")
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)
		}).Should(BeNotFoundError())
	})
})

func untilInTest(_ context.Context, _ time.Duration, _ retry.Func) error {
	return nil
}

func makeManagedResourceHealthy(name, namespace string) func(Gomega) {
	return func(g Gomega) {
		mr := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(Succeed(), fmt.Sprintf("for %s/%s", namespace, name))

		patch := client.MergeFrom(mr.DeepCopy())
		mr.Status.ObservedGeneration = mr.Generation
		mr.Status.Conditions = []gardencorev1beta1.Condition{
			{
				Type:               "ResourcesHealthy",
				Status:             "True",
				LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
				LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
			},
			{
				Type:               "ResourcesApplied",
				Status:             "True",
				LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
				LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
			},
			{
				Type:               "ResourcesProgressing",
				Status:             "False",
				LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
				LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
			},
		}
		g.Expect(testClient.Status().Patch(ctx, mr, patch)).To(Succeed(), fmt.Sprintf("for %s/%s", namespace, name))
	}
}

func newDeployment(name, namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
			Replicas: ptr.To[int32](1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "foo-container",
						Image: "foo",
					}},
				},
			},
		},
	}
}

func patchExtensionStatus(cl client.Client, name, namespace string, lastOp gardencorev1beta1.LastOperationState) {
	var ext = &extensionsv1alpha1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "garden-" + name,
			Namespace: namespace,
		},
	}

	EventuallyWithOffset(1, func() error {
		return mgrClient.Get(ctx, client.ObjectKeyFromObject(ext), ext)
	}).Should(Succeed())

	patch := client.MergeFrom(ext.DeepCopy())
	ExpectWithOffset(1, testClient.Patch(ctx, ext, patch)).To(Succeed())

	patch = client.MergeFrom(ext.DeepCopy())
	ext.Status = extensionsv1alpha1.ExtensionStatus{
		DefaultStatus: extensionsv1alpha1.DefaultStatus{
			LastOperation: &gardencorev1beta1.LastOperation{
				LastUpdateTime: metav1.NewTime(time.Date(9999, time.January, 1, 0, 0, 0, 0, time.UTC)),
				State:          lastOp,
			},
			ObservedGeneration: ext.Generation,
		},
	}
	ExpectWithOffset(1, cl.Status().Patch(ctx, ext, patch)).To(Succeed())
}
