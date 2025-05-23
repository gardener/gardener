// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package project_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/cloudprofile"
	"github.com/gardener/gardener/pkg/controllermanager/controller/namespacedcloudprofile"
	"github.com/gardener/gardener/pkg/controllermanager/controller/project/project"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/test"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
	"github.com/gardener/gardener/test/utils/namespacefinalizer"
)

func TestProject(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration ControllerManager Project Main Suite")
}

const testID = "project-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *gardenerenvtest.GardenerTestEnvironment
	testClient client.Client
	mgrClient  client.Reader
	testRunID  string

	defaultResourceQuota corev1.ResourceQuota
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,ExtensionValidator,ShootQuotaValidator,ShootValidator,ShootTolerationRestriction,ShootDNS"},
		},
	}

	var err error
	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("Create test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: kubernetes.GardenScheme})
	Expect(err).NotTo(HaveOccurred())

	testRunID = utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
	log.Info("Using test run ID for test", "testRunID", testRunID)

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  kubernetes.GardenScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&gardencorev1beta1.Project{}: {
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

	// The project controller waits for namespaces to be gone, so we need to finalize them as envtest doesn't run the
	// namespace controller.
	Expect((&namespacefinalizer.Reconciler{}).AddToManager(mgr)).To(Succeed())

	By("Setup field indexes")
	Expect(indexer.AddProjectNamespace(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddNamespacedCloudProfileParentRefName(ctx, mgr.GetFieldIndexer())).To(Succeed())

	By("Register Project controller")
	defaultResourceQuota = corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      map[string]string{"foo": testRunID},
			Annotations: map[string]string{"foo": testRunID},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				"count/shoots.core.gardener.cloud": resource.MustParse("100"),
			},
		},
	}

	Expect((&project.Reconciler{
		Config: controllermanagerconfigv1alpha1.ProjectControllerConfiguration{
			ConcurrentSyncs: ptr.To(5),
			Quotas: []controllermanagerconfigv1alpha1.QuotaConfiguration{{
				Config:          defaultResourceQuota,
				ProjectSelector: &metav1.LabelSelector{},
			}},
		},
		// limit exponential backoff in tests
		RateLimiter: workqueue.NewTypedWithMaxWaitRateLimiter(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](), 100*time.Millisecond),
	}).AddToManager(mgr)).To(Succeed())

	// The registration of the CloudProfile and NamespacedCloudProfile controllers allows for the validation of
	// project member authorizations to interact with NamespacedCloudProfiles within a Project.
	By("Register CloudProfile controller")
	Expect((&cloudprofile.Reconciler{
		Config: controllermanagerconfigv1alpha1.CloudProfileControllerConfiguration{
			ConcurrentSyncs: ptr.To(5),
		},
	}).AddToManager(mgr)).To(Succeed())

	By("Register NamespacedCloudProfile controller")
	Expect((&namespacedcloudprofile.Reconciler{
		Config: controllermanagerconfigv1alpha1.NamespacedCloudProfileControllerConfiguration{
			ConcurrentSyncs: ptr.To(5),
		},
	}).AddToManager(mgr)).To(Succeed())

	DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, true))

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
