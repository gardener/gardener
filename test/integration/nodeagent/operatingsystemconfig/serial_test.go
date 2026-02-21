// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/pkg/api/indexer"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	fakecontainerdclient "github.com/gardener/gardener/pkg/nodeagent/containerd/fake"
	"github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
	fakedbus "github.com/gardener/gardener/pkg/nodeagent/dbus/fake"
	fakeregistry "github.com/gardener/gardener/pkg/nodeagent/registry/fake"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("OperatingSystemConfig controller serial reconciliation tests", func() {
	var (
		fakeDBus *fakedbus.DBus
		fakeFS   afero.Afero

		oscSecretName = testRunID

		operatingSystemConfig *extensionsv1alpha1.OperatingSystemConfig
		oscRaw                []byte
		oscSecret             *corev1.Secret

		imageMountDirectory                string
		pathBootstrapTokenFile             = filepath.Join("/", "var", "lib", "gardener-node-agent", "credentials", "bootstrap-token")
		pathKubeletBootstrapKubeconfigFile = filepath.Join("/", "var", "lib", "kubelet", "kubeconfig-bootstrap")

		hostName1, hostName2, hostName3 = "host1", "host2", "host3"
		remainingHosts                  = func(leaders ...string) []string {
			var (
				allHosts  = sets.New(hostName1, hostName2, hostName3)
				leaderSet = sets.New(leaders...).Intersection(allHosts)
			)

			return sets.List(allHosts.Difference(leaderSet))
		}
	)

	BeforeEach(func() {
		fakeDBus = fakedbus.New()
		fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}
		mgrClient = testClient

		var err error
		imageMountDirectory, err = fakeFS.TempDir("", "fake-node-agent-")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(fakeFS.RemoveAll(imageMountDirectory)).To(Succeed()) })

		DeferCleanup(test.WithVars(
			&operatingsystemconfig.RequeueAfterRestart, time.Second,
			&operatingsystemconfig.Exec, func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return []byte(""), nil
			},
		))

		operatingSystemConfig = &extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Files: []extensionsv1alpha1.File{
					{
						Path:        "/example/file/" + hostName1,
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "data"}},
						Permissions: ptr.To[uint32](0777),
						HostName:    &hostName1,
					},
					{
						Path:        "/example/file/" + hostName2,
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "data"}},
						Permissions: ptr.To[uint32](0777),
						HostName:    &hostName2,
					},
					{
						Path:        "/example/file/" + hostName3,
						Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "data"}},
						Permissions: ptr.To[uint32](0777),
						HostName:    &hostName3,
					},
				},
			},
		}

		oscRaw, err = runtime.Encode(codec, operatingSystemConfig)
		Expect(err).NotTo(HaveOccurred())

		oscSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      oscSecretName,
				Namespace: metav1.NamespaceSystem,
				Labels:    map[string]string{testID: testRunID},
				Annotations: map[string]string{
					"checksum/data-script":                                utils.ComputeSHA256Hex(oscRaw),
					"reconciliation.osc.node-agent.gardener.cloud/serial": "true",
				},
			},
			Data: map[string][]byte{"osc.yaml": oscRaw},
		}

		By("Create bootstrap token file")
		_, err = fakeFS.Create(pathBootstrapTokenFile)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			Expect(fakeFS.Remove(pathBootstrapTokenFile)).To(Or(Succeed(), MatchError(afero.ErrFileNotFound)))
		})

		By("Create kubelet bootstrap kubeconfig file")
		_, err = fakeFS.Create(pathKubeletBootstrapKubeconfigFile)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			Expect(fakeFS.Remove(pathKubeletBootstrapKubeconfigFile)).To(Or(Succeed(), MatchError(afero.ErrFileNotFound)))
		})

		slowFakeFs := afero.Afero{Fs: &slowFs{Fs: fakeFS.Fs}}
		By("Create and start new controller instance for " + hostName1)
		startNewOperatingSystemConfigControllerInstance(hostName1, oscSecretName, fakeDBus, slowFakeFs, imageMountDirectory)
		By("Create and start new controller instance for " + hostName2)
		startNewOperatingSystemConfigControllerInstance(hostName2, oscSecretName, fakeDBus, slowFakeFs, imageMountDirectory)
		By("Create and start new controller instance for " + hostName3)
		startNewOperatingSystemConfigControllerInstance(hostName3, oscSecretName, fakeDBus, slowFakeFs, imageMountDirectory)
	})

	It("should ensure that one node is updated at a time", func() {
		By("Create Secret containing the operating system config")
		Expect(testClient.Create(ctx, oscSecret)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, oscSecret)).To(Succeed())
		})

		lease := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: oscSecret.Name, Namespace: oscSecret.Namespace}}

		By("Wait until first leader is elected")
		var leader1 string
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
			g.Expect(lease.Spec.HolderIdentity).NotTo(BeNil())
			leader1 = *lease.Spec.HolderIdentity
		}).To(Succeed())

		By("Ensure first leader has reconciled while others haven't")
		Eventually(func(g Gomega) {
			exists, err := fakeFS.Exists("/example/file/" + leader1)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(exists).To(BeTrue())
		}).Should(Succeed())
		for _, remainder := range remainingHosts(leader1) {
			test.AssertNoFileOnDisk(fakeFS, "/example/file/"+remainder)
		}

		By("Wait until second leader is elected")
		var leader2 string
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
			g.Expect(lease.Spec.HolderIdentity).NotTo(BeNil())
			leader2 = *lease.Spec.HolderIdentity
			g.Expect(leader2).NotTo(Equal(leader1))
		}).To(Succeed())

		By("Ensure second leader has reconciled while others haven't")
		Eventually(func(g Gomega) {
			exists, err := fakeFS.Exists("/example/file/" + leader2)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(exists).To(BeTrue())
		}).Should(Succeed())
		for _, remainder := range remainingHosts(leader1, leader2) {
			test.AssertNoFileOnDisk(fakeFS, "/example/file/"+remainder)
		}

		By("Wait until third leader is elected")
		var leader3 string
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
			g.Expect(lease.Spec.HolderIdentity).NotTo(BeNil())
			leader3 = *lease.Spec.HolderIdentity
			g.Expect(leader3).NotTo(Or(Equal(leader1), Equal(leader2)))
		}).To(Succeed())

		By("Ensure third leader has reconciled")
		Eventually(func(g Gomega) {
			exists, err := fakeFS.Exists("/example/file/" + leader3)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(exists).To(BeTrue())
		}).Should(Succeed())
		Expect(remainingHosts(leader1, leader2, leader3)).To(BeEmpty())

		By("Ensure nobody claims the Lease again since the work is done")
		Eventually(func(g Gomega) *string { // wait until Lease is released
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
			return lease.Spec.HolderIdentity
		}).Should(BeNil())

		Consistently(func(g Gomega) *string { // ensure Lease is not acquired again
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
			return lease.Spec.HolderIdentity
		}).To(BeNil())
	})
})

func startNewOperatingSystemConfigControllerInstance(hostName, oscSecretName string, fakeDBus *fakedbus.DBus, fakeFS afero.Afero, imageMountDirectory string) {
	GinkgoHelper()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: hostName,
			Labels: map[string]string{
				testID:                   testRunID,
				"kubernetes.io/hostname": hostName,
			},
		},
	}
	By("Create Node")
	Expect(testClient.Create(ctx, node)).To(Succeed())
	DeferCleanup(func() {
		By("Delete Node")
		Expect(testClient.Delete(ctx, node)).To(Succeed())
	})

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Logger:  log.WithName(hostName),
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			DefaultLabelSelector: labels.SelectorFromSet(labels.Set{testID: testRunID}),
		},
		Controller: controllerconfig.Controller{
			SkipNameValidation: ptr.To(true),
		},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(indexer.AddPodNodeName(ctx, mgr.GetFieldIndexer())).To(Succeed())

	By("Register controller")
	Expect((&operatingsystemconfig.Reconciler{
		Config: nodeagentconfigv1alpha1.OperatingSystemConfigControllerConfig{
			SyncPeriod:        &metav1.Duration{Duration: time.Hour},
			SecretName:        oscSecretName,
			KubernetesVersion: semver.MustParse("1.2.3"),
		},
		ConfigDir:             "/var/lib/gardener-node-agent",
		DBus:                  fakeDBus,
		FS:                    fakeFS,
		HostName:              hostName,
		NodeName:              node.Name,
		Extractor:             fakeregistry.NewExtractor(fakeFS, imageMountDirectory),
		CancelContext:         (&cancelFuncEnsurer{}).cancel,
		ContainerdClient:      fakecontainerdclient.NewClient(),
		SkipWritingStateFiles: true,
	}).AddToManager(ctx, mgr)).To(Succeed())

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
}

// slowFs makes "WriteFile" slower for the purpose of this integration test. This is to make the test reliably able to
// observe leader changes on the Lease object for the reconciliation coordination.
type slowFs struct {
	afero.Fs
}

func (fs *slowFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	time.Sleep(2 * time.Second)
	return fs.Fs.OpenFile(name, flag, perm)
}
