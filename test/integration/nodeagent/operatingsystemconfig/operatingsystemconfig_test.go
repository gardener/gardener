// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/apis/config"
	"github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
	fakedbus "github.com/gardener/gardener/pkg/nodeagent/dbus/fake"
	fakeregistry "github.com/gardener/gardener/pkg/nodeagent/registry/fake"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("OperatingSystemConfig controller tests", func() {
	var (
		fakeDBus *fakedbus.DBus
		fakeFS   afero.Afero

		oscSecretName     = testRunID
		kubernetesVersion = semver.MustParse("1.2.3")

		hostName = "test-hostname"
		node     *corev1.Node

		containerdConfigFileContent string

		file1, file2, file3, file4, file5, file6, file7, file8                                           extensionsv1alpha1.File
		gnaUnit, unit1, unit2, unit3, unit4, unit5, unit5DropInsOnly, unit6, unit7, unit8, unit9, unit10 extensionsv1alpha1.Unit
		cgroupDriver                                                                                     extensionsv1alpha1.CgroupDriverName
		registryConfig1, registryConfig2                                                                 extensionsv1alpha1.RegistryConfig
		pluginConfig1, pluginConfig2, pluginConfig3                                                      extensionsv1alpha1.PluginConfig

		operatingSystemConfig *extensionsv1alpha1.OperatingSystemConfig
		oscRaw                []byte
		oscSecret             *corev1.Secret

		imageMountDirectory                string
		cancelFunc                         cancelFuncEnsurer
		pathBootstrapTokenFile             = filepath.Join("/", "var", "lib", "gardener-node-agent", "credentials", "bootstrap-token")
		pathKubeletBootstrapKubeconfigFile = filepath.Join("/", "var", "lib", "kubelet", "kubeconfig-bootstrap")
	)

	BeforeEach(func() {
		var err error

		fakeDBus = fakedbus.New()
		fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}

		imageMountDirectory, err = fakeFS.TempDir("", "fake-node-agent-")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(fakeFS.RemoveAll(imageMountDirectory)).To(Succeed()) })

		cancelFunc = cancelFuncEnsurer{}

		By("Setup manager")
		mgr, err := manager.New(restConfig, manager.Options{
			Metrics: metricsserver.Options{BindAddress: "0"},
			Cache: cache.Options{
				DefaultLabelSelector: labels.SelectorFromSet(labels.Set{testID: testRunID}),
			},
			Controller: controllerconfig.Controller{
				SkipNameValidation: ptr.To(true),
			},
		})
		Expect(err).NotTo(HaveOccurred())

		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: testRunID,
				Labels: map[string]string{
					testID:                   testRunID,
					"kubernetes.io/hostname": hostName,
				},
			},
		}

		containerdConfigFileContent = `imports = ["/etc/containerd/conf.d/*.toml"]

[plugins]

  [plugins.bar]

  [plugins.foo]

    [plugins.foo.bar]
      someKey2 = "someValue2"

      [plugins.foo.bar."foo.bar"]
        someKey = "someValue"

  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "registry.k8s.io/pause:latest"

    [plugins."io.containerd.grpc.v1.cri".containerd]

      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]

        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]

          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
            SystemdCgroup = true

    [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = "/etc/containerd/certs.d"
`

		By("Create Node")
		Expect(testClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(func() {
			By("Delete Node")
			Expect(testClient.Delete(ctx, node)).To(Succeed())
		})

		By("Register controller")
		Expect((&operatingsystemconfig.Reconciler{
			Config: config.OperatingSystemConfigControllerConfig{
				SyncPeriod:        &metav1.Duration{Duration: time.Hour},
				SecretName:        oscSecretName,
				KubernetesVersion: kubernetesVersion,
			},
			DBus:          fakeDBus,
			FS:            fakeFS,
			HostName:      hostName,
			Extractor:     fakeregistry.NewExtractor(fakeFS, imageMountDirectory),
			CancelContext: cancelFunc.cancel,
		}).AddToManager(ctx, mgr)).To(Succeed())

		By("Start manager")
		mgrContext, mgrCancel := context.WithCancel(ctx)

		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(mgrContext)).To(Succeed())
		}()

		revertExec := test.WithVar(&operatingsystemconfig.Exec, func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(""), nil
		})

		DeferCleanup(func() {
			revertExec()
			By("Stop manager")
			mgrCancel()
		})

		file1 = extensionsv1alpha1.File{
			Path:        "/example/file",
			Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "file1"}},
			Permissions: ptr.To[int32](0777),
		}
		file2 = extensionsv1alpha1.File{
			Path:    "/another/file",
			Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: "ZmlsZTI="}},
		}
		file3 = extensionsv1alpha1.File{
			Path:        "/third/file",
			Content:     extensionsv1alpha1.FileContent{ImageRef: &extensionsv1alpha1.FileContentImageRef{Image: "foo-image", FilePathInImage: "/foo-file"}},
			Permissions: ptr.To[int32](0750),
		}
		Expect(fakeFS.WriteFile(path.Join(imageMountDirectory, file3.Content.ImageRef.FilePathInImage), []byte("file3"), 0755)).To(Succeed())
		file4 = extensionsv1alpha1.File{
			Path:        "/unchanged/file",
			Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "file4"}},
			Permissions: ptr.To[int32](0750),
		}
		file5 = extensionsv1alpha1.File{
			Path:        "/changed/file",
			Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "file5"}},
			Permissions: ptr.To[int32](0750),
		}
		file6 = extensionsv1alpha1.File{
			Path:        "/sixth/file",
			Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "file6"}},
			Permissions: ptr.To[int32](0750),
		}
		file7 = extensionsv1alpha1.File{
			Path:        "/seventh/file",
			Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "file7"}},
			Permissions: ptr.To[int32](0750),
		}
		file8 = extensionsv1alpha1.File{
			Path:        "/opt/bin/init-containerd",
			Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "file8"}},
			Permissions: ptr.To[int32](0644),
		}

		gnaUnit = extensionsv1alpha1.Unit{
			Name:    "gardener-node-agent.service",
			Enable:  ptr.To(false),
			Content: ptr.To("#gna"),
		}
		unit1 = extensionsv1alpha1.Unit{
			Name:    "unit1",
			Enable:  ptr.To(true),
			Command: ptr.To(extensionsv1alpha1.CommandStart),
			Content: ptr.To("#unit1"),
			DropIns: []extensionsv1alpha1.DropIn{{
				Name:    "drop",
				Content: "#unit1drop",
			}},
		}
		unit2 = extensionsv1alpha1.Unit{
			Name:    "unit2",
			Enable:  ptr.To(false),
			Command: ptr.To(extensionsv1alpha1.CommandStop),
			Content: ptr.To("#unit2"),
		}
		unit3 = extensionsv1alpha1.Unit{
			Name: "unit3",
			DropIns: []extensionsv1alpha1.DropIn{{
				Name:    "drop",
				Content: "#unit3drop",
			}},
			FilePaths: []string{file4.Path},
		}
		unit4 = extensionsv1alpha1.Unit{
			Name:    "unit4",
			Enable:  ptr.To(true),
			Command: ptr.To(extensionsv1alpha1.CommandStart),
			Content: ptr.To("#unit4"),
			DropIns: []extensionsv1alpha1.DropIn{{
				Name:    "drop",
				Content: "#unit4drop",
			}},
		}
		unit5 = extensionsv1alpha1.Unit{
			Name:    "unit5",
			Enable:  ptr.To(true),
			Command: ptr.To(extensionsv1alpha1.CommandStart),
			Content: ptr.To("#unit5"),
			DropIns: []extensionsv1alpha1.DropIn{
				{
					Name:    "drop1",
					Content: "#unit5drop1",
				},
				{
					Name:    "drop2",
					Content: "#unit5drop2",
				},
			},
		}
		unit5DropInsOnly = extensionsv1alpha1.Unit{
			Name: "unit5",
			DropIns: []extensionsv1alpha1.DropIn{{
				Name:    "extensionsdrop",
				Content: "#unit5extensionsdrop",
			}},
		}
		unit6 = extensionsv1alpha1.Unit{
			Name:      "unit6",
			Enable:    ptr.To(true),
			Content:   ptr.To("#unit6"),
			FilePaths: []string{file3.Path},
		}
		unit7 = extensionsv1alpha1.Unit{
			Name:      "unit7",
			Enable:    ptr.To(true),
			Content:   ptr.To("#unit7"),
			FilePaths: []string{file5.Path},
		}
		unit8 = extensionsv1alpha1.Unit{
			Name:      "unit8",
			Enable:    ptr.To(true),
			Command:   ptr.To(extensionsv1alpha1.CommandStart),
			Content:   ptr.To("#unit8"),
			FilePaths: []string{file6.Path},
		}
		unit9 = extensionsv1alpha1.Unit{
			Name: "unit9",
			DropIns: []extensionsv1alpha1.DropIn{{
				Name:    "drop",
				Content: "#unit9drop",
			}},
			FilePaths: []string{file7.Path},
		}
		unit10 = extensionsv1alpha1.Unit{
			Name:      "containerd-initializer.service",
			FilePaths: []string{file8.Path},
		}

		cgroupDriver = "systemd"

		registryConfig1 = extensionsv1alpha1.RegistryConfig{
			Upstream: "_default",
			Server:   ptr.To("https://registry.hub.docker.com"),
			Hosts: []extensionsv1alpha1.RegistryHost{
				{URL: "https://10.10.10.100:8080"},
				{URL: "https://10.10.10.200:8080"},
			},
		}
		registryConfig2 = extensionsv1alpha1.RegistryConfig{
			Upstream: "registry.k8s.io",
			Server:   ptr.To("https://registry.k8s.io"),
			Hosts: []extensionsv1alpha1.RegistryHost{
				{URL: "https://10.10.10.101:8080", Capabilities: []extensionsv1alpha1.RegistryCapability{"pull"}, CACerts: []string{"/var/certs/ca.crt"}},
			},
		}

		pluginConfig1 = extensionsv1alpha1.PluginConfig{
			Path:   []string{"foo", "bar", "foo.bar"},
			Values: &apiextensionsv1.JSON{Raw: []byte(`{"someKey": "someValue"}`)},
		}
		pluginConfig2 = extensionsv1alpha1.PluginConfig{
			Path:   []string{"foo", "bar"},
			Values: &apiextensionsv1.JSON{Raw: []byte(`{"someKey2": "someValue2"}`)},
		}
		pluginConfig3 = extensionsv1alpha1.PluginConfig{
			Path: []string{"bar"},
		}

		operatingSystemConfig = &extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Files: []extensionsv1alpha1.File{file1, file3, file5, file8},
				Units: []extensionsv1alpha1.Unit{unit1, unit2, unit5, unit5DropInsOnly, unit6, unit7, unit10},
				CRIConfig: &extensionsv1alpha1.CRIConfig{
					Name:         "containerd",
					CgroupDriver: &cgroupDriver,
					Containerd: &extensionsv1alpha1.ContainerdConfig{
						Registries:   []extensionsv1alpha1.RegistryConfig{registryConfig1, registryConfig2},
						SandboxImage: "registry.k8s.io/pause:latest",
						Plugins:      []extensionsv1alpha1.PluginConfig{pluginConfig1, pluginConfig2, pluginConfig3},
					},
				},
			},
			Status: extensionsv1alpha1.OperatingSystemConfigStatus{
				ExtensionFiles: []extensionsv1alpha1.File{file2, file4, file6, file7},
				ExtensionUnits: []extensionsv1alpha1.Unit{unit3, unit4, unit8, unit9},
			},
		}

	})

	JustBeforeEach(func() {
		var err error
		oscRaw, err = runtime.Encode(codec, operatingSystemConfig)
		Expect(err).NotTo(HaveOccurred())

		oscSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        oscSecretName,
				Namespace:   metav1.NamespaceSystem,
				Labels:      map[string]string{testID: testRunID},
				Annotations: map[string]string{"checksum/data-script": utils.ComputeSHA256Hex(oscRaw)},
			},
			Data: map[string][]byte{"osc.yaml": oscRaw},
		}

		By("Create Secret containing the operating system config")
		Expect(testClient.Create(ctx, oscSecret)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, oscSecret)).To(Succeed())
		})

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
	})

	It("should reconcile the configuration when there is no previous OSC", func() {
		By("Wait for node annotations to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Annotations
		}).Should(HaveKeyWithValue("checksum/cloud-config-data", utils.ComputeSHA256Hex(oscRaw)))

		By("Wait for node labels to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Labels
		}).Should(HaveKeyWithValue("worker.gardener.cloud/kubernetes-version", kubernetesVersion.String()))

		By("Assert that files and units have been created")
		test.AssertFileOnDisk(fakeFS, file1.Path, "file1", 0777)
		test.AssertFileOnDisk(fakeFS, file2.Path, "file2", 0600)
		test.AssertFileOnDisk(fakeFS, file3.Path, "file3", 0750)
		test.AssertFileOnDisk(fakeFS, file4.Path, "file4", 0750)
		test.AssertFileOnDisk(fakeFS, file5.Path, "file5", 0750)
		test.AssertFileOnDisk(fakeFS, file6.Path, "file6", 0750)
		test.AssertFileOnDisk(fakeFS, file7.Path, "file7", 0750)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit1.Name, "#unit1", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit1.Name+".d/"+unit1.DropIns[0].Name, "#unit1drop", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit2.Name, "#unit2", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit3.Name+".d/"+unit3.DropIns[0].Name, "#unit3drop", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit4.Name, "#unit4", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit4.Name+".d/"+unit4.DropIns[0].Name, "#unit4drop", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit5.Name, "#unit5", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit5.Name+".d/"+unit5.DropIns[0].Name, "#unit5drop1", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit5.Name+".d/"+unit5.DropIns[1].Name, "#unit5drop2", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit5.Name+".d/"+unit5DropInsOnly.DropIns[0].Name, "#unit5extensionsdrop", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit6.Name, "#unit6", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit7.Name, "#unit7", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit8.Name, "#unit8", 0600)
		test.AssertNoFileOnDisk(fakeFS, "/etc/systemd/system/"+unit9.Name)
		test.AssertNoFileOnDisk(fakeFS, "/opt/bin/init-containerd")
		test.AssertDirectoryOnDisk(fakeFS, "/var/bin/containerruntimes")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/containerd/certs.d")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/containerd/conf.d")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/systemd/system/containerd.service.d")
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/config.toml", containerdConfigFileContent, 0644)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/containerd.service.d/30-env_config.conf", "[Service]\nEnvironment=\"PATH=/var/bin/containerruntimes:"+os.Getenv("PATH")+"\"\n", 0644)
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/certs.d/"+registryConfig1.Upstream+"/hosts.toml", "# managed by gardener-node-agent\nserver = \"https://registry.hub.docker.com\"\n\n[host.\"https://10.10.10.100:8080\"]\n  capabilities = [\"pull\",\"resolve\"]\n\n[host.\"https://10.10.10.200:8080\"]\n  capabilities = [\"pull\",\"resolve\"]\n\n", 0644)
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/certs.d/"+registryConfig2.Upstream+"/hosts.toml", "# managed by gardener-node-agent\nserver = \"https://registry.k8s.io\"\n\n[host.\"https://10.10.10.101:8080\"]\n  capabilities = [\"pull\"]\n  ca = [\"/var/certs/ca.crt\"]\n\n", 0644)

		By("Assert that unit actions have been applied")
		Expect(fakeDBus.Actions).To(ConsistOf(
			fakedbus.SystemdAction{Action: fakedbus.ActionStart, UnitNames: []string{"containerd.service"}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit1.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDisable, UnitNames: []string{unit2.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit3.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit4.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit5.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit6.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit7.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit8.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit9.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit1.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionStop, UnitNames: []string{unit2.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit3.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit4.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit5.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit6.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit7.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit8.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit9.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{"containerd.service"}},
		))

		By("Assert that bootstrap files have been deleted")
		test.AssertNoFileOnDisk(fakeFS, pathKubeletBootstrapKubeconfigFile)
		test.AssertNoFileOnDisk(fakeFS, pathBootstrapTokenFile)

		By("Assert that cancel func has not been called")
		Expect(cancelFunc.called).To(BeFalse())
	})

	It("should reconcile the configuration when there is a previous OSC", func() {
		By("Wait for node annotations to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Annotations
		}).Should(HaveKeyWithValue("checksum/cloud-config-data", utils.ComputeSHA256Hex(oscRaw)))

		By("Wait for node labels to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Labels
		}).Should(HaveKeyWithValue("worker.gardener.cloud/kubernetes-version", kubernetesVersion.String()))

		fakeDBus.Actions = nil // reset actions on dbus to not repeat assertions from above for update scenario

		// manually change permissions of unit and drop-in file (should be restored on next reconciliation)
		Expect(fakeFS.Chmod("/etc/systemd/system/"+unit2.Name, 0777)).To(Succeed())

		// manually change content of containerd registry content (should be restored on next reconciliation)
		Expect(fakeFS.WriteFile("/etc/containerd/certs.d/"+registryConfig1.Upstream+"/hosts.toml", []byte("foo"), 0600)).To(Succeed())

		By("Update Operating System Config")
		// delete unit1
		// delete file2
		// add drop-in to unit2 and enable+start it
		// disable unit4 and remove all drop-ins
		// remove only first drop-in from unit5
		// remove file3 from unit6.FilePaths while keeping it unchanged
		// the content of file5 (belonging to unit7) is changed, so unit7 is restarting
		// the content of file6 (belonging to unit8) is changed, so unit8 is restarting
		// the content of file7 (belonging to unit9) is changed, so unit9 is restarting
		// file1, unit3, and gardener-node-agent unit are unchanged, so unit3 is not restarting and cancel func is not called
		unit2.Enable = ptr.To(true)
		unit2.Command = ptr.To(extensionsv1alpha1.CommandStart)
		unit2.DropIns = []extensionsv1alpha1.DropIn{{Name: "dropdropdrop", Content: "#unit2drop"}}
		unit4.Enable = ptr.To(false)
		unit4.DropIns = nil
		unit5.DropIns = unit5.DropIns[1:]
		unit6.FilePaths = nil

		operatingSystemConfig.Spec.Units = []extensionsv1alpha1.Unit{unit2, unit5, unit6, unit7, unit10}
		operatingSystemConfig.Spec.Files[2].Content.Inline.Data = "changeme"
		operatingSystemConfig.Status.ExtensionUnits = []extensionsv1alpha1.Unit{unit3, unit4, unit8, unit9}
		operatingSystemConfig.Status.ExtensionFiles = []extensionsv1alpha1.File{file4, file6, file7}
		operatingSystemConfig.Status.ExtensionFiles[1].Content.Inline.Data = "changed"
		operatingSystemConfig.Status.ExtensionFiles[2].Content.Inline.Data = "changed-as-well"

		var err error
		oscRaw, err = runtime.Encode(codec, operatingSystemConfig)
		Expect(err).NotTo(HaveOccurred())

		By("Update Secret containing the operating system config")
		patch := client.MergeFrom(oscSecret.DeepCopy())
		oscSecret.Annotations["checksum/data-script"] = utils.ComputeSHA256Hex(oscRaw)
		oscSecret.Data["osc.yaml"] = oscRaw
		Expect(testClient.Patch(ctx, oscSecret, patch)).To(Succeed())

		By("Wait for node annotations to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Annotations
		}).Should(HaveKeyWithValue("checksum/cloud-config-data", utils.ComputeSHA256Hex(oscRaw)))

		By("Wait for node labels to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Labels
		}).Should(HaveKeyWithValue("worker.gardener.cloud/kubernetes-version", kubernetesVersion.String()))

		By("Assert that files and units have been created")
		test.AssertFileOnDisk(fakeFS, file1.Path, "file1", 0777)
		test.AssertNoFileOnDisk(fakeFS, file2.Path)
		test.AssertFileOnDisk(fakeFS, file3.Path, "file3", 0750)
		test.AssertFileOnDisk(fakeFS, file4.Path, "file4", 0750)
		test.AssertFileOnDisk(fakeFS, file5.Path, "changeme", 0750)
		test.AssertFileOnDisk(fakeFS, file6.Path, "changed", 0750)
		test.AssertFileOnDisk(fakeFS, file7.Path, "changed-as-well", 0750)
		test.AssertNoFileOnDisk(fakeFS, "/etc/systemd/system/"+unit1.Name)
		test.AssertNoDirectoryOnDisk(fakeFS, "/etc/systemd/system/"+unit1.Name+".d")
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit2.Name, "#unit2", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit2.Name+".d/"+unit2.DropIns[0].Name, "#unit2drop", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit3.Name+".d/"+unit3.DropIns[0].Name, "#unit3drop", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit4.Name, "#unit4", 0600)
		test.AssertNoDirectoryOnDisk(fakeFS, "/etc/systemd/system/"+unit4.Name+".d")
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit5.Name, "#unit5", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit5.Name+".d/"+unit5.DropIns[0].Name, "#unit5drop2", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit7.Name, "#unit7", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+unit8.Name, "#unit8", 0600)
		test.AssertNoFileOnDisk(fakeFS, "/etc/systemd/system/"+unit9.Name)
		test.AssertNoFileOnDisk(fakeFS, "/opt/bin/init-containerd")
		test.AssertDirectoryOnDisk(fakeFS, "/var/bin/containerruntimes")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/containerd/certs.d")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/containerd/conf.d")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/systemd/system/containerd.service.d")
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/config.toml", containerdConfigFileContent, 0644)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/containerd.service.d/30-env_config.conf", "[Service]\nEnvironment=\"PATH=/var/bin/containerruntimes:"+os.Getenv("PATH")+"\"\n", 0644)
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/certs.d/"+registryConfig1.Upstream+"/hosts.toml", "# managed by gardener-node-agent\nserver = \"https://registry.hub.docker.com\"\n\n[host.\"https://10.10.10.100:8080\"]\n  capabilities = [\"pull\",\"resolve\"]\n\n[host.\"https://10.10.10.200:8080\"]\n  capabilities = [\"pull\",\"resolve\"]\n\n", 0644)
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/certs.d/"+registryConfig2.Upstream+"/hosts.toml", "# managed by gardener-node-agent\nserver = \"https://registry.k8s.io\"\n\n[host.\"https://10.10.10.101:8080\"]\n  capabilities = [\"pull\"]\n  ca = [\"/var/certs/ca.crt\"]\n\n", 0644)

		By("Assert that unit actions have been applied")
		Expect(fakeDBus.Actions).To(ConsistOf(
			fakedbus.SystemdAction{Action: fakedbus.ActionStart, UnitNames: []string{"containerd.service"}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit2.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit5.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit6.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit7.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit8.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit9.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDisable, UnitNames: []string{unit4.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDisable, UnitNames: []string{unit1.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionStop, UnitNames: []string{unit1.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit2.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit5.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionStop, UnitNames: []string{unit4.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit6.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit7.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit8.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit9.Name}},
		))

		By("Assert that cancel func has not been called")
		Expect(cancelFunc.called).To(BeFalse())
	})

	It("should reconcile the configuration when the containerd registries change", func() {
		By("Wait for node annotations to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Annotations
		}).Should(HaveKeyWithValue("checksum/cloud-config-data", utils.ComputeSHA256Hex(oscRaw)))

		By("Wait for node labels to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Labels
		}).Should(HaveKeyWithValue("worker.gardener.cloud/kubernetes-version", kubernetesVersion.String()))

		fakeDBus.Actions = nil // reset actions on dbus to not repeat assertions from above for update scenario

		By("Update Operating System Config")
		operatingSystemConfig.Spec.CRIConfig.Containerd.Registries = []extensionsv1alpha1.RegistryConfig{registryConfig2}
		operatingSystemConfig.Spec.CRIConfig.Containerd.SandboxImage = operatingSystemConfig.Spec.CRIConfig.Containerd.SandboxImage + "-test"

		var err error
		oscRaw, err = runtime.Encode(codec, operatingSystemConfig)
		Expect(err).NotTo(HaveOccurred())

		By("Update Secret containing the operating system config")
		patch := client.MergeFrom(oscSecret.DeepCopy())
		oscSecret.Annotations["checksum/data-script"] = utils.ComputeSHA256Hex(oscRaw)
		oscSecret.Data["osc.yaml"] = oscRaw
		Expect(testClient.Patch(ctx, oscSecret, patch)).To(Succeed())

		By("Wait for node annotations to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Annotations
		}).Should(HaveKeyWithValue("checksum/cloud-config-data", utils.ComputeSHA256Hex(oscRaw)))

		By("Wait for node labels to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Labels
		}).Should(HaveKeyWithValue("worker.gardener.cloud/kubernetes-version", kubernetesVersion.String()))

		By("Assert that files and directories have been created")
		test.AssertDirectoryOnDisk(fakeFS, "/var/bin/containerruntimes")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/containerd/certs.d")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/containerd/conf.d")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/systemd/system/containerd.service.d")
		test.AssertNoFileOnDisk(fakeFS, "/etc/containerd/certs.d/"+registryConfig1.Upstream+"/hosts.toml")
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/certs.d/"+registryConfig2.Upstream+"/hosts.toml", "# managed by gardener-node-agent\nserver = \"https://registry.k8s.io\"\n\n[host.\"https://10.10.10.101:8080\"]\n  capabilities = [\"pull\"]\n  ca = [\"/var/certs/ca.crt\"]\n\n", 0644)

		By("Assert that unit actions have been applied")
		Expect(fakeDBus.Actions).To(ConsistOf(
			fakedbus.SystemdAction{Action: fakedbus.ActionStart, UnitNames: []string{"containerd.service"}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{"containerd.service"}},
		))

		By("Assert that cancel func has not been called")
		Expect(cancelFunc.called).To(BeFalse())
	})

	It("should reconcile the configuration when the containerd plugins change", func() {
		By("Wait for node annotations to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Annotations
		}).Should(HaveKeyWithValue("checksum/cloud-config-data", utils.ComputeSHA256Hex(oscRaw)))

		By("Wait for node labels to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Labels
		}).Should(HaveKeyWithValue("worker.gardener.cloud/kubernetes-version", kubernetesVersion.String()))

		fakeDBus.Actions = nil // reset actions on dbus to not repeat assertions from above for update scenario

		By("Checking containerd configuration before change")
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/config.toml", containerdConfigFileContent, 0644)

		By("Update Operating System Config")
		operatingSystemConfig.Spec.CRIConfig.Containerd.Plugins = append(operatingSystemConfig.Spec.CRIConfig.Containerd.Plugins, extensionsv1alpha1.PluginConfig{
			Path: []string{"foo"},
			Op:   ptr.To[extensionsv1alpha1.PluginPathOperation]("remove"),
		})

		var err error
		oscRaw, err = runtime.Encode(codec, operatingSystemConfig)
		Expect(err).NotTo(HaveOccurred())

		By("Update Secret containing the operating system config")
		patch := client.MergeFrom(oscSecret.DeepCopy())
		oscSecret.Annotations["checksum/data-script"] = utils.ComputeSHA256Hex(oscRaw)
		oscSecret.Data["osc.yaml"] = oscRaw
		Expect(testClient.Patch(ctx, oscSecret, patch)).To(Succeed())

		By("Wait for node annotations to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Annotations
		}).Should(HaveKeyWithValue("checksum/cloud-config-data", utils.ComputeSHA256Hex(oscRaw)))

		By("Wait for node labels to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Labels
		}).Should(HaveKeyWithValue("worker.gardener.cloud/kubernetes-version", kubernetesVersion.String()))

		By("Assert that containerd config was updated properly")
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/config.toml", "imports = [\"/etc/containerd/conf.d/*.toml\"]\n\n[plugins]\n\n  [plugins.bar]\n\n  [plugins.\"io.containerd.grpc.v1.cri\"]\n    sandbox_image = \"registry.k8s.io/pause:latest\"\n\n    [plugins.\"io.containerd.grpc.v1.cri\".containerd]\n\n      [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes]\n\n        [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.runc]\n\n          [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.runc.options]\n            SystemdCgroup = true\n\n    [plugins.\"io.containerd.grpc.v1.cri\".registry]\n      config_path = \"/etc/containerd/certs.d\"\n", 0644)

		By("Assert that unit actions have been applied")
		Expect(fakeDBus.Actions).To(ConsistOf(
			fakedbus.SystemdAction{Action: fakedbus.ActionStart, UnitNames: []string{"containerd.service"}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{"containerd.service"}},
		))

		By("Assert that cancel func has not been called")
		Expect(cancelFunc.called).To(BeFalse())
	})

	It("should reconcile the configuration when the cgroup driver changes", func() {
		By("Wait for node annotations to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Annotations
		}).Should(HaveKeyWithValue("checksum/cloud-config-data", utils.ComputeSHA256Hex(oscRaw)))

		By("Wait for node labels to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Labels
		}).Should(HaveKeyWithValue("worker.gardener.cloud/kubernetes-version", kubernetesVersion.String()))

		fakeDBus.Actions = nil // reset actions on dbus to not repeat assertions from above for update scenario

		By("Checking containerd configuration before change")
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/config.toml", containerdConfigFileContent, 0644)

		By("Update Operating System Config")
		operatingSystemConfig.Spec.CRIConfig.CgroupDriver = ptr.To(extensionsv1alpha1.CgroupDriverName("cgroupfs"))

		var err error
		oscRaw, err = runtime.Encode(codec, operatingSystemConfig)
		Expect(err).NotTo(HaveOccurred())

		By("Update Secret containing the operating system config")
		patch := client.MergeFrom(oscSecret.DeepCopy())
		oscSecret.Annotations["checksum/data-script"] = utils.ComputeSHA256Hex(oscRaw)
		oscSecret.Data["osc.yaml"] = oscRaw
		Expect(testClient.Patch(ctx, oscSecret, patch)).To(Succeed())

		By("Wait for node annotations to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Annotations
		}).Should(HaveKeyWithValue("checksum/cloud-config-data", utils.ComputeSHA256Hex(oscRaw)))

		By("Wait for node labels to be updated")
		Eventually(func(g Gomega) map[string]string {
			updatedNode := &corev1.Node{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
			return updatedNode.Labels
		}).Should(HaveKeyWithValue("worker.gardener.cloud/kubernetes-version", kubernetesVersion.String()))

		By("Assert that containerd config was updated properly")
		expectedContainerdContent := strings.ReplaceAll(containerdConfigFileContent, "SystemdCgroup = true", "SystemdCgroup = false")
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/config.toml", expectedContainerdContent, 0644)

		By("Assert that unit actions have been applied")
		Expect(fakeDBus.Actions).To(ConsistOf(
			fakedbus.SystemdAction{Action: fakedbus.ActionStart, UnitNames: []string{"containerd.service"}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{"containerd.service"}},
		))

		By("Assert that cancel func has not been called")
		Expect(cancelFunc.called).To(BeFalse())
	})

	It("should call the cancel function when gardener-node-agent must be restarted itself", func() {
		var lastAppliedOSC []byte
		By("Wait last-applied OSC file to be persisted")
		Eventually(func() error {
			var err error
			lastAppliedOSC, err = fakeFS.ReadFile("/var/lib/gardener-node-agent/last-applied-osc.yaml")
			return err
		}).Should(Succeed())

		fakeDBus.Actions = nil // reset actions on dbus to not repeat assertions from above for update scenario

		By("Update Operating System Config")
		operatingSystemConfig.Spec.Units = append(operatingSystemConfig.Spec.Units, gnaUnit)

		var err error
		oscRaw, err = runtime.Encode(codec, operatingSystemConfig)
		Expect(err).NotTo(HaveOccurred())

		By("Update Secret containing the operating system config")
		patch := client.MergeFrom(oscSecret.DeepCopy())
		oscSecret.Annotations["checksum/data-script"] = utils.ComputeSHA256Hex(oscRaw)
		oscSecret.Data["osc.yaml"] = oscRaw
		Expect(testClient.Patch(ctx, oscSecret, patch)).To(Succeed())

		By("Wait last-applied OSC file to be updated")
		Eventually(func(g Gomega) []byte {
			content, err := fakeFS.ReadFile("/var/lib/gardener-node-agent/last-applied-osc.yaml")
			g.Expect(err).NotTo(HaveOccurred())
			return content
		}).ShouldNot(Equal(lastAppliedOSC))

		By("Assert that files and units have been created")
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+gnaUnit.Name, "#gna", 0600)

		By("Assert that unit actions have been applied")
		Expect(fakeDBus.Actions).To(ConsistOf(
			fakedbus.SystemdAction{Action: fakedbus.ActionStart, UnitNames: []string{"containerd.service"}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{gnaUnit.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
		))

		By("Expect that cancel func has been called")
		Expect(cancelFunc.called).To(BeTrue())
	})

	Context("when CRI is not containerd", func() {
		BeforeEach(func() {
			operatingSystemConfig.Spec.CRIConfig = nil
		})

		It("should not handle containerd configs", func() {
			By("Wait for node annotations to be updated")
			Eventually(func(g Gomega) map[string]string {
				updatedNode := &corev1.Node{}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
				return updatedNode.Annotations
			}).Should(HaveKeyWithValue("checksum/cloud-config-data", utils.ComputeSHA256Hex(oscRaw)))

			By("Wait for node labels to be updated")
			Eventually(func(g Gomega) map[string]string {
				updatedNode := &corev1.Node{}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
				return updatedNode.Labels
			}).Should(HaveKeyWithValue("worker.gardener.cloud/kubernetes-version", kubernetesVersion.String()))

			By("Assert that files and units have been created")
			test.AssertNoDirectoryOnDisk(fakeFS, "/var/bin/containerruntimes")
			test.AssertNoDirectoryOnDisk(fakeFS, "/etc/containerd/certs.d")
			test.AssertNoDirectoryOnDisk(fakeFS, "/etc/containerd/conf.d")
			test.AssertNoDirectoryOnDisk(fakeFS, "/etc/systemd/system/containerd.service.d")
			test.AssertNoFileOnDisk(fakeFS, "/etc/containerd/config.toml")
		})
	})
})

type cancelFuncEnsurer struct {
	called bool
}

func (c *cancelFuncEnsurer) cancel() {
	c.called = true
}
