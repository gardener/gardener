// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/pkg/api/indexer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/nodeagent"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	healthcheckcontroller "github.com/gardener/gardener/pkg/nodeagent/controller/healthcheck"
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
		channel  chan event.TypedGenericEvent[*corev1.Secret]

		oscSecretName            = testRunID
		kubernetesVersion        = semver.MustParse("1.2.3")
		secretName1, secretName2 string

		hostName = "test-hostname"
		node     *corev1.Node

		containerdConfigFileContent string

		file1, file2, file3, file4, file5, file6, file7, file8                                                                         extensionsv1alpha1.File
		gnaUnit, unit1, unit2, unit3, unit4, unit5, unit5DropInsOnly, unit6, unit7, unit8, unit9, existingUnitDropIn, containerdDropIn extensionsv1alpha1.Unit
		cgroupDriver                                                                                                                   extensionsv1alpha1.CgroupDriverName
		registryConfig1, registryConfig2                                                                                               extensionsv1alpha1.RegistryConfig
		pluginConfig1, pluginConfig2, pluginConfig3                                                                                    extensionsv1alpha1.PluginConfig

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
		Expect(indexer.AddPodNodeName(ctx, mgr.GetFieldIndexer())).To(Succeed())

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

    [plugins."io.containerd.grpc.v1.cri".cni]
      bin_dir = "/opt/cni/bin"

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

		channel = make(chan event.TypedGenericEvent[*corev1.Secret])
		secretName1 = "test-secret-1"
		secretName2 = "test-secret-2"

		By("Register controller")
		Expect((&operatingsystemconfig.Reconciler{
			Config: nodeagentconfigv1alpha1.OperatingSystemConfigControllerConfig{
				SyncPeriod:        &metav1.Duration{Duration: time.Hour},
				SecretName:        oscSecretName,
				KubernetesVersion: kubernetesVersion,
			},
			DBus:      fakeDBus,
			FS:        fakeFS,
			HostName:  hostName,
			NodeName:  node.Name,
			Extractor: fakeregistry.NewExtractor(fakeFS, imageMountDirectory),
			Channel:   channel,
			TokenSecretSyncConfigs: []nodeagentconfigv1alpha1.TokenSecretSyncConfig{
				{SecretName: secretName1},
				{SecretName: secretName2},
			},
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
			Permissions: ptr.To[uint32](0777),
		}
		file2 = extensionsv1alpha1.File{
			Path:    "/another/file",
			Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: "ZmlsZTI="}},
		}
		file3 = extensionsv1alpha1.File{
			Path:        "/third/file",
			Content:     extensionsv1alpha1.FileContent{ImageRef: &extensionsv1alpha1.FileContentImageRef{Image: "foo-image", FilePathInImage: "/foo-file"}},
			Permissions: ptr.To[uint32](0750),
		}
		Expect(fakeFS.WriteFile(path.Join(imageMountDirectory, file3.Content.ImageRef.FilePathInImage), []byte("file3"), 0755)).To(Succeed())
		file4 = extensionsv1alpha1.File{
			Path:        "/unchanged/file",
			Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "file4"}},
			Permissions: ptr.To[uint32](0750),
		}
		file5 = extensionsv1alpha1.File{
			Path:        "/changed/file",
			Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "file5"}},
			Permissions: ptr.To[uint32](0750),
		}
		file6 = extensionsv1alpha1.File{
			Path:        "/sixth/file",
			Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "file6"}},
			Permissions: ptr.To[uint32](0750),
		}
		file7 = extensionsv1alpha1.File{
			Path:        "/seventh/file",
			Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "file7"}},
			Permissions: ptr.To[uint32](0750),
		}
		file8 = extensionsv1alpha1.File{
			Path:        "/opt/bin/init-containerd",
			Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "", Data: "file8"}},
			Permissions: ptr.To[uint32](0644),
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
		existingUnitDropIn = extensionsv1alpha1.Unit{
			Name: "existing-unit.service",
			DropIns: []extensionsv1alpha1.DropIn{{
				Name:    "drop",
				Content: "#unit11drop",
			}},
		}
		containerdDropIn = extensionsv1alpha1.Unit{
			Name: "containerd.service",
			DropIns: []extensionsv1alpha1.DropIn{{
				Name:    "extensionsdrop.conf",
				Content: "#containerdextensionsdrop",
			}},
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
				Units: []extensionsv1alpha1.Unit{unit1, unit2, unit5, unit5DropInsOnly, unit6, unit7},
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
				ExtensionUnits: []extensionsv1alpha1.Unit{unit3, unit4, unit8, unit9, existingUnitDropIn},
			},
		}

	})

	JustBeforeEach(func() {
		if operatingSystemConfig.Spec.CRIConfig != nil {
			operatingSystemConfig.Status.ExtensionUnits = append(operatingSystemConfig.Status.ExtensionUnits, containerdDropIn)
		}

		Expect(fakeFS.WriteFile("/etc/systemd/system/existing-unit.service", []byte("#existingunit"), 0600)).To(Succeed())
		Expect(fakeFS.WriteFile("/etc/systemd/system/existing-unit.service.d/existing-dropin.conf", []byte("#existingdropin"), 0600)).To(Succeed())

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
		waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
		waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

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
		test.AssertDirectoryOnDisk(fakeFS, "/var/bin/containerruntimes")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/containerd/certs.d")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/containerd/conf.d")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/systemd/system/containerd.service.d")
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/config.toml", containerdConfigFileContent, 0644)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/containerd.service.d/30-env_config.conf", "[Service]\nEnvironment=\"PATH=/var/bin/containerruntimes:"+os.Getenv("PATH")+"\"\n", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/containerd.service.d/"+containerdDropIn.DropIns[0].Name, containerdDropIn.DropIns[0].Content, 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/certs.d/"+registryConfig1.Upstream+"/hosts.toml", "# managed by gardener-node-agent\nserver = \"https://registry.hub.docker.com\"\n\n[host.\"https://10.10.10.100:8080\"]\n  capabilities = [\"pull\",\"resolve\"]\n\n[host.\"https://10.10.10.200:8080\"]\n  capabilities = [\"pull\",\"resolve\"]\n\n", 0644)
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/certs.d/"+registryConfig2.Upstream+"/hosts.toml", "# managed by gardener-node-agent\nserver = \"https://registry.k8s.io\"\n\n[host.\"https://10.10.10.101:8080\"]\n  capabilities = [\"pull\"]\n  ca = [\"/var/certs/ca.crt\"]\n\n", 0644)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/existing-unit.service", "#existingunit", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/existing-unit.service.d/existing-dropin.conf", "#existingdropin", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+existingUnitDropIn.Name+".d/"+existingUnitDropIn.DropIns[0].Name, "#unit11drop", 0600)
		test.AssertFileOnDisk(fakeFS, "/var/lib/gardener-node-agent/last-computed-osc-changes.yaml", `caRotation:
  kubelet: false
  nodeAgent: false
containerd:
  configFileChanged: false
  registries: {}
files: {}
kubeletUpdate:
  configUpdate: false
  cpuManagerPolicy: false
  minorVersionUpdate: false
mustRestartNodeAgent: false
operatingSystemConfigChecksum: 4330078242f98407daaaa8e755dbc054dc301233a6bab2bc7706801365711527
osUpdate: false
saKeyRotation: false
units: {}
`, 0600)

		By("Assert that unit actions have been applied")
		Expect(fakeDBus.Actions).To(ConsistOf(
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit1.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDisable, UnitNames: []string{unit2.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit3.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit4.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit5.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit6.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit7.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit8.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit9.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{"containerd.service"}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{"existing-unit.service"}},
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
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{"existing-unit.service"}},
		))

		By("Assert that bootstrap files have been deleted")
		test.AssertNoFileOnDisk(fakeFS, pathKubeletBootstrapKubeconfigFile)
		test.AssertNoFileOnDisk(fakeFS, pathBootstrapTokenFile)

		By("Assert that cancel func has not been called")
		Expect(cancelFunc.called).To(BeFalse())
	})

	It("should reconcile only parts of the configuration that were not applied yet", func() {
		waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
		waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

		fakeDBus.Actions = nil // reset actions on dbus to not repeat assertions from above for update scenario

		operatingSystemConfig.Spec.Units[0].Command = ptr.To(extensionsv1alpha1.CommandStop)
		operatingSystemConfig.Spec.Units[1].Enable = ptr.To(true)
		operatingSystemConfig.Spec.Units[1].Command = ptr.To(extensionsv1alpha1.CommandStart)
		fakeDBus.InjectRestartFailure(fmt.Errorf("injected failure for unit2"), unit2.Name)

		var err error
		oscRaw, err = runtime.Encode(codec, operatingSystemConfig)
		Expect(err).NotTo(HaveOccurred())

		By("Update Secret containing the operating system config")
		patch := client.MergeFrom(oscSecret.DeepCopy())
		oscSecret.Annotations["checksum/data-script"] = utils.ComputeSHA256Hex(oscRaw)
		oscSecret.Data["osc.yaml"] = oscRaw
		Expect(testClient.Patch(ctx, oscSecret, patch)).To(Succeed())

		waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
		waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

		By("Assert that unit actions have been applied")
		Expect(fakeDBus.Actions).To(ConsistOf(
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit1.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit2.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
			fakedbus.SystemdAction{Action: fakedbus.ActionStop, UnitNames: []string{unit1.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit2.Name}},
			// failure was injected here, so we expect the next attempt to only retry the failed action (restart unit2)
			// and the actions that are taken on every reconcile.
			fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{unit2.Name}},
		))
	})

	It("should reconcile the configuration when there is a previous OSC", func() {
		waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
		waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

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
		// remove existingUnitDropIn, so the drop-in file should be removed, but the existing unit should not be affected
		// remove containerd drop-in extension unit, the other containerd drop-in should not be affected
		unit2.Enable = ptr.To(true)
		unit2.Command = ptr.To(extensionsv1alpha1.CommandStart)
		unit2.DropIns = []extensionsv1alpha1.DropIn{{Name: "dropdropdrop", Content: "#unit2drop"}}
		unit4.Enable = ptr.To(false)
		unit4.DropIns = nil
		unit5.DropIns = unit5.DropIns[1:]
		unit6.FilePaths = nil

		operatingSystemConfig.Spec.Units = []extensionsv1alpha1.Unit{unit2, unit5, unit6, unit7}
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

		waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
		waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

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
		test.AssertDirectoryOnDisk(fakeFS, "/var/bin/containerruntimes")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/containerd/certs.d")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/containerd/conf.d")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/systemd/system/containerd.service.d")
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/config.toml", containerdConfigFileContent, 0644)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/containerd.service.d/30-env_config.conf", "[Service]\nEnvironment=\"PATH=/var/bin/containerruntimes:"+os.Getenv("PATH")+"\"\n", 0600)
		test.AssertNoFileOnDisk(fakeFS, "/etc/systemd/system/containerd.service.d/"+containerdDropIn.DropIns[0].Name)
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/certs.d/"+registryConfig1.Upstream+"/hosts.toml", "# managed by gardener-node-agent\nserver = \"https://registry.hub.docker.com\"\n\n[host.\"https://10.10.10.100:8080\"]\n  capabilities = [\"pull\",\"resolve\"]\n\n[host.\"https://10.10.10.200:8080\"]\n  capabilities = [\"pull\",\"resolve\"]\n\n", 0644)
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/certs.d/"+registryConfig2.Upstream+"/hosts.toml", "# managed by gardener-node-agent\nserver = \"https://registry.k8s.io\"\n\n[host.\"https://10.10.10.101:8080\"]\n  capabilities = [\"pull\"]\n  ca = [\"/var/certs/ca.crt\"]\n\n", 0644)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/existing-unit.service", "#existingunit", 0600)
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/existing-unit.service.d/existing-dropin.conf", "#existingdropin", 0600)
		test.AssertNoFileOnDisk(fakeFS, "/etc/systemd/system/"+existingUnitDropIn.Name+".d/"+existingUnitDropIn.DropIns[0].Name)

		By("Assert that unit actions have been applied")
		Expect(fakeDBus.Actions).To(ConsistOf(
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit2.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit5.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit6.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit7.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit8.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{unit9.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{"containerd.service"}},
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
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{"containerd.service"}},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{"existing-unit.service"}},
		))

		By("Assert that cancel func has not been called")
		Expect(cancelFunc.called).To(BeFalse())
	})

	It("should reconcile the configuration when the containerd registries change", func() {
		waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
		waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

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

		waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
		waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

		By("Assert that files and directories have been created")
		test.AssertDirectoryOnDisk(fakeFS, "/var/bin/containerruntimes")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/containerd/certs.d")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/containerd/conf.d")
		test.AssertDirectoryOnDisk(fakeFS, "/etc/systemd/system/containerd.service.d")
		test.AssertNoFileOnDisk(fakeFS, "/etc/containerd/certs.d/"+registryConfig1.Upstream+"/hosts.toml")
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/certs.d/"+registryConfig2.Upstream+"/hosts.toml", "# managed by gardener-node-agent\nserver = \"https://registry.k8s.io\"\n\n[host.\"https://10.10.10.101:8080\"]\n  capabilities = [\"pull\"]\n  ca = [\"/var/certs/ca.crt\"]\n\n", 0644)

		By("Assert that unit actions have been applied")
		Expect(fakeDBus.Actions).To(ConsistOf(
			fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{"containerd.service"}},
		))

		By("Assert that cancel func has not been called")
		Expect(cancelFunc.called).To(BeFalse())
	})

	It("should reconcile the configuration when the containerd plugins change", func() {
		waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
		waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

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

		waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
		waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

		By("Assert that containerd config was updated properly")
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/config.toml", "imports = [\"/etc/containerd/conf.d/*.toml\"]\n\n[plugins]\n\n  [plugins.bar]\n\n  [plugins.\"io.containerd.grpc.v1.cri\"]\n    sandbox_image = \"registry.k8s.io/pause:latest\"\n\n    [plugins.\"io.containerd.grpc.v1.cri\".cni]\n      bin_dir = \"/opt/cni/bin\"\n\n    [plugins.\"io.containerd.grpc.v1.cri\".containerd]\n\n      [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes]\n\n        [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.runc]\n\n          [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.runc.options]\n            SystemdCgroup = true\n\n    [plugins.\"io.containerd.grpc.v1.cri\".registry]\n      config_path = \"/etc/containerd/certs.d\"\n", 0644)

		By("Assert that unit actions have been applied")
		Expect(fakeDBus.Actions).To(ConsistOf(
			fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{"containerd.service"}},
		))

		By("Assert that cancel func has not been called")
		Expect(cancelFunc.called).To(BeFalse())
	})

	It("should reconcile the configuration when the cgroup driver changes", func() {
		waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
		waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

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

		waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
		waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

		By("Assert that containerd config was updated properly")
		expectedContainerdContent := strings.ReplaceAll(containerdConfigFileContent, "SystemdCgroup = true", "SystemdCgroup = false")
		test.AssertFileOnDisk(fakeFS, "/etc/containerd/config.toml", expectedContainerdContent, 0644)

		By("Assert that unit actions have been applied")
		Expect(fakeDBus.Actions).To(ConsistOf(
			fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
			fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{"containerd.service"}},
		))

		By("Assert that cancel func has not been called")
		Expect(cancelFunc.called).To(BeFalse())
	})

	It("should call the cancel function when gardener-node-agent must be restarted itself", func() {
		var lastAppliedOSC []byte
		By("Wait until last-applied OSC file is persisted")
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

		By("Wait for last-applied OSC file to be updated")
		Eventually(func(g Gomega) []byte {
			content, err := fakeFS.ReadFile("/var/lib/gardener-node-agent/last-applied-osc.yaml")
			g.Expect(err).NotTo(HaveOccurred())
			return content
		}).ShouldNot(Equal(lastAppliedOSC))

		By("Assert that files and units have been created")
		test.AssertFileOnDisk(fakeFS, "/etc/systemd/system/"+gnaUnit.Name, "#gna", 0600)

		By("Assert that unit actions have been applied")
		Expect(fakeDBus.Actions).To(ConsistOf(
			fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{gnaUnit.Name}},
			fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
		))

		By("Expect that cancel func has been called")
		Eventually(cancelFunc.called).Should(BeTrue())
	})

	Context("when CRI is not containerd", func() {
		BeforeEach(func() {
			operatingSystemConfig.Spec.CRIConfig = nil
		})

		It("should not handle containerd configs", func() {
			waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
			waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

			By("Assert that files and units have been created")
			test.AssertNoDirectoryOnDisk(fakeFS, "/var/bin/containerruntimes")
			test.AssertNoDirectoryOnDisk(fakeFS, "/etc/containerd/certs.d")
			test.AssertNoDirectoryOnDisk(fakeFS, "/etc/containerd/conf.d")
			test.AssertNoDirectoryOnDisk(fakeFS, "/etc/systemd/system/containerd.service.d")
			test.AssertNoFileOnDisk(fakeFS, "/etc/containerd/config.toml")
		})
	})

	Context("in-place updates", func() {
		var (
			kubeletUnit                    extensionsv1alpha1.Unit
			kubeletFile, kubeletConfigFile extensionsv1alpha1.File
			kubeletFilePath                = "/opt/bin/kubelet"
			kubeletConfigFilePath          = kubelet.PathKubeletConfig
			nodeAgentConfig                *nodeagentconfigv1alpha1.NodeAgentConfiguration

			server *httptest.Server
		)

		BeforeEach(func() {
			operatingSystemConfig.Spec.InPlaceUpdates = &extensionsv1alpha1.InPlaceUpdates{
				OperatingSystemVersion: "1.2.3",
				KubeletVersion:         "1.31.3",
				CredentialsRotation:    nil,
			}

			kubeletUnit = extensionsv1alpha1.Unit{
				Name:      "kubelet.service",
				Enable:    ptr.To(true),
				Content:   ptr.To("#kubelet"),
				FilePaths: []string{kubeletFilePath, kubeletConfigFilePath},
			}

			kubeletFile = extensionsv1alpha1.File{
				Path: kubeletFilePath,
				Content: extensionsv1alpha1.FileContent{
					ImageRef: &extensionsv1alpha1.FileContentImageRef{
						FilePathInImage: "/kubelet",
						Image:           "kubelet:v1.31.3",
					},
				},
				Permissions: ptr.To[uint32](0755),
			}
			Expect(fakeFS.WriteFile(path.Join(imageMountDirectory, kubeletFile.Content.ImageRef.FilePathInImage), []byte("some-data"), 0755)).To(Succeed())

			kubeletConfigFile = extensionsv1alpha1.File{
				Path: kubeletConfigFilePath,
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data: utils.EncodeBase64([]byte(`apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
cpuManagerPolicy: none
evictionHard:
  imagefs.available: 5%
  imagefs.inodesFree: 5%
  memory.available: 100Mi
  nodefs.available: 5%
  nodefs.inodesFree: 5%
kubeReserved:
  cpu: 80m
  memory: 1Gi
  pid: 20k
`)),
					},
				},
				Permissions: ptr.To[uint32](0600),
			}

			nodeAgentConfig = &nodeagentconfigv1alpha1.NodeAgentConfiguration{
				APIServer: nodeagentconfigv1alpha1.APIServer{
					CABundle: []byte("new-ca-bundle"),
					Server:   "https://test-server",
				},
			}

			nodeAgentKubeconfig := getNodeAgentKubeConfig([]byte("old-ca-bundle"), nodeAgentConfig.APIServer.Server, "old-cert")
			Expect(fakeFS.WriteFile(nodeagentconfigv1alpha1.KubeconfigFilePath, []byte(nodeAgentKubeconfig), 0600)).To(Succeed())

			nodeAgentConfigFile := extensionsv1alpha1.File{
				Path: nodeagentconfigv1alpha1.ConfigFilePath,
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data: utils.EncodeBase64([]byte(`apiServer:
  caBundle: ` + utils.EncodeBase64(nodeAgentConfig.APIServer.CABundle) + `
  server: ` + nodeAgentConfig.APIServer.Server + `
apiVersion: nodeagent.config.gardener.cloud/v1alpha1
kind: NodeAgentConfiguration
`)),
					},
				},
				Permissions: ptr.To[uint32](0600),
			}

			operatingSystemConfig.Spec.Files = append(operatingSystemConfig.Spec.Files, kubeletConfigFile, kubeletFile, nodeAgentConfigFile)
			operatingSystemConfig.Spec.Units = append(operatingSystemConfig.Spec.Units, kubeletUnit)

			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				n, err := fmt.Fprintln(w, "OK")
				Expect(err).NotTo(HaveOccurred())
				Expect(n).To(BeNumerically(">", 0))
			}))

			DeferCleanup(func() {
				server.Close()
			})

			DeferCleanup(test.WithVars(
				&operatingsystemconfig.GetOSVersion, func() (*string, error) { return ptr.To("1.2.3"), nil },
				&operatingsystemconfig.KubeletHealthCheckRetryTimeout, 2*time.Second,
				&operatingsystemconfig.KubeletHealthCheckRetryInterval, 200*time.Millisecond,
				&healthcheckcontroller.DefaultKubeletHealthEndpoint, server.URL,
				&nodeagent.RequestAndStoreKubeconfig, func(_ context.Context, _ logr.Logger, fs afero.Afero, restConfig *rest.Config, _ string) error {
					nodeAgentConfig := &nodeagentconfigv1alpha1.NodeAgentConfiguration{
						APIServer: nodeagentconfigv1alpha1.APIServer{
							CABundle: []byte("new-ca-bundle"),
							Server:   "https://test-server",
						},
					}

					newKubeConfig := getNodeAgentKubeConfig(restConfig.TLSClientConfig.CAData, nodeAgentConfig.APIServer.Server, "new-cert")

					Expect(fs.WriteFile(nodeagentconfigv1alpha1.KubeconfigFilePath, []byte(newKubeConfig), 0600)).To(Succeed())

					return nil
				},
			))

			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.NodeAgentAuthorizer, true))
		})

		JustBeforeEach(func() {
			waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
			waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

			fakeDBus.Actions = nil // reset actions on dbus to not repeat assertions from above for update scenario

			patch := client.MergeFrom(node.DeepCopy())
			node.Status.Conditions = []corev1.NodeCondition{
				{
					Type:   machinev1alpha1.NodeInPlaceUpdate,
					Status: corev1.ConditionTrue,
					Reason: machinev1alpha1.ReadyForUpdate,
				},
			}
			Expect(testClient.Status().Patch(ctx, node, patch)).To(Succeed())
		})

		It("should succesfully update the OS", func() {
			operatingSystemConfig.Spec.InPlaceUpdates.OperatingSystemVersion = "1.2.4"
			operatingSystemConfig.Status.InPlaceUpdates = &extensionsv1alpha1.InPlaceUpdatesStatus{
				OSUpdate: &extensionsv1alpha1.OSUpdate{
					Command: "echo 'OS update successful'",
				},
			}

			DeferCleanup(test.WithVar(&operatingsystemconfig.ExecCommandCombinedOutput, func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return []byte("OS update successful"), nil
			}))

			var err error
			oscRaw, err = runtime.Encode(codec, operatingSystemConfig)
			Expect(err).NotTo(HaveOccurred())

			By("Update Secret containing the operating system config")
			patch := client.MergeFrom(oscSecret.DeepCopy())
			oscSecret.Annotations["checksum/data-script"] = utils.ComputeSHA256Hex(oscRaw)
			oscSecret.Data["osc.yaml"] = oscRaw
			Expect(testClient.Patch(ctx, oscSecret, patch)).To(Succeed())

			waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
			waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Labels).To(HaveKeyWithValue(machinev1alpha1.LabelKeyNodeUpdateResult, machinev1alpha1.LabelValueNodeUpdateSuccessful))
		})

		It("should successfully update the kubelet config", func() {
			Expect(fakeFS.WriteFile("/var/lib/kubelet/cpu_manager_state", []byte("some-data"), 0755)).To(Succeed())

			kubeletConfig := `apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
cpuManagerPolicy: static
evictionHard:
  imagefs.available: 6%
  imagefs.inodesFree: 6%
  memory.available: 200Mi
  nodefs.available: 6%
  nodefs.inodesFree: 6%
kubeReserved:
  cpu: 90m
  memory: 900Mi
  pid: 25k
`

			kubeletConfigFileIndex := slices.IndexFunc(operatingSystemConfig.Spec.Files, func(f extensionsv1alpha1.File) bool {
				return f.Path == kubeletConfigFilePath
			})
			Expect(kubeletConfigFileIndex).To(BeNumerically(">=", 0))
			operatingSystemConfig.Spec.Files[kubeletConfigFileIndex].Content.Inline.Data = utils.EncodeBase64([]byte(kubeletConfig))

			var err error
			oscRaw, err = runtime.Encode(codec, operatingSystemConfig)
			Expect(err).NotTo(HaveOccurred())

			By("Update Secret containing the operating system config")
			patch := client.MergeFrom(oscSecret.DeepCopy())
			oscSecret.Annotations["checksum/data-script"] = utils.ComputeSHA256Hex(oscRaw)
			oscSecret.Data["osc.yaml"] = oscRaw
			Expect(testClient.Patch(ctx, oscSecret, patch)).To(Succeed())

			waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
			waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

			By("Assert that unit actions have been applied")
			Expect(fakeDBus.Actions).To(ConsistOf(
				fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{kubeletUnit.Name}},
				fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{kubeletUnit.Name}},
				fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
			))

			Expect(afero.Exists(fakeFS, "/var/lib/kubelet/cpu_manager_state")).To(BeFalse())
			test.AssertFileOnDisk(fakeFS, kubeletConfigFilePath, kubeletConfig, 0600)

			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Labels).To(HaveKeyWithValue(machinev1alpha1.LabelKeyNodeUpdateResult, machinev1alpha1.LabelValueNodeUpdateSuccessful))
		})

		It("should successfully update the kubelet minor version", func() {
			operatingSystemConfig.Spec.InPlaceUpdates.KubeletVersion = "1.32.1"

			kubeletFileIndex := slices.IndexFunc(operatingSystemConfig.Spec.Files, func(f extensionsv1alpha1.File) bool {
				return f.Path == kubeletFilePath
			})
			Expect(kubeletFileIndex).To(BeNumerically(">=", 0))
			operatingSystemConfig.Spec.Files[kubeletFileIndex].Content.ImageRef.Image = "kubelet:v1.32.1"

			var err error
			oscRaw, err = runtime.Encode(codec, operatingSystemConfig)
			Expect(err).NotTo(HaveOccurred())

			By("Update Secret containing the operating system config")
			patch := client.MergeFrom(oscSecret.DeepCopy())
			oscSecret.Annotations["checksum/data-script"] = utils.ComputeSHA256Hex(oscRaw)
			oscSecret.Data["osc.yaml"] = oscRaw
			Expect(testClient.Patch(ctx, oscSecret, patch)).To(Succeed())

			waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
			waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

			By("Assert that unit actions have been applied")
			Expect(fakeDBus.Actions).To(ConsistOf(
				fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{kubeletUnit.Name}},
				fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{kubeletUnit.Name}},
				fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
			))

			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Labels).To(HaveKeyWithValue(machinev1alpha1.LabelKeyNodeUpdateResult, machinev1alpha1.LabelValueNodeUpdateSuccessful))
		})

		It("should successfully complete service account key rotation", func() {
			operatingSystemConfig.Spec.InPlaceUpdates.CredentialsRotation = &extensionsv1alpha1.CredentialsRotation{
				ServiceAccountKey: &extensionsv1alpha1.ServiceAccountKeyRotation{
					LastInitiationTime: &metav1.Time{Time: time.Now().Add(-time.Hour)},
				},
			}

			var err error
			oscRaw, err = runtime.Encode(codec, operatingSystemConfig)
			Expect(err).NotTo(HaveOccurred())

			By("Update Secret containing the operating system config")
			patch := client.MergeFrom(oscSecret.DeepCopy())
			oscSecret.Annotations["checksum/data-script"] = utils.ComputeSHA256Hex(oscRaw)
			oscSecret.Data["osc.yaml"] = oscRaw
			Expect(testClient.Patch(ctx, oscSecret, patch)).To(Succeed())

			for _, secretName := range []string{secretName1, secretName2} {
				var event event.TypedGenericEvent[*corev1.Secret]
				Eventually(func(g Gomega) {
					g.Expect(channel).To(Receive(&event))
				}).Should(Succeed())

				Expect(event.Object.GetName()).To(Equal(secretName))
				Expect(event.Object.GetNamespace()).To(Equal("kube-system"))
			}

			waitForUpdatedNodeAnnotationCloudConfig(node, utils.ComputeSHA256Hex(oscRaw))
			waitForUpdatedNodeLabelKubernetesVersion(node, kubernetesVersion.String())

			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Labels).To(HaveKeyWithValue(machinev1alpha1.LabelKeyNodeUpdateResult, machinev1alpha1.LabelValueNodeUpdateSuccessful))
		})

		It("should successfully complete CA rotation", func() {
			operatingSystemConfig.Spec.InPlaceUpdates.CredentialsRotation = &extensionsv1alpha1.CredentialsRotation{
				CertificateAuthorities: &extensionsv1alpha1.CARotation{
					LastInitiationTime: &metav1.Time{Time: time.Now().Add(-time.Hour)},
				},
			}

			kubeletCertPath := filepath.Join(kubelet.PathKubeletDirectory, "pki", "kubelet-client-current.pem")
			kubeletCertDir := filepath.Join(kubelet.PathKubeletDirectory, "pki")
			fakeKubeConfig := `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ` + utils.EncodeBase64([]byte("test-ca-bundle")) + `
    server: https://test-server
  name: default-cluster
contexts:
- context:
    cluster: test-cluster
    user: system:node:test-node
  name: test-context
current-context: test-context
kind: Config
preferences: {}
`

			Expect(fakeFS.WriteFile(kubeletCertPath, []byte("test-cert"), 0600)).To(Succeed())
			Expect(fakeFS.WriteFile(kubelet.PathKubeconfigReal, []byte(fakeKubeConfig), 0600)).To(Succeed())

			var err error
			oscRaw, err = runtime.Encode(codec, operatingSystemConfig)
			Expect(err).NotTo(HaveOccurred())

			By("Update Secret containing the operating system config")
			patch := client.MergeFrom(oscSecret.DeepCopy())
			oscSecret.Annotations["checksum/data-script"] = utils.ComputeSHA256Hex(oscRaw)
			oscSecret.Data["osc.yaml"] = oscRaw
			Expect(testClient.Patch(ctx, oscSecret, patch)).To(Succeed())

			By("Assert that unit actions have been applied")
			Eventually(func(g Gomega) {
				g.Expect(fakeDBus.Actions).To(ConsistOf(
					fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{kubeletUnit.Name}},
					fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
				))
			}).Should(Succeed())

			Expect(cancelFunc.called).To(BeTrue())

			Expect(fakeFS.DirExists(kubeletCertDir)).To(BeFalse())
			expectedBootStrapConfig := `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ` + utils.EncodeBase64(nodeAgentConfig.APIServer.CABundle) + `
    server: ` + nodeAgentConfig.APIServer.Server + `
  name: default-cluster
contexts:
- context:
    cluster: test-cluster
    user: system:node:test-node
  name: test-context
current-context: test-context
kind: Config
preferences: {}
users:
- name: default-auth
  user:
    client-certificate-data: ` + utils.EncodeBase64([]byte("test-cert")) + `
    client-key-data: ` + utils.EncodeBase64([]byte("test-cert")) + `
`
			test.AssertFileOnDisk(fakeFS, kubelet.PathKubeconfigBootstrap, expectedBootStrapConfig, 0600)

			// Verify the kubeconfig has the latest CA
			expectedNodeAgentKubeConfig := getNodeAgentKubeConfig(nodeAgentConfig.APIServer.CABundle, nodeAgentConfig.APIServer.Server, "new-cert")
			test.AssertFileOnDisk(fakeFS, nodeagentconfigv1alpha1.KubeconfigFilePath, expectedNodeAgentKubeConfig, 0600)

			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Labels).To(HaveKeyWithValue(machinev1alpha1.LabelKeyNodeUpdateResult, machinev1alpha1.LabelValueNodeUpdateSuccessful))
		})
	})
})

type cancelFuncEnsurer struct {
	called bool
}

func (c *cancelFuncEnsurer) cancel() {
	c.called = true
}

func waitForUpdatedNodeAnnotationCloudConfig(node *corev1.Node, value string) {
	By("Wait for node annotations to be updated")
	EventuallyWithOffset(1, func(g Gomega) map[string]string {
		updatedNode := &corev1.Node{}
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
		return updatedNode.Annotations
	}).Should(HaveKeyWithValue("checksum/cloud-config-data", value))
}

func waitForUpdatedNodeLabelKubernetesVersion(node *corev1.Node, value string) {
	By("Wait for node labels to be updated")
	EventuallyWithOffset(1, func(g Gomega) map[string]string {
		updatedNode := &corev1.Node{}
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), updatedNode)).To(Succeed())
		return updatedNode.Labels
	}).Should(HaveKeyWithValue("worker.gardener.cloud/kubernetes-version", value))
}

func getNodeAgentKubeConfig(caBundle []byte, server, clientCertificate string) string {
	return `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ` + utils.EncodeBase64(caBundle) + `
    server: ` + server + `
  name: node-agent
contexts:
- context:
    cluster: node-agent
    user: node-agent
  name: node-agent
current-context: node-agent
kind: Config
preferences: {}
users:
- name: node-agent
  user:
    client-certificate-data: ` + utils.EncodeBase64([]byte(clientCertificate)) + `
    client-key-data: ` + utils.EncodeBase64([]byte(clientCertificate)) + `
`
}
