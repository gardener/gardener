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

package garden_test

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	gardenletconfig "github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserverexposure"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	"github.com/gardener/gardener/pkg/operation/botanist/component/shared"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	gardencontroller "github.com/gardener/gardener/pkg/operator/controller/garden"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/test/utils/operationannotation"
)

var _ = Describe("Garden controller tests", func() {
	var (
		loadBalancerServiceAnnotations = map[string]string{"foo": "bar"}
		garden                         *operatorv1alpha1.Garden
		testRunID                      string
		testNamespace                  *corev1.Namespace
	)

	BeforeEach(func() {
		DeferCleanup(test.WithVar(&secretsutils.GenerateKey, secretsutils.FakeGenerateKey))
		DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.HVPA, true))
		DeferCleanup(test.WithVars(
			&etcd.DefaultInterval, 100*time.Millisecond,
			&etcd.DefaultTimeout, 500*time.Millisecond,
			&kubeapiserverexposure.DefaultInterval, 100*time.Millisecond,
			&kubeapiserverexposure.DefaultTimeout, 500*time.Millisecond,
			&kubeapiserver.IntervalWaitForDeployment, 100*time.Millisecond,
			&kubeapiserver.TimeoutWaitForDeployment, 500*time.Millisecond,
			&resourcemanager.SkipWebhookDeployment, true,
			&resourcemanager.IntervalWaitForDeployment, 100*time.Millisecond,
			&resourcemanager.TimeoutWaitForDeployment, 500*time.Millisecond,
			&shared.IntervalWaitForGardenerResourceManagerBootstrapping, 500*time.Millisecond,
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
		mapper, err := apiutil.NewDynamicRESTMapper(restConfig)
		Expect(err).NotTo(HaveOccurred())

		mgr, err := manager.New(restConfig, manager.Options{
			Scheme:             operatorclient.RuntimeScheme,
			MetricsBindAddress: "0",
			NewCache: cache.BuilderWithOptions(cache.Options{
				Mapper: mapper,
				SelectorsByObject: map[client.Object]cache.ObjectSelector{
					&operatorv1alpha1.Garden{}: {
						Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
					},
				},
			}),
		})
		Expect(err).NotTo(HaveOccurred())
		mgrClient = mgr.GetClient()

		// The controller waits for the operation annotation to be removed from Etcd resources, so we need to add a
		// reconciler for it since envtest does not run the responsible controller (etcd-druid).
		Expect((&operationannotation.Reconciler{ForObject: func() client.Object { return &druidv1alpha1.Etcd{} }}).AddToManager(mgr)).To(Succeed())

		By("Register controller")
		chartsPath := filepath.Join("..", "..", "..", "..", charts.Path)
		imageVector, err := imagevector.ReadGlobalImageVectorWithEnvOverride(filepath.Join(chartsPath, "images.yaml"))
		Expect(err).NotTo(HaveOccurred())

		Expect((&gardencontroller.Reconciler{
			Config: config.OperatorConfiguration{
				Controllers: config.ControllerConfiguration{
					Garden: config.GardenControllerConfig{
						ConcurrentSyncs: pointer.Int(5),
						SyncPeriod:      &metav1.Duration{Duration: time.Minute},
						ETCDConfig: &gardenletconfig.ETCDConfig{
							ETCDController:      &gardenletconfig.ETCDController{Workers: pointer.Int64(5)},
							CustodianController: &gardenletconfig.CustodianController{Workers: pointer.Int64(5)},
							BackupCompactionController: &gardenletconfig.BackupCompactionController{
								EnableBackupCompaction: pointer.Bool(false),
								Workers:                pointer.Int64(5),
								EventsThreshold:        pointer.Int64(100),
							},
						},
					},
				},
			},
			ImageVector:     imageVector,
			Identity:        &gardencorev1beta1.Gardener{Name: "test-gardener"},
			GardenNamespace: testNamespace.Name,
		}).AddToManager(mgr)).To(Succeed())

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

		DeferCleanup(test.WithVar(&gardencontroller.NewClientFromSecretObject, func(secret *corev1.Secret, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
			Expect(secret.Name).To(Equal("gardener-internal"))
			Expect(secret.Namespace).To(Equal(testNamespace.Name))
			return kubernetesfake.NewClientSetBuilder().WithClient(testClient).Build(), nil
		}))

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "garden-" + testRunID,
				Labels: map[string]string{testID: testRunID},
			},
			Spec: operatorv1alpha1.GardenSpec{
				RuntimeCluster: operatorv1alpha1.RuntimeCluster{
					Networking: operatorv1alpha1.RuntimeNetworking{
						Pods:     "10.1.0.0/16",
						Services: "10.2.0.0/16",
					},
					Provider: operatorv1alpha1.Provider{
						Zones: []string{"a", "b", "c"},
					},
					Settings: &operatorv1alpha1.Settings{
						LoadBalancerServices: &operatorv1alpha1.SettingLoadBalancerServices{
							Annotations: loadBalancerServiceAnnotations,
						},
						VerticalPodAutoscaler: &operatorv1alpha1.SettingVerticalPodAutoscaler{
							Enabled: pointer.Bool(true),
						},
					},
				},
				VirtualCluster: operatorv1alpha1.VirtualCluster{
					DNS: operatorv1alpha1.DNS{
						Domain: "virtual-garden.local.gardener.cloud",
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
						Services: "100.64.0.0/13",
					},
				},
			},
		}

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
	})

	It("should properly maintain the Reconciled condition", func() {
		By("Wait for Garden to have finalizer")
		Eventually(func(g Gomega) []string {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
			return garden.Finalizers
		}).Should(ConsistOf("gardener.cloud/operator"))

		By("Wait for Reconciled condition to be set to Progressing")
		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
			return garden.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.GardenReconciled),
			WithStatus(gardencorev1beta1.ConditionProgressing),
		))
		Expect(garden.Status.Gardener).NotTo(BeNil())

		By("Verify that the custom resource definitions have been created")
		// When the controller succeeds then it deletes the `ManagedResource` CRD, so we only need to ensure here that
		// the `ManagedResource` API is no longer available.
		Eventually(func(g Gomega) []apiextensionsv1.CustomResourceDefinition {
			crdList := &apiextensionsv1.CustomResourceDefinitionList{}
			g.Expect(testClient.List(ctx, crdList)).To(Succeed())
			return crdList.Items
		}).Should(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("hvpas.autoscaling.k8s.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("verticalpodautoscalers.autoscaling.k8s.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("authorizationpolicies.security.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("destinationrules.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("envoyfilters.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gateways.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("peerauthentications.security.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("proxyconfigs.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("requestauthentications.security.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("serviceentries.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("sidecars.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("telemetries.telemetry.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtualservices.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("wasmplugins.extensions.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("workloadentries.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("workloadgroups.networking.istio.io")})}),
		))

		By("Verify that garden runtime CA secret was generated")
		Eventually(func(g Gomega) []corev1.Secret {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name), client.MatchingLabels{"name": "ca-garden-runtime", "managed-by": "secrets-manager", "manager-identity": "gardener-operator"})).To(Succeed())
			return secretList.Items
		}).Should(HaveLen(1))

		By("Verify that garden namespace was labeled and annotated appropriately")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(testNamespace), testNamespace)).To(Succeed())
			g.Expect(testNamespace.Labels).To(HaveKeyWithValue("high-availability-config.resources.gardener.cloud/consider", "true"))
			g.Expect(testNamespace.Annotations).To(HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "a,b,c"))
		}).Should(Succeed())

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

		By("Verify that the relevant ManagedResources have been deployed")
		Eventually(func(g Gomega) []resourcesv1alpha1.ManagedResource {
			managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
			g.Expect(testClient.List(ctx, managedResourceList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return managedResourceList.Items
		}).Should(ConsistOf(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("garden-system")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("vpa")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("hvpa")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-druid")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-state-metrics")})}),
		))

		// The garden controller waits for the Istio ManagedResources to be healthy, but Istio is not really running in
		// this test, so let's fake this here.
		By("Patch Istio ManagedResources to report healthiness")
		Eventually(func(g Gomega) {
			for _, name := range []string{"istio-system", "virtual-garden-istio"} {
				mr := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "istio-system"}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(Succeed(), "for "+mr.Name)

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
				g.Expect(testClient.Status().Patch(ctx, mr, patch)).To(Succeed(), "for "+mr.Name)
			}
		}).Should(Succeed())

		By("Verify that the virtual garden control plane components have been deployed")
		Eventually(func(g Gomega) []druidv1alpha1.Etcd {
			etcdList := &druidv1alpha1.EtcdList{}
			g.Expect(testClient.List(ctx, etcdList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return etcdList.Items
		}).Should(ConsistOf(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-etcd-main")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-etcd-events")})}),
		))

		Eventually(func(g Gomega) map[string]string {
			service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "virtual-garden-kube-apiserver", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(service), service)).To(Succeed())
			return service.Annotations
		}).Should(Equal(utils.MergeStringMaps(loadBalancerServiceAnnotations, map[string]string{
			"networking.resources.gardener.cloud/from-world-to-ports":            `[{"protocol":"TCP","port":443}]`,
			"networking.resources.gardener.cloud/from-policy-allowed-ports":      `[{"protocol":"TCP","port":443}]`,
			"networking.resources.gardener.cloud/from-policy-pod-label-selector": "all-scrape-targets",
			"networking.resources.gardener.cloud/namespace-selectors":            `[{"matchLabels":{"gardener.cloud/role":"istio-ingress"}}]`,
		})))

		// The garden controller waits for the Etcd resources to be healthy, but etcd-druid is not really running in
		// this test, so let's fake this here.
		By("Patch Etcd resources to report healthiness")
		Eventually(func(g Gomega) {
			for _, suffix := range []string{"main", "events"} {
				etcd := &druidv1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Name: "virtual-garden-etcd-" + suffix, Namespace: testNamespace.Name}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(etcd), etcd)).To(Succeed(), "for "+etcd.Name)

				patch := client.MergeFrom(etcd.DeepCopy())
				etcd.Status.ObservedGeneration = &etcd.Generation
				etcd.Status.Ready = pointer.Bool(true)
				g.Expect(testClient.Status().Patch(ctx, etcd, patch)).To(Succeed(), "for "+etcd.Name)
			}
		}).Should(Succeed())

		// The garden controller waits for the virtual-garden-kube-apiserver Service resource to be ready, but there is
		// no service controller running in this test which would make it ready, so let's fake this here.
		By("Patch virtual-garden-kube-apiserver Service resource to report readiness")
		Eventually(func(g Gomega) {
			service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "virtual-garden-kube-apiserver", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(service), service)).To(Succeed())

			patch := client.MergeFrom(service.DeepCopy())
			service.Status.LoadBalancer.Ingress = append(service.Status.LoadBalancer.Ingress, corev1.LoadBalancerIngress{Hostname: "localhost"})
			g.Expect(testClient.Status().Patch(ctx, service, patch)).To(Succeed())
		}).Should(Succeed())

		Eventually(func(g Gomega) []appsv1.Deployment {
			deploymentList := &appsv1.DeploymentList{}
			g.Expect(testClient.List(ctx, deploymentList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return deploymentList.Items
		}).Should(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-kube-apiserver")})}),
		))

		// The garden controller waits for the virtual-garden-kube-apiserver Deployment to be healthy, so let's fake
		// this here.
		By("Patch virtual-garden-kube-apiserver deployment to report healthiness")
		Eventually(func(g Gomega) {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "virtual-garden-kube-apiserver", Namespace: testNamespace.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			podList := &corev1.PodList{}
			g.Expect(testClient.List(ctx, podList, client.InNamespace(testNamespace.Name), client.MatchingLabels(kubeapiserver.GetLabels()))).To(Succeed())

			if desiredReplicas := int(pointer.Int32Deref(deployment.Spec.Replicas, 1)); len(podList.Items) != desiredReplicas {
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
		Eventually(func(g Gomega) []appsv1.Deployment {
			deploymentList := &appsv1.DeploymentList{}
			g.Expect(testClient.List(ctx, deploymentList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return deploymentList.Items
		}).Should(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-gardener-resource-manager")})}),
		))

		// The secret with the bootstrap certificate indicates that the bootstrapping of virtual-garden-gardener-resource-manager started.
		Eventually(func(g Gomega) []corev1.Secret {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return secretList.Items
		}).Should(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": ContainSubstring("shoot-access-gardener-resource-manager-bootstrap-")})}),
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

		Eventually(func(g Gomega) []resourcesv1alpha1.ManagedResource {
			managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
			g.Expect(testClient.List(ctx, managedResourceList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return managedResourceList.Items
		}).Should(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("shoot-core-gardener-resource-manager")})}),
		))

		// The garden controller waits for the shoot-core-gardener-resource-manager ManagedResources to be healthy, but virtual-garden-gardener-resource-manager is not really running in
		// this test, so let's fake this here.
		By("Patch shoot-core-gardener-resource-manager ManagedResources to report healthiness")
		Eventually(func(g Gomega) {
			mr := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: "shoot-core-gardener-resource-manager", Namespace: testNamespace.Name}}
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

		// The secret with the bootstrap certificate should be gone when virtual-garden-gardener-resource-manager was bootstrapped.
		Eventually(func(g Gomega) []corev1.Secret {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return secretList.Items
		}).ShouldNot(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": ContainSubstring("shoot-access-gardener-resource-manager-bootstrap-")})}),
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

			patch := client.MergeFrom(secret.DeepCopy())
			secret.Data = map[string][]byte{"kubeconfig": []byte(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: AAAA
    server: https://api-virtual-garden.local.gardener.cloud
  name: garden
contexts:
- context:
    cluster: garden
    user: garden
  name: garden
current-context: garden
kind: Config
preferences: {}
users:
- name: garden
  user:
    token: foobar
`)}
			g.Expect(testClient.Patch(ctx, secret, patch)).To(Succeed())
		}).Should(Succeed())

		By("Wait for Reconciled condition to be set to True")
		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
			return garden.Status.Conditions
		}).Should(ContainCondition(OfType(operatorv1alpha1.GardenReconciled), WithStatus(gardencorev1beta1.ConditionTrue)))

		By("Delete Garden")
		Expect(testClient.Delete(ctx, garden)).To(Succeed())

		By("Verify that the virtual garden control plane components have been deleted")
		Eventually(func(g Gomega) []appsv1.Deployment {
			deploymentList := &appsv1.DeploymentList{}
			g.Expect(testClient.List(ctx, deploymentList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return deploymentList.Items
		}).ShouldNot(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-kube-apiserver")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-gardener-resource-manager")})}),
		))

		Eventually(func(g Gomega) []druidv1alpha1.Etcd {
			etcdList := &druidv1alpha1.EtcdList{}
			g.Expect(testClient.List(ctx, etcdList, client.InNamespace(testNamespace.Name))).To(Succeed())
			return etcdList.Items
		}).ShouldNot(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-etcd-main")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtual-garden-etcd-events")})}),
		))

		By("Verify that the garden system components have been deleted")
		// When the controller succeeds then it deletes the `ManagedResource` CRD, so we only need to ensure here that
		// the `ManagedResource` API is no longer available.
		Eventually(func(g Gomega) error {
			return testClient.List(ctx, &resourcesv1alpha1.ManagedResourceList{}, client.InNamespace(testNamespace.Name))
		}).Should(BeNotFoundError())

		By("Verify that the custom resource definitions have been deleted")
		// When the controller succeeds then it deletes the `ManagedResource` CRD, so we only need to ensure here that
		// the `ManagedResource` API is no longer available.
		Eventually(func(g Gomega) []apiextensionsv1.CustomResourceDefinition {
			crdList := &apiextensionsv1.CustomResourceDefinitionList{}
			g.Expect(testClient.List(ctx, crdList)).To(Succeed())
			return crdList.Items
		}).ShouldNot(ContainElements(
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("hvpas.autoscaling.k8s.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcds.druid.gardener.cloud")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("authorizationpolicies.security.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("destinationrules.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("envoyfilters.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("gateways.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("peerauthentications.security.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("proxyconfigs.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("requestauthentications.security.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("serviceentries.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("sidecars.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("telemetries.telemetry.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("virtualservices.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("wasmplugins.extensions.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("workloadentries.networking.istio.io")})}),
			MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("workloadgroups.networking.istio.io")})}),
		))

		By("Verify that gardener-resource-manager has been deleted")
		Eventually(func(g Gomega) error {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: testNamespace.Name}}
			return testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
		}).Should(BeNotFoundError())

		By("Verify that secrets have been deleted")
		Eventually(func(g Gomega) []corev1.Secret {
			secretList := &corev1.SecretList{}
			g.Expect(testClient.List(ctx, secretList, client.InNamespace(testNamespace.Name), client.MatchingLabels{"managed-by": "secrets-manager", "manager-identity": "gardener-operator"})).To(Succeed())
			return secretList.Items
		}).Should(BeEmpty())

		By("Ensure Garden is gone")
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)
		}).Should(BeNotFoundError())
	})
})
