package selfupgrade_test

import (
	"context"
	"path"

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
	"github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/selfupgrade"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	"github.com/gardener/gardener/pkg/nodeagent/registry"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils/images"
)

var _ = Describe("Nodeagent selfupgrade controller tests", func() {
	var (
		testRunID                string
		testFs                   afero.Fs
		nodeAgentPath            string
		controllerTriggerChannel chan event.GenericEvent
		nodeAgentConfig          *nodeagentv1alpha1.NodeAgentConfiguration
		fakeExtractor            *registry.FakeRegistryExtractor
		fakeDbus                 *dbus.FakeDbus
	)
	const (
		contractGardenerNodeAgentName = "gardener-node-agent"
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
			Image:           images.ImageNameGardenerNodeAgent,
		}
		configBytes, err := yaml.Marshal(nodeAgentConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(afero.WriteFile(testFs, nodeagentv1alpha1.NodeAgentConfigPath, configBytes, 0644)).To(Succeed())

		nodeAgentPath = "/usr/local/bin/gardener-node-agent"

		controllerTriggerChannel = make(chan event.GenericEvent)
		fakeExtractor = &registry.FakeRegistryExtractor{}
		fakeDbus = &dbus.FakeDbus{}

		selfupgradeReconciler := &selfupgrade.Reconciler{
			Client:         mgr.GetClient(),
			Fs:             testFs,
			Config:         nodeAgentConfig,
			TriggerChannel: controllerTriggerChannel,
			Extractor:      fakeExtractor,
			Dbus:           fakeDbus,
			SelfBinaryPath: nodeAgentPath,
		}
		Expect((selfupgradeReconciler.AddToManager(mgr))).To(Succeed())

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

	It("should update and restart gardener-node-agent when channel triggered", func() {
		controllerTriggerChannel <- event.GenericEvent{}

		Eventually(func(g Gomega) error {
			g.Expect(fakeExtractor.Extractions).To(HaveLen(1))
			actual := fakeExtractor.Extractions[0]
			g.Expect(actual.Image).To(Equal(nodeAgentConfig.Image))
			g.Expect(actual.PathSuffix).To(Equal(contractGardenerNodeAgentName))
			g.Expect(actual.Dest).To(Equal(nodeAgentPath))

			g.Expect(fakeDbus.Actions).To(HaveLen(1))
			g.Expect(fakeDbus.Actions[0].Action).To(Equal(dbus.FakeRestart))
			g.Expect(fakeDbus.Actions[0].UnitNames).To(Equal([]string{nodeagentv1alpha1.NodeAgentUnitName}))
			return nil
		}).Should(Succeed())
	})

	It("should skip update and not restart gardener-node-agent when channel not triggered", func() {
		imageDownloadedPath := path.Join(nodeagentv1alpha1.NodeAgentBaseDir, "node-agent-downloaded")

		Expect(afero.WriteFile(testFs, imageDownloadedPath, []byte(nodeAgentConfig.Image), 0644)).Should(Succeed())

		controllerTriggerChannel <- event.GenericEvent{}

		Consistently(func(g Gomega) error {
			g.Expect(fakeDbus.Actions).To(HaveLen(0))
			return nil
		}).Should(Succeed())
	})
})
