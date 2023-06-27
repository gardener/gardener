package kubeletupgrade_test

import (
	"context"
	"path"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/kubeletupgrade"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	"github.com/gardener/gardener/pkg/nodeagent/registry"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var _ = Describe("Nodeagent kubeletupgrade controller tests", func() {
	var (
		testRunID                string
		testFs                   afero.Fs
		kubeletPath              string
		controllerTriggerChannel chan event.GenericEvent
		nodeAgentConfig          *nodeagentv1alpha1.NodeAgentConfiguration
		fakeExtractor            *registry.FakeRegistryExtractor
		fakeDbus                 *dbus.FakeDbus
	)
	const (
		contractHyperkubeKubeletName = "kubelet"
	)

	BeforeEach(func() {
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

		By("Register controller")
		testFs = afero.NewMemMapFs()
		nodeAgentConfig = &nodeagentv1alpha1.NodeAgentConfiguration{
			TokenSecretName: v1alpha1.NodeAgentTokenSecretName,
			HyperkubeImage:  images.ImageNameHyperkube,
		}
		configBytes, err := yaml.Marshal(nodeAgentConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(afero.WriteFile(testFs, nodeagentv1alpha1.NodeAgentConfigPath, configBytes, 0644)).To(Succeed())

		kubeletPath = "/opt/bin/kubelet"

		controllerTriggerChannel = make(chan event.GenericEvent)
		fakeExtractor = &registry.FakeRegistryExtractor{}
		fakeDbus = &dbus.FakeDbus{}

		kubeletReconciler := &kubeletupgrade.Reconciler{
			Client:           mgr.GetClient(),
			Fs:               testFs,
			Config:           nodeAgentConfig,
			TriggerChannel:   controllerTriggerChannel,
			Extractor:        fakeExtractor,
			Dbus:             fakeDbus,
			TargetBinaryPath: kubeletPath,
		}
		Expect((kubeletReconciler.AddToManager(mgr))).To(Succeed())

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

	It("should update and restart kubelet when channel triggered", func() {
		controllerTriggerChannel <- event.GenericEvent{}

		Eventually(func(g Gomega) error {
			g.Expect(fakeExtractor.Extractions).To(HaveLen(1))
			actual := fakeExtractor.Extractions[0]
			g.Expect(actual.Image).To(Equal(nodeAgentConfig.HyperkubeImage))
			g.Expect(actual.PathSuffix).To(Equal(contractHyperkubeKubeletName))
			g.Expect(actual.Dest).To(Equal(kubeletPath))

			g.Expect(fakeDbus.Actions).To(HaveLen(1))
			g.Expect(fakeDbus.Actions[0].Action).To(Equal(dbus.FakeRestart))
			g.Expect(fakeDbus.Actions[0].UnitNames).To(Equal([]string{kubelet.UnitName}))
			return nil
		}).Should(Succeed())
	})

	It("should skip update and not restart kubelet when channel not triggered", func() {
		hyperkubeImageDownloadedPath := path.Join(nodeagentv1alpha1.NodeAgentBaseDir, "hyperkube-downloaded")

		Expect(afero.WriteFile(testFs, hyperkubeImageDownloadedPath, []byte(nodeAgentConfig.HyperkubeImage), 0644)).Should(Succeed())

		controllerTriggerChannel <- event.GenericEvent{}

		Consistently(func(g Gomega) error {
			g.Expect(fakeDbus.Actions).To(HaveLen(0))
			return nil
		}).Should(Succeed())
	})
})

func untilInTest(_ context.Context, _ time.Duration, _ retry.Func) error {
	return nil
}
