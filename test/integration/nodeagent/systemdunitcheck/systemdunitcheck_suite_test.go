// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package systemdunitcheck_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/nodeagent/controller/systemdunitcheck"
	fakedbus "github.com/gardener/gardener/pkg/nodeagent/dbus/fake"
)

func TestSystemdUnitCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration NodeAgent SystemdUnitCheck Suite")
}

const testID = "systemdunitcheck-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *envtest.Environment
	testClient client.Client

	fakeFS    afero.Afero
	fakeDBus  *fakedbus.DBus
	fakeClock *testclock.FakeClock
	oscCodec  runtime.Codec
	nodeName  string
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("Start test environment")
	testEnv = &envtest.Environment{}

	var err error
	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("Create test client")
	testClient, err = client.New(restConfig, client.Options{})
	Expect(err).NotTo(HaveOccurred())

	fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}
	fakeDBus = fakedbus.New()
	fakeClock = testclock.NewFakeClock(time.Now())

	scheme := runtime.NewScheme()
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))
	oscCodec = serializer.NewCodecFactory(scheme).LegacyCodec(extensionsv1alpha1.SchemeGroupVersion)

	nodeName = testID

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			DefaultLabelSelector: client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(labels.Set{testID: testID})},
		},
		Controller: config.Controller{
			SkipNameValidation: ptr.To(true),
		},
	})
	Expect(err).NotTo(HaveOccurred())

	By("Register controller")
	Expect((&systemdunitcheck.Reconciler{
		Client: mgr.GetClient(),
		DBus:   fakeDBus,
		Clock:  fakeClock,
		FS:     fakeFS,
		Config: nodeagentconfigv1alpha1.SystemdUnitCheckControllerConfig{
			SyncPeriod:     &metav1.Duration{Duration: time.Second},
			StuckThreshold: &metav1.Duration{Duration: 5 * time.Minute},
		},
	}).AddToManager(mgr, predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetName() == nodeName
	}))).To(Succeed())

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
