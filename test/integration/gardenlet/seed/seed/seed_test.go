// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/pkg/api/indexer"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/autoscaling/vpa"
	"github.com/gardener/gardener/pkg/component/extensions/dnsrecord"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	"github.com/gardener/gardener/pkg/component/networking/istio"
	"github.com/gardener/gardener/pkg/component/networking/nginxingress"
	"github.com/gardener/gardener/pkg/component/observability/logging/fluentoperator"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheusoperator"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	seedcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/seed/seed"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/test/utils/namespacefinalizer"
)

var _ = Describe("Seed controller tests", func() {
	var (
		testRunID          string
		testNamespace      *corev1.Namespace
		seedName           string
		seed               *gardencorev1beta1.Seed
		seedControllerInst *gardencorev1beta1.ControllerInstallation
		identity           = &gardencorev1beta1.Gardener{Version: "1.2.3"}
	)

	BeforeEach(func() {
		By("Create test Namespace")
		testNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "garden-",
			},
		}
		Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
		log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)
		testRunID = testNamespace.Name
		seedName = "seed-" + testRunID

		DeferCleanup(func() {
			By("Delete test Namespace")
			Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Setup manager")
		httpClient, err := rest.HTTPClientFor(restConfig)
		Expect(err).NotTo(HaveOccurred())
		mapper, err := apiutil.NewDynamicRESTMapper(restConfig, httpClient)
		Expect(err).NotTo(HaveOccurred())

		mgr, err := manager.New(restConfig, manager.Options{
			Scheme:  testScheme,
			Metrics: metricsserver.Options{BindAddress: "0"},
			Cache: cache.Options{
				Mapper: mapper,
				ByObject: map[client.Object]cache.ByObject{
					&gardencorev1beta1.Seed{}: {
						Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
					},
					&operatorv1alpha1.Garden{}: {
						Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
					},
				},
			},
			Controller: controllerconfig.Controller{
				SkipNameValidation: ptr.To(true),
			},
		})
		Expect(err).NotTo(HaveOccurred())
		mgrClient = mgr.GetClient()

		// We create the seed namespace in the garden and delete it after every test, so let's ensure it gets finalized.
		Expect((&namespacefinalizer.Reconciler{}).AddToManager(mgr)).To(Succeed())

		By("Setup field indexes")
		Expect(indexer.AddBackupBucketSeedName(ctx, mgr.GetFieldIndexer())).To(Succeed())
		Expect(indexer.AddControllerInstallationSeedRefName(ctx, mgr.GetFieldIndexer())).To(Succeed())
		Expect(indexer.AddShootSeedName(ctx, mgr.GetFieldIndexer())).To(Succeed())

		By("Create test clientset")
		testClientSet, err = kubernetes.NewWithConfig(
			kubernetes.WithRESTConfig(mgr.GetConfig()),
			kubernetes.WithRuntimeAPIReader(mgr.GetAPIReader()),
			kubernetes.WithRuntimeClient(mgr.GetClient()),
			kubernetes.WithRuntimeCache(mgr.GetCache()),
		)
		Expect(err).NotTo(HaveOccurred())

		By("Register controller")
		Expect((&seedcontroller.Reconciler{
			SeedClientSet: testClientSet,
			Config: gardenletconfigv1alpha1.GardenletConfiguration{
				Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
					Seed: &gardenletconfigv1alpha1.SeedControllerConfiguration{
						// This controller is pretty heavy-weight, so use a higher duration.
						SyncPeriod: &metav1.Duration{Duration: time.Minute},
					},
				},
				SNI: &gardenletconfigv1alpha1.SNI{
					Ingress: &gardenletconfigv1alpha1.SNIIngress{
						Namespace: ptr.To(testNamespace.Name + "-istio"),
					},
				},
				Logging: &gardenletconfigv1alpha1.Logging{
					Enabled: ptr.To(true),
					Vali: &gardenletconfigv1alpha1.Vali{
						Enabled: ptr.To(true),
					},
				},
				ETCDConfig: &gardenletconfigv1alpha1.ETCDConfig{
					BackupCompactionController: &gardenletconfigv1alpha1.BackupCompactionController{
						EnableBackupCompaction: ptr.To(false),
						EventsThreshold:        ptr.To[int64](1),
						Workers:                ptr.To[int64](1),
					},
					CustodianController: &gardenletconfigv1alpha1.CustodianController{
						Workers: ptr.To[int64](1),
					},
					ETCDController: &gardenletconfigv1alpha1.ETCDController{
						Workers: ptr.To[int64](1),
					},
				},
				SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
					SeedTemplate: gardencorev1beta1.SeedTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Name: seedName,
						},
					},
				},
			},
			Identity:        identity,
			GardenNamespace: testNamespace.Name,
		}).AddToManager(mgr, mgr)).To(Succeed())

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
	})

	JustBeforeEach(func() {
		DeferCleanup(test.WithVars(
			&secretsutils.GenerateKey, secretsutils.FakeGenerateKey,
			&resourcemanager.SkipWebhookDeployment, true,
		))

		By("Create DNS provider secret in garden namespace")
		dnsProviderSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			GenerateName: "secret-",
			Namespace:    testNamespace.Name,
			Labels: map[string]string{
				testID: testRunID,
			},
		}}
		Expect(testClient.Create(ctx, dnsProviderSecret)).To(Succeed())

		By("Wait until the manager cache observes the DNS provider secret")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(dnsProviderSecret), dnsProviderSecret)
		}).Should(Succeed())

		provider := "providerType"

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name:   seedName,
				Labels: map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Region: "region",
					Type:   provider,
					Zones:  []string{"a", "b", "c"},
				},
				Networks: gardencorev1beta1.SeedNetworks{
					Pods:     "10.0.0.0/16",
					Services: "10.1.0.0/16",
					Nodes:    ptr.To("10.2.0.0/16"),
				},
				Ingress: &gardencorev1beta1.Ingress{
					Domain: "someingress.example.com",
					Controller: gardencorev1beta1.IngressController{
						Kind: "nginx",
					},
				},
				DNS: gardencorev1beta1.SeedDNS{
					Provider: &gardencorev1beta1.SeedDNSProvider{
						Type: provider,
						SecretRef: corev1.SecretReference{
							Name:      dnsProviderSecret.Name,
							Namespace: dnsProviderSecret.Namespace,
						},
					},
				},
			},
		}

		controllerRegistration := &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ctrlreg-" + testID + "-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{Kind: extensionsv1alpha1.DNSRecordResource, Type: provider},
					{Kind: extensionsv1alpha1.ControlPlaneResource, Type: provider},
					{Kind: extensionsv1alpha1.InfrastructureResource, Type: provider},
					{Kind: extensionsv1alpha1.WorkerResource, Type: provider},
				},
			},
		}

		By("Create Seed")
		Expect(testClient.Create(ctx, seed)).To(Succeed())
		log.Info("Created Seed for test", "seed", seed.Name)

		By("Create ControllerRegistration")
		Expect(testClient.Create(ctx, controllerRegistration)).To(Succeed())
		log.Info("Created ControllerRegistration for test", "controllerRegistration", controllerRegistration.Name)

		seedControllerInst = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ctrlinst-" + testID + "-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				RegistrationRef: corev1.ObjectReference{
					Name: controllerRegistration.Name,
				},
				SeedRef: corev1.ObjectReference{
					Name: seedName,
				},
			},
		}

		By("Create ControllerInstallation")
		Expect(testClient.Create(ctx, seedControllerInst)).To(Succeed())
		log.Info("Created ControllerInstallation for test", "seedControllerInst", seedControllerInst.Name)

		By("Patch ControllerInstallation")
		patch := client.MergeFrom(seedControllerInst.DeepCopy())
		seedControllerInst.Status.Conditions = []gardencorev1beta1.Condition{
			{Type: gardencorev1beta1.ControllerInstallationInstalled, Status: gardencorev1beta1.ConditionTrue},
			{Type: gardencorev1beta1.ControllerInstallationHealthy, Status: gardencorev1beta1.ConditionTrue},
			{Type: gardencorev1beta1.ControllerInstallationProgressing, Status: gardencorev1beta1.ConditionFalse},
		}
		Expect(testClient.Status().Patch(ctx, seedControllerInst, patch)).To(Succeed())

		DeferCleanup(func() {
			By("Delete Seed")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, seed))).To(Succeed())

			By("Delete ControllerInstallation")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, seedControllerInst))).To(Succeed())

			By("Forcefully remove finalizers")
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, seed))).To(Succeed())

			By("Ensure Seed is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)
			}).Should(BeNotFoundError())

			By("Delete ControllerRegistration")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, controllerRegistration))).To(Succeed())

			By("Delete DNS provider secret in garden namespace")
			Expect(testClient.Delete(ctx, dnsProviderSecret)).To(Succeed())

			By("Cleanup all labels/annotations from test namespace")
			patch := client.MergeFrom(testNamespace)
			testNamespace.Annotations = nil
			testNamespace.Labels = nil
			Expect(testClient.Patch(ctx, testNamespace, patch)).To(Succeed())
		})
	})

	Context("when seed namespace does not exist", func() {
		It("should set the last operation to 'Error'", func() {
			By("Wait for 'last operation' state to be set to Error")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				g.Expect(seed.Status.LastOperation).NotTo(BeNil())
				g.Expect(seed.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
				g.Expect(seed.Status.LastOperation.Description).To(ContainSubstring("failed to get seed namespace in garden cluster"))
			}).Should(Succeed())
		})
	})

	Context("when seed namespace exists", func() {
		// Typically, GCM creates the seed-specific namespace, but it doesn't run in this test, hence we have to do it.
		var seedNamespace *corev1.Namespace

		JustBeforeEach(func() {
			By("Create seed namespace in garden")
			seedNamespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gardenerutils.ComputeGardenNamespace(seed.Name)}}
			Expect(testClient.Create(ctx, seedNamespace)).To(Succeed())

			By("Wait until the manager cache observes the seed namespace")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(seedNamespace), &corev1.Namespace{})
			}).Should(Succeed())

			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, seedNamespace)).To(Succeed())
			})

			By("Wait for Seed to have a cluster identity")
			Eventually(func(g Gomega) *string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				return seed.Status.ClusterIdentity
			}).ShouldNot(BeNil())
		})

		Context("when internal domain secret does not exist", func() {
			It("should set the last operation to 'Error'", func() {
				By("Wait for 'last operation' state to be set to Error")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
					g.Expect(seed.Status.LastOperation).NotTo(BeNil())
					g.Expect(seed.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
					g.Expect(seed.Status.LastOperation.Description).To(ContainSubstring("need an internal domain secret but found none"))
				}).Should(Succeed())
			})
		})

		Context("when internal domain secret exists", func() {
			JustBeforeEach(func() {
				DeferCleanup(
					test.WithVars(
						&resourcemanager.Until, untilInTest,
						&resourcemanager.TimeoutWaitForDeployment, 50*time.Millisecond,
						&nginxingress.WaitUntilHealthy, waitUntilHealthyInTest,
						&seedcontroller.WaitUntilLoadBalancerIsReady, waitUntilLoadBalancerIsReadyInTest,
						&dnsrecord.WaitUntilExtensionObjectReady, waitUntilExtensionObjectReadyInTest,
					),
				)

				By("Create internal domain secret in seed namespace")
				internalDomainSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "secret-",
					Namespace:    seedNamespace.Name,
					Labels: map[string]string{
						testID:                testRunID,
						"gardener.cloud/role": "internal-domain",
					},
					Annotations: map[string]string{
						"dns.gardener.cloud/provider": "test",
						"dns.gardener.cloud/domain":   "example.com",
					},
				}}
				Expect(testClient.Create(ctx, internalDomainSecret)).To(Succeed())

				By("Wait until the manager cache observes the internal domain secret")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(internalDomainSecret), internalDomainSecret)
				}).Should(Succeed())

				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, internalDomainSecret)).To(Succeed())
				})
			})

			Context("when global monitoring secret does not exist", func() {
				It("should set the last operation to 'Error'", func() {
					By("Wait for 'last operation' state to be set to Error")
					Eventually(func(g Gomega) {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
						g.Expect(seed.Status.LastOperation).NotTo(BeNil())
						g.Expect(seed.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
						g.Expect(seed.Status.LastOperation.Description).To(ContainSubstring("global monitoring secret not found in seed namespace"))
					}).Should(Succeed())
				})
			})

			Context("when global monitoring secret exists", func() {
				// Typically, GCM creates the global monitoring secret, but it doesn't run in this test, hence we have to do it.
				var globalMonitoringSecret *corev1.Secret

				JustBeforeEach(func() {
					By("Create global monitoring secret in seed namespace")
					globalMonitoringSecret = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName: "secret-",
							Namespace:    seedNamespace.Name,
							Labels: map[string]string{
								testID:                testRunID,
								"gardener.cloud/role": "global-monitoring",
							},
						},
						Data: map[string][]byte{"foo": []byte("bar")},
					}
					Expect(testClient.Create(ctx, globalMonitoringSecret)).To(Succeed())

					By("Wait until the manager cache observes the global monitoring secret")
					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(globalMonitoringSecret), globalMonitoringSecret)
					}).Should(Succeed())

					DeferCleanup(func() {
						Expect(testClient.Delete(ctx, globalMonitoringSecret)).To(Succeed())
					})
				})

				test := func(seedIsGarden bool) {
					By("Wait for Seed to have finalizer")
					Eventually(func(g Gomega) []string {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
						return seed.Finalizers
					}).Should(ConsistOf("gardener"))

					By("Wait for 'last operation' state to be set to Processing")
					Eventually(func(g Gomega) {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
						g.Expect(seed.Status.LastOperation).NotTo(BeNil())
						g.Expect(seed.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateProcessing))
					}).Should(Succeed())

					By("Verify that CA secret was generated")
					Eventually(func(g Gomega) []corev1.Secret {
						secretList := &corev1.SecretList{}
						g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name), client.MatchingLabels{"name": "ca-seed", "managed-by": "secrets-manager"})).To(Succeed())
						return secretList.Items
					}).Should(HaveLen(1))

					if !seedIsGarden {
						By("Verify that garden namespace was labeled and annotated appropriately")
						Eventually(func(g Gomega) {
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(testNamespace), testNamespace)).To(Succeed())
							g.Expect(testNamespace.Labels).To(And(
								HaveKeyWithValue("role", "garden"),
								HaveKeyWithValue("pod-security.kubernetes.io/enforce", "privileged"),
								HaveKeyWithValue("high-availability-config.resources.gardener.cloud/consider", "true"),
							))
							g.Expect(testNamespace.Annotations).To(And(
								HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "a,b,c"),
							))
						}).Should(Succeed())
					}

					By("Verify that kube-system namespace was labeled appropriately")
					Eventually(func(g Gomega) map[string]string {
						kubeSystemNamespace := &corev1.Namespace{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystemNamespace)).To(Succeed())
						return kubeSystemNamespace.Labels
					}).Should(HaveKeyWithValue("role", "kube-system"))

					By("Verify that global monitoring secret was replicated")
					Eventually(func(g Gomega) {
						secret := &corev1.Secret{}
						g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "seed-" + globalMonitoringSecret.Name, Namespace: testNamespace.Name}, secret)).To(Succeed())
						g.Expect(secret.Data).To(HaveKey("auth"))
					}).Should(Succeed())

					var (
						crdsOnlyForSeedClusters = []gomegatypes.GomegaMatcher{
							// machine-controller-manager
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("machineclasses.machine.sapcloud.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("machinedeployments.machine.sapcloud.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("machines.machine.sapcloud.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("machinesets.machine.sapcloud.io")})}),
							// extensions
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("backupentries.extensions.gardener.cloud")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("bastions.extensions.gardener.cloud")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusters.extensions.gardener.cloud")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("containerruntimes.extensions.gardener.cloud")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("controlplanes.extensions.gardener.cloud")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("infrastructures.extensions.gardener.cloud")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("networks.extensions.gardener.cloud")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("operatingsystemconfigs.extensions.gardener.cloud")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("workers.extensions.gardener.cloud")})}),
						}
						crdsSharedWithGardenCluster = []gomegatypes.GomegaMatcher{
							// extensions
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("backupbuckets.extensions.gardener.cloud")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("dnsrecords.extensions.gardener.cloud")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("extensions.extensions.gardener.cloud")})}),
							// etcd-druid
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcds.druid.gardener.cloud")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcdcopybackupstasks.druid.gardener.cloud")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("managedresources.resources.gardener.cloud")})}),
							// istio
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("destinationrules.networking.istio.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("envoyfilters.networking.istio.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gateways.networking.istio.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("serviceentries.networking.istio.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("sidecars.networking.istio.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtualservices.networking.istio.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("authorizationpolicies.security.istio.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("peerauthentications.security.istio.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("requestauthentications.security.istio.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("workloadentries.networking.istio.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("workloadgroups.networking.istio.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("telemetries.telemetry.istio.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("wasmplugins.extensions.istio.io")})}),
							// vertical-pod-autoscaler
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("verticalpodautoscalers.autoscaling.k8s.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("verticalpodautoscalercheckpoints.autoscaling.k8s.io")})}),
							// fluent-operator
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusterfilters.fluentbit.fluent.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusterfluentbitconfigs.fluentbit.fluent.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusterinputs.fluentbit.fluent.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusteroutputs.fluentbit.fluent.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusterparsers.fluentbit.fluent.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("fluentbits.fluentbit.fluent.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("collectors.fluentbit.fluent.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("fluentbitconfigs.fluentbit.fluent.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("filters.fluentbit.fluent.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("parsers.fluentbit.fluent.io")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("outputs.fluentbit.fluent.io")})}),
							// prometheus-operator
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("alertmanagerconfigs.monitoring.coreos.com")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("alertmanagers.monitoring.coreos.com")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("podmonitors.monitoring.coreos.com")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("probes.monitoring.coreos.com")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("prometheusagents.monitoring.coreos.com")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("prometheuses.monitoring.coreos.com")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("prometheusrules.monitoring.coreos.com")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("scrapeconfigs.monitoring.coreos.com")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("servicemonitors.monitoring.coreos.com")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("thanosrulers.monitoring.coreos.com")})}),
						}
					)

					By("Verify that the seed-specific CRDs have been deployed")
					Eventually(func(g Gomega) []apiextensionsv1.CustomResourceDefinition {
						crdList := &apiextensionsv1.CustomResourceDefinitionList{}
						g.Expect(testClient.List(ctx, crdList)).To(Succeed())
						return crdList.Items
					}).Should(ContainElements(crdsOnlyForSeedClusters))

					By("Verify that VPA was created for gardenlet")
					Eventually(func() error {
						return testClient.Get(ctx, client.ObjectKey{Name: "gardenlet-vpa", Namespace: testNamespace.Name}, &vpaautoscalingv1.VerticalPodAutoscaler{})
					}).Should(Succeed())

					if !seedIsGarden {
						By("Verify that the CRDs shared with the garden cluster have been deployed")
						Eventually(func(g Gomega) []apiextensionsv1.CustomResourceDefinition {
							crdList := &apiextensionsv1.CustomResourceDefinitionList{}
							g.Expect(testClient.List(ctx, crdList)).To(Succeed())
							return crdList.Items
						}).Should(ContainElements(crdsSharedWithGardenCluster))

						// The seed controller waits for the gardener-resource-manager Deployment to be healthy, so
						// let's fake this here.
						By("Patch gardener-resource-manager deployment to report healthiness")
						Eventually(func(g Gomega) {
							deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: testNamespace.Name}}
							g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

							patch := client.MergeFrom(deployment.DeepCopy())
							deployment.Status.ObservedGeneration = deployment.Generation
							deployment.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}
							g.Expect(testClient.Status().Patch(ctx, deployment, patch)).To(Succeed())
						}).Should(Succeed())
					} else {
						By("Verify that the CRDs shared with the garden cluster have not been deployed (gardener-operator deploys them)")
						Eventually(func(g Gomega) []apiextensionsv1.CustomResourceDefinition {
							crdList := &apiextensionsv1.CustomResourceDefinitionList{}
							g.Expect(testClient.List(ctx, crdList)).To(Succeed())
							return crdList.Items
						}).ShouldNot(ContainElements(crdsSharedWithGardenCluster))

						// Usually, the gardener-operator deploys and manages the following resources.
						// However, it is not really running, so we have to fake its behaviour here.
						By("Create resources managed by gardener-operator")
						chartApplier, err := kubernetes.NewChartApplierForConfig(restConfig)
						Expect(err).NotTo(HaveOccurred())

						var (
							applier                  = kubernetes.NewApplier(testClient, testClient.RESTMapper())
							managedResourceCRDReader = kubernetes.NewManifestReader([]byte(managedResourcesCRD))
							istioSystemNamespace     = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "istio-system"}}
							istioCRDs                = istio.NewCRD(chartApplier)
							vpaCRD                   = vpa.NewCRD(applier, nil)
							fluentCRD                = fluentoperator.NewCRDs(applier)
						)

						monitoringCRD, err := prometheusoperator.NewCRDs(testClient, applier)
						Expect(err).NotTo(HaveOccurred())

						Expect(applier.ApplyManifest(ctx, managedResourceCRDReader, kubernetes.DefaultMergeFuncs)).To(Succeed())
						Expect(testClient.Create(ctx, istioSystemNamespace)).To(Succeed())
						Expect(istioCRDs.Deploy(ctx)).To(Succeed())
						Expect(vpaCRD.Deploy(ctx)).To(Succeed())
						Expect(fluentCRD.Deploy(ctx)).To(Succeed())
						Expect(monitoringCRD.Deploy(ctx)).To(Succeed())

						DeferCleanup(func() {
							Expect(applier.DeleteManifest(ctx, managedResourceCRDReader)).To(Succeed())
							Expect(testClient.Delete(ctx, istioSystemNamespace)).To(Succeed())
							Eventually(func() error {
								return testClient.Get(ctx, client.ObjectKeyFromObject(istioSystemNamespace), istioSystemNamespace)
							}).Should(BeNotFoundError())
							Expect(istioCRDs.Destroy(ctx)).To(Succeed())
							Expect(vpaCRD.Destroy(ctx)).To(Succeed())
							Expect(fluentCRD.Destroy(ctx)).To(Succeed())
							Expect(monitoringCRD.Destroy(ctx)).To(Succeed())
						})
					}

					By("Verify that the seed system components have been deployed")
					expectedManagedResources := []gomegatypes.GomegaMatcher{
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("cluster-autoscaler")})}),
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("dependency-watchdog-weeder")})}),
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("dependency-watchdog-prober")})}),
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("machine-controller-manager")})}),
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("system")})}),
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("prometheus-cache")})}),
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("prometheus-seed")})}),
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("prometheus-aggregate")})}),
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-state-metrics-seed")})}),
					}

					if !seedIsGarden {
						expectedManagedResources = append(expectedManagedResources,
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("vpa")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-druid")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("nginx-ingress")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("plutono")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("vali")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("fluent-bit")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("fluent-operator")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("fluent-operator-custom-resources")})}),
							MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("prometheus-operator")})}),
						)
					}

					Eventually(func(g Gomega) []resourcesv1alpha1.ManagedResource {
						managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
						g.Expect(testClient.List(ctx, managedResourceList, client.InNamespace(testNamespace.Name))).To(Succeed())
						return managedResourceList.Items
					}).Should(ConsistOf(expectedManagedResources))

					expectedIstioManagedResources := []gomegatypes.GomegaMatcher{
						MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("istio")})}),
					}
					if !seedIsGarden {
						expectedIstioManagedResources = append(expectedIstioManagedResources, MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("istio-system")})}))
					}

					Eventually(func(g Gomega) []resourcesv1alpha1.ManagedResource {
						managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
						g.Expect(testClient.List(ctx, managedResourceList, client.InNamespace("istio-system"))).To(Succeed())
						return managedResourceList.Items
					}).Should(ConsistOf(expectedIstioManagedResources))

					By("Wait for 'last operation' state to be set to Succeeded")
					Eventually(func(g Gomega) {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
						g.Expect(seed.Status.LastOperation).NotTo(BeNil())
						g.Expect(seed.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					}).Should(Succeed())

					By("Delete Seed")
					Expect(testClient.Delete(ctx, seed)).To(Succeed())

					// The ControllerInstallation for the seed provider must be deleted manually because the
					// ControllerRegistration controller does not run in this test.
					By("Delete ControllerInstallation")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, seedControllerInst))).To(Succeed())

					if seedIsGarden {
						// The CRDs are cleaned up by the Destroy function of GRM. In case the seed is garden, the Destroy is called by the gardener-operator and since it's
						// not running in this test, we can safely assert the below-mentioned. But if the seed is not garden, it might so happen that, before we fetch the
						// ManagedResourceList and expect it to be empty, the CRDs are already gone. Since the gardener-resource-manager is deleted only after all the
						// managedresources are gone, we don't need to assert it separately.
						By("Verify that the seed system components have been deleted")
						Eventually(func(g Gomega) []resourcesv1alpha1.ManagedResource {
							managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
							g.Expect(testClient.List(ctx, managedResourceList, client.InNamespace(testNamespace.Name))).To(Succeed())
							return managedResourceList.Items
						}).Should(BeEmpty())
					} else {
						By("Verify that gardener-resource-manager has been deleted")
						Eventually(func() error {
							deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: testNamespace.Name}}
							return testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
						}).Should(BeNotFoundError())

						// We should wait for the CRD to be deleted since it is a cluster-scoped resource so that we do not interfere
						// with other test cases.
						By("Verify that CRD has been deleted")
						Eventually(func() error {
							return testClient.Get(ctx, client.ObjectKey{Name: "managedresources.resources.gardener.cloud"}, &apiextensionsv1.CustomResourceDefinition{})
						}).Should(BeNotFoundError())
					}

					By("Ensure Seed is gone")
					Eventually(func() error {
						return testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)
					}).Should(BeNotFoundError())
				}

				It("should properly maintain the last operation and deploy all seed system components", func() {
					test(false)
				})

				Context("when seed cluster is garden cluster at the same time", func() {
					BeforeEach(func() {
						By("Create Garden")
						garden := &operatorv1alpha1.Garden{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: "garden-",
								Labels: map[string]string{
									testID: testRunID,
								},
							},
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
										Version: "1.26.3",
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
						log.Info("Created Garden for test", "garden", garden.Name)

						By("Wait until the manager cache observes the garden")
						Eventually(func() error {
							return mgrClient.Get(ctx, client.ObjectKeyFromObject(garden), &operatorv1alpha1.Garden{})
						}).Should(Succeed())

						DeferCleanup(func() {
							By("Delete Garden")
							Expect(client.IgnoreNotFound(testClient.Delete(ctx, garden))).To(Succeed())

							By("Wait until the manager cache observes garden deletion")
							Eventually(func() error {
								return mgrClient.Get(ctx, client.ObjectKeyFromObject(garden), &operatorv1alpha1.Garden{})
							}).Should(BeNotFoundError())
						})
					})

					It("should not manage components managed by gardener-operator", func() {
						test(true)
					})
				})
			})
		})
	})
})

func untilInTest(_ context.Context, _ time.Duration, _ retry.Func) error {
	return nil
}

func waitUntilHealthyInTest(_ context.Context, _ client.Reader, _, _ string) error {
	return nil
}

func waitUntilLoadBalancerIsReadyInTest(_ context.Context, _ logr.Logger, _ client.Client, _, _ string, _ time.Duration) (string, error) {
	return "someingress.example.com", nil
}

func waitUntilExtensionObjectReadyInTest(_ context.Context, _ client.Client, _ logr.Logger, _ extensionsv1alpha1.Object, _ string, _, _, _ time.Duration, _ func() error) error {
	return nil
}
