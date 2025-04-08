// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/gardener/gardener/pkg/api/indexer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubeletcomponent "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	healthcheckcontroller "github.com/gardener/gardener/pkg/nodeagent/controller/healthcheck"
	fakedbus "github.com/gardener/gardener/pkg/nodeagent/dbus/fake"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx             context.Context
		fs              afero.Afero
		fakeDBus        *fakedbus.DBus
		c               client.Client
		reconciler      *Reconciler
		node            *corev1.Node
		nodeAgentConfig *nodeagentconfigv1alpha1.NodeAgentConfiguration
		log             logr.Logger

		kubeletCertPath string
		kubeletCertDir  string
		fakeKubeConfig  string
	)

	BeforeEach(func() {
		ctx = context.TODO()
		fs = afero.Afero{Fs: afero.NewMemMapFs()}
		fakeDBus = fakedbus.New()
		log = logr.Discard()
		c = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.SeedScheme).
			WithIndex(&corev1.Pod{}, indexer.PodNodeName, indexer.PodNodeNameIndexerFunc).
			Build()

		reconciler = &Reconciler{
			Client:   c,
			FS:       fs,
			DBus:     fakeDBus,
			Recorder: nil,
		}

		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
			},
		}

		Expect(c.Create(ctx, node)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(c.Delete(ctx, node))).To(Succeed())
		})

		nodeAgentConfig = &nodeagentconfigv1alpha1.NodeAgentConfiguration{
			APIServer: nodeagentconfigv1alpha1.APIServer{
				CABundle: []byte("new-ca-bundle"),
				Server:   "https://test-server",
			},
		}

		fakeKubeConfig = `apiVersion: v1
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

		kubeletCertPath = filepath.Join(kubeletcomponent.PathKubeletDirectory, "pki", "kubelet-client-current.pem")
		kubeletCertDir = filepath.Join(kubeletcomponent.PathKubeletDirectory, "pki")
	})

	Context("#GetOSVersion", func() {
		BeforeEach(func() {
			Expect(fs.WriteFile("/etc/os-release", []byte(`ID=gardenlinux
NAME="Garden Linux"
PRETTY_NAME="Garden Linux 1592.6"
HOME_URL="https://gardenlinux.io"
SUPPORT_URL="https://github.com/gardenlinux/gardenlinux"
BUG_REPORT_URL="https://github.com/gardenlinux/gardenlinux/issues"
GARDENLINUX_CNAME=gardener_prod-amd64-1592.6
GARDENLINUX_FEATURES=log
GARDENLINUX_VERSION=1592.6
GARDENLINUX_COMMIT_ID=5b20e1c
GARDENLINUX_COMMIT_ID_LONG=5b20e1c0436d229b051f0481a72d0a366315b220
`), 0600)).To(Succeed())
		})

		It("should return the OS version", func() {
			version, err := GetOSVersion(&extensionsv1alpha1.InPlaceUpdates{}, fs)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(PointTo(Equal("1592.6")))
		})

		It("should return nil when inPlaceUpdates is nil", func() {
			version, err := GetOSVersion(nil, fs)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(BeNil())
		})

		It("should return an error when os-release file does not exist", func() {
			Expect(fs.Remove("/etc/os-release")).To(Succeed())

			version, err := GetOSVersion(&extensionsv1alpha1.InPlaceUpdates{}, fs)
			Expect(err).To(MatchError(ContainSubstring("file does not exist")))
			Expect(version).To(BeNil())
		})

		It("should return an error when it is not able to get the version", func() {
			Expect(fs.WriteFile("/etc/os-release", []byte(`ID=gardenlinux
NAME="Garden Linux"
`), 0600)).To(Succeed())

			version, err := GetOSVersion(&extensionsv1alpha1.InPlaceUpdates{}, fs)
			Expect(err).To(MatchError(ContainSubstring("unable to find version")))
			Expect(version).To(BeNil())
		})

		It("should return an error when it is not able to parse the version", func() {
			Expect(fs.WriteFile("/etc/os-release", []byte(`ID=gardenlinux
NAME="Garden Linux"
PRETTY_NAME="Garden Linux 1592"
`), 0600)).To(Succeed())

			version, err := GetOSVersion(&extensionsv1alpha1.InPlaceUpdates{}, fs)
			Expect(err).To(MatchError(ContainSubstring("unable to find version")))
			Expect(version).To(BeNil())
		})
	})

	Context("#deleteRemainingPods", func() {
		It("should delete all pods running on this node", func() {
			pods := []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod-1",
					},
					Spec: corev1.PodSpec{
						NodeName: "test-node",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod-2",
					},
					Spec: corev1.PodSpec{
						NodeName: "test-node",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod-3",
					},
					Spec: corev1.PodSpec{
						NodeName: "another-node",
					},
				},
			}

			for _, pod := range pods {
				Expect(c.Create(ctx, pod)).To(Succeed())
			}

			DeferCleanup(func() {
				Expect(c.DeleteAllOf(ctx, &corev1.Pod{})).To(Or(Succeed(), BeNotFoundError()))
			})

			Expect(reconciler.deleteRemainingPods(ctx, log, node)).To(Succeed())

			podList := &corev1.PodList{}
			Expect(c.List(ctx, podList)).To(Succeed())
			Expect(podList.Items).To(HaveLen(1))
			Expect(podList.Items[0].Name).To(Equal("pod-3"))
		})
	})

	Context("#updateOSInPlace", func() {
		var (
			osc        *extensionsv1alpha1.OperatingSystemConfig
			oscChanges *operatingSystemConfigChanges
		)

		BeforeEach(func() {
			osc = &extensionsv1alpha1.OperatingSystemConfig{
				Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
					InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdates{
						OperatingSystemVersion: "1.2.3",
					},
				},
				Status: extensionsv1alpha1.OperatingSystemConfigStatus{
					InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdatesStatus{
						OSUpdate: &extensionsv1alpha1.OSUpdate{
							Command: "/bin/echo",
							Args:    []string{"OS update successful"},
						},
					},
				},
			}

			oscChanges = &operatingSystemConfigChanges{
				InPlaceUpdates: inPlaceUpates{
					OperatingSystem: true,
				},
			}

			DeferCleanup(test.WithVars(
				&OSUpdateRetryInterval, 1*time.Millisecond,
				&OSUpdateRetryTimeout, 10*time.Millisecond,
			))
		})

		It("should return nil if oscChanges.OSVersion.Changed is false", func() {
			oscChanges.InPlaceUpdates.OperatingSystem = false

			Expect(reconciler.updateOSInPlace(ctx, log, oscChanges, osc, node)).To(Succeed())
		})

		It("should successfully execute the update command and patch the node", func() {
			DeferCleanup(test.WithVar(&ExecCommandCombinedOutput, func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return []byte("OS update successful"), nil
			}))

			Expect(reconciler.updateOSInPlace(ctx, log, oscChanges, osc, node)).To(Succeed())

			Expect(reconciler.Client.Get(ctx, client.ObjectKey{Name: node.Name}, node)).To(Succeed())
			Expect(node.Annotations).To(HaveKeyWithValue("node-agent.gardener.cloud/updating-operating-system-version", "1.2.3"))
			Expect(node.Labels).NotTo(HaveKeyWithValue(machinev1alpha1.LabelKeyNodeUpdateResult, machinev1alpha1.LabelValueNodeUpdateFailed))
		})

		It("should return an error if the update command is not provided", func() {
			osc.Status.InPlaceUpdates.OSUpdate.Command = ""

			err := reconciler.updateOSInPlace(ctx, log, oscChanges, osc, node)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("update command is not provided in OSC"))
		})

		It("should return an error if spec.InPlaceUpdates.OperatingSystemVersion is not provided", func() {
			osc.Spec.InPlaceUpdates.OperatingSystemVersion = ""

			err := reconciler.updateOSInPlace(ctx, log, oscChanges, osc, node)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("operating system version is not provided in OSC"))
		})

		It("should return an error if the update command fails with a retriable error", func() {
			DeferCleanup(test.WithVar(&ExecCommandCombinedOutput, func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return []byte("network problems"), errors.New("command failed")
			}))

			Expect(reconciler.updateOSInPlace(ctx, log, oscChanges, osc, node)).To(Succeed())

			Expect(reconciler.Client.Get(ctx, client.ObjectKey{Name: node.Name}, node)).To(Succeed())
			Expect(node.Annotations).To(HaveKeyWithValue("node-agent.gardener.cloud/updating-operating-system-version", "1.2.3"))
			Expect(node.Labels).To(HaveKeyWithValue(machinev1alpha1.LabelKeyNodeUpdateResult, machinev1alpha1.LabelValueNodeUpdateFailed))
			Expect(node.Annotations).To(HaveKeyWithValue(machinev1alpha1.AnnotationKeyMachineUpdateFailedReason, ContainSubstring("retriable error detected: command failed, output: network problems")))
		})

		It("should return an error if the update command fails with a non-retriable error", func() {
			DeferCleanup(test.WithVar(&ExecCommandCombinedOutput, func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				return []byte("invalid arguments"), errors.New("command failed")
			}))

			Expect(reconciler.updateOSInPlace(ctx, log, oscChanges, osc, node)).To(Succeed())

			Expect(reconciler.Client.Get(ctx, client.ObjectKey{Name: node.Name}, node)).To(Succeed())
			Expect(node.Annotations).To(HaveKeyWithValue("node-agent.gardener.cloud/updating-operating-system-version", "1.2.3"))
			Expect(node.Labels).To(HaveKeyWithValue(machinev1alpha1.LabelKeyNodeUpdateResult, machinev1alpha1.LabelValueNodeUpdateFailed))
			Expect(node.Annotations).To(HaveKeyWithValue(machinev1alpha1.AnnotationKeyMachineUpdateFailedReason, ContainSubstring("non-retriable error detected: command failed, output: invalid arguments")))
		})
	})

	Context("#performInPlaceUpdate", func() {
		var (
			osc        *extensionsv1alpha1.OperatingSystemConfig
			oscChanges *operatingSystemConfigChanges
			osVersion  = "1.2.3"
		)

		BeforeEach(func() {
			osc = &extensionsv1alpha1.OperatingSystemConfig{
				Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
					InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdates{
						OperatingSystemVersion: osVersion,
					},
				},
				Status: extensionsv1alpha1.OperatingSystemConfigStatus{
					InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdatesStatus{
						OSUpdate: &extensionsv1alpha1.OSUpdate{
							Command: "/bin/echo",
							Args:    []string{"OS update successful"},
						},
					},
				},
			}

			oscChanges = &operatingSystemConfigChanges{}

			DeferCleanup(test.WithVars(
				&OSUpdateRetryInterval, 1*time.Millisecond,
				&OSUpdateRetryTimeout, 10*time.Millisecond,
			))
		})

		It("should return nil if node is nil", func() {
			Expect(reconciler.performInPlaceUpdate(ctx, log, osc, oscChanges, nil, &osVersion)).To(Succeed())
		})

		It("should set the node to update-failed if the lastAttempted version is equal to the osc.Spec.InPlaceUpdates.OperatingSystemVersion", func() {
			node.Annotations = map[string]string{"node-agent.gardener.cloud/updating-operating-system-version": "1.2.4"}
			osc.Spec.InPlaceUpdates.OperatingSystemVersion = "1.2.4"

			Expect(reconciler.performInPlaceUpdate(ctx, log, osc, oscChanges, node, &osVersion)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Labels).To(HaveKeyWithValue(machinev1alpha1.LabelKeyNodeUpdateResult, machinev1alpha1.LabelValueNodeUpdateFailed))
			Expect(node.Annotations).To(HaveKeyWithValue(machinev1alpha1.AnnotationKeyMachineUpdateFailedReason, ContainSubstring("OS update might have failed and rolled back to the previous version. Desired version: %q, Current version: %q", "1.2.4", "1.2.3")))
		})

		It("should not patch the node as update successful or delete the pods if the node deoes not have InPlaceUpdate condition with reason ReadyForUpdate", func() {
			pods := []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod-1",
					},
					Spec: corev1.PodSpec{
						NodeName: "test-node",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod-2",
					},
					Spec: corev1.PodSpec{
						NodeName: "test-node",
					},
				},
			}

			for _, pod := range pods {
				Expect(c.Create(ctx, pod)).To(Succeed())
			}

			DeferCleanup(func() {
				Expect(c.DeleteAllOf(ctx, &corev1.Pod{})).To(Or(Succeed(), BeNotFoundError()))
			})

			Expect(reconciler.performInPlaceUpdate(ctx, log, osc, oscChanges, node, &osVersion)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Labels).NotTo(HaveKey(machinev1alpha1.LabelKeyNodeUpdateResult))

			podList := &corev1.PodList{}
			Expect(c.List(ctx, podList)).To(Succeed())
			Expect(podList.Items).To(HaveLen(2))
		})

		It("should patch the node as update successful and delete the pods if the node has InPlaceUpdate condition with reason ReadyForUpdate", func() {
			node.Status.Conditions = []corev1.NodeCondition{
				{
					Type:   machinev1alpha1.NodeInPlaceUpdate,
					Status: corev1.ConditionTrue,
					Reason: machinev1alpha1.ReadyForUpdate,
				},
			}

			pods := []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod-1",
					},
					Spec: corev1.PodSpec{
						NodeName: "test-node",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod-2",
					},
					Spec: corev1.PodSpec{
						NodeName: "test-node",
					},
				},
			}

			for _, pod := range pods {
				Expect(c.Create(ctx, pod)).To(Succeed())
			}

			DeferCleanup(func() {
				Expect(c.DeleteAllOf(ctx, &corev1.Pod{})).To(Or(Succeed(), BeNotFoundError()))
			})

			Expect(reconciler.performInPlaceUpdate(ctx, log, osc, oscChanges, node, &osVersion)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Labels).To(HaveKeyWithValue(machinev1alpha1.LabelKeyNodeUpdateResult, machinev1alpha1.LabelValueNodeUpdateSuccessful))

			podList := &corev1.PodList{}
			Expect(c.List(ctx, podList)).To(Succeed())
			Expect(podList.Items).To(BeEmpty())
		})
	})

	Context("#checkKubeletHealth", func() {
		var server *httptest.Server

		BeforeEach(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				n, err := fmt.Fprintln(w, "OK")
				Expect(err).NotTo(HaveOccurred())
				Expect(n).To(BeNumerically(">", 0))
			}))

			DeferCleanup(func() {
				server.Close()
			})

			DeferCleanup(test.WithVars(
				&KubeletHealthCheckRetryInterval, 1*time.Millisecond,
				&KubeletHealthCheckRetryTimeout, 10*time.Millisecond,
			))
		})

		It("should mark the node as failed when kubelet health check fails", func() {
			err := reconciler.checkKubeletHealth(ctx, log, node)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("HTTP request to kubelet health endpoint failed"))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Annotations).To(HaveKeyWithValue(machinev1alpha1.AnnotationKeyMachineUpdateFailedReason, ContainSubstring("HTTP request to kubelet health endpoint failed")))
		})

		It("should succeed when kubelet health endpoint returns OK", func() {
			DeferCleanup(test.WithVar(&healthcheckcontroller.DefaultKubeletHealthEndpoint, server.URL))

			Expect(reconciler.checkKubeletHealth(ctx, log, node)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Annotations).NotTo(HaveKey(machinev1alpha1.AnnotationKeyMachineUpdateFailedReason))
		})

		It("should fail when the retry times out", func() {
			DeferCleanup(test.WithVar(&healthcheckcontroller.DefaultKubeletHealthEndpoint, server.URL))

			DeferCleanup(test.WithVar(&retry.UntilTimeout, func(_ context.Context, _, _ time.Duration, _ retry.Func) error {
				return errors.New("timeout reached")
			}))

			err := reconciler.checkKubeletHealth(ctx, log, node)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("timeout reached"))
			Expect(node.Annotations).To(HaveKeyWithValue(machinev1alpha1.AnnotationKeyMachineUpdateFailedReason, "kubelet is not healthy after in-place update: timeout reached"))
		})

		It("should fail when patching the node fails", func() {
			Expect(c.Delete(ctx, node)).To(Succeed())

			Expect(reconciler.checkKubeletHealth(ctx, log, node)).To(BeNotFoundError())
		})
	})

	Context("#completeKubeletInPlaceUpdate", func() {
		var (
			oscChanges *operatingSystemConfigChanges
			server     *httptest.Server
		)

		BeforeEach(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				n, err := fmt.Fprintln(w, "OK")
				Expect(err).NotTo(HaveOccurred())
				Expect(n).To(BeNumerically(">", 0))
			}))

			DeferCleanup(func() {
				server.Close()
			})

			oscChanges = &operatingSystemConfigChanges{
				InPlaceUpdates: inPlaceUpates{
					Kubelet: kubelet{
						MinorVersion:     true,
						Config:           true,
						CPUManagerPolicy: true,
					},
				},
				fs: fs,
			}

			DeferCleanup(test.WithVars(
				&KubeletHealthCheckRetryInterval, 1*time.Millisecond,
				&KubeletHealthCheckRetryTimeout, 10*time.Millisecond,
			))
		})

		It("should successfully complete kubelet in-place update", func() {
			DeferCleanup(test.WithVar(&healthcheckcontroller.DefaultKubeletHealthEndpoint, server.URL))

			Expect(reconciler.completeKubeletInPlaceUpdate(ctx, log, oscChanges, node)).To(Succeed())

			Expect(oscChanges.InPlaceUpdates.Kubelet.MinorVersion).To(BeFalse())
			Expect(oscChanges.InPlaceUpdates.Kubelet.Config).To(BeFalse())
			Expect(oscChanges.InPlaceUpdates.Kubelet.CPUManagerPolicy).To(BeFalse())
		})

		It("should fail if kubelet health check fails", func() {
			err := reconciler.completeKubeletInPlaceUpdate(ctx, log, oscChanges, node)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("kubelet is not healthy after minor version/config update"))
		})
	})

	Context("#rebootstrapKubelet", func() {
		It("should successfully rebootstrap kubelet", func() {
			Expect(fs.WriteFile(kubeletCertPath, []byte("test-cert"), 0600)).To(Succeed())
			Expect(fs.WriteFile(kubeletcomponent.PathKubeconfigReal, []byte(fakeKubeConfig), 0600)).To(Succeed())

			Expect(reconciler.rebootstrapKubelet(ctx, log, nodeAgentConfig, node)).To(Succeed())

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
			test.AssertFileOnDisk(fs, kubeletcomponent.PathKubeconfigBootstrap, expectedBootStrapConfig, 0600)

			Expect(fs.DirExists(kubeletCertDir)).To(BeFalse())

			Expect(fakeDBus.Actions).To(ContainElement(fakedbus.SystemdAction{
				Action:    fakedbus.ActionRestart,
				UnitNames: []string{"kubelet.service"},
			}))
		})

		It("should not fail if kubelet client certificate is missing but bootstrap file exists", func() {
			Expect(fs.WriteFile(kubeletcomponent.PathKubeconfigBootstrap, []byte("test-bootstrap-kubeconfig"), 0600)).To(Succeed())

			Expect(reconciler.rebootstrapKubelet(ctx, log, nodeAgentConfig, node)).To(Succeed())

			Expect(fs.DirExists(kubeletCertDir)).To(BeFalse())

			Expect(fakeDBus.Actions).To(ContainElement(fakedbus.SystemdAction{
				Action:    fakedbus.ActionRestart,
				UnitNames: []string{"kubelet.service"},
			}))
		})

		It("should fail if kubelet client certificate and bootstrap file are missing", func() {
			err := reconciler.rebootstrapKubelet(ctx, log, nodeAgentConfig, node)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("kubeconfig file %q does not exist, cannot proceed with rebootstrap", kubeletcomponent.PathKubeconfigBootstrap))
		})

		It("should fail if kubeconfig file cannot be loaded", func() {
			Expect(fs.WriteFile(kubeletcomponent.PathKubeconfigReal, []byte("invalid-kubeconfig"), 0600)).To(Succeed())

			err := reconciler.rebootstrapKubelet(ctx, log, nodeAgentConfig, node)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to load kubeconfig file"))
		})

		It("should fail if DBus restart fails", func() {
			Expect(fs.WriteFile(kubeletcomponent.PathKubeconfigBootstrap, []byte("test-bootstrap-kubeconfig"), 0600)).To(Succeed())

			// Inject DBus restart failure
			fakeDBus.InjectRestartFailure(errors.New("restart failed"), "kubelet.service")

			err := reconciler.rebootstrapKubelet(ctx, log, nodeAgentConfig, node)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to restart unit"))
		})
	})

	Context("#performCredentialsRotationInPlace", func() {
		var (
			channel    chan event.TypedGenericEvent[*corev1.Secret]
			oscChanges *operatingSystemConfigChanges
			server     *httptest.Server

			nodeAgentConfigFile []byte
		)

		BeforeEach(func() {
			channel = make(chan event.TypedGenericEvent[*corev1.Secret])
			oscChanges = &operatingSystemConfigChanges{
				fs: fs,
			}

			reconciler.Channel = channel

			nodeAgentConfigFile = []byte(`apiServer:
  caBundle: ` + utils.EncodeBase64(nodeAgentConfig.APIServer.CABundle) + `
  server: ` + nodeAgentConfig.APIServer.Server + `
apiVersion: nodeagent.config.gardener.cloud/v1alpha1
kind: NodeAgentConfiguration
`)
			Expect(fs.WriteFile(nodeagentconfigv1alpha1.ConfigFilePath, nodeAgentConfigFile, 0600)).To(Succeed())

			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				n, err := fmt.Fprintln(w, "OK")
				Expect(err).NotTo(HaveOccurred())
				Expect(n).To(BeNumerically(">", 0))
			}))

			DeferCleanup(func() {
				server.Close()
			})

			DeferCleanup(test.WithVars(
				&KubeletHealthCheckRetryInterval, 1*time.Millisecond,
				&KubeletHealthCheckRetryTimeout, 10*time.Millisecond,
				&healthcheckcontroller.DefaultKubeletHealthEndpoint, server.URL,
			))
		})

		It("should successfully generate events for service account key rotation", func(ctx context.Context) {
			oscChanges.InPlaceUpdates.ServiceAccountKeyRotation = true
			reconciler.TokenSecretSyncConfigs = []nodeagentconfigv1alpha1.TokenSecretSyncConfig{
				{SecretName: "test-token-1"},
				{SecretName: "test-token-2"},
			}

			go func() {
				defer GinkgoRecover()
				expectedSecrets := []string{"test-token-1", "test-token-2"}
				for _, secretName := range expectedSecrets {
					event := <-channel
					Expect(event.Object.GetName()).To(Equal(secretName))
					Expect(event.Object.GetNamespace()).To(Equal("kube-system"))
				}
			}()

			Expect(reconciler.performCredentialsRotationInPlace(ctx, log, oscChanges, node)).To(Succeed())
			Expect(oscChanges.InPlaceUpdates.ServiceAccountKeyRotation).To(BeFalse())
		}, NodeTimeout(time.Second*5))

		It("should successfully rotate CA certificate for kubelet", func() {
			Expect(fs.WriteFile(kubeletCertPath, []byte("test-cert"), 0600)).To(Succeed())
			Expect(fs.WriteFile(kubeletcomponent.PathKubeconfigReal, []byte(fakeKubeConfig), 0600)).To(Succeed())

			oscChanges.InPlaceUpdates.CertificateAuthoritiesRotation.Kubelet = true

			Expect(reconciler.performCredentialsRotationInPlace(ctx, log, oscChanges, node)).To(Succeed())
			Expect(fakeDBus.Actions).To(ContainElement(fakedbus.SystemdAction{
				Action:    fakedbus.ActionRestart,
				UnitNames: []string{"kubelet.service"},
			}))

			Expect(oscChanges.InPlaceUpdates.CertificateAuthoritiesRotation.Kubelet).To(BeFalse())
		})

		It("should successfully rotate the CA certificate for gardener-node-agent", func() {
			nodeAgentKubeconfig := getNodeAgentKubeConfig([]byte("old-ca-bundle"), nodeAgentConfig.APIServer.Server, "old-cert")
			Expect(fs.WriteFile(nodeagentconfigv1alpha1.KubeconfigFilePath, []byte(nodeAgentKubeconfig), 0600)).To(Succeed())

			DeferCleanup(test.WithVar(
				&RequestAndStoreKubeconfig, func(_ context.Context, _ logr.Logger, _ afero.Afero, restConfig *rest.Config, _ string) error {
					newKubeConfig := getNodeAgentKubeConfig(restConfig.TLSClientConfig.CAData, nodeAgentConfig.APIServer.Server, "new-cert")

					Expect(fs.WriteFile(nodeagentconfigv1alpha1.KubeconfigFilePath, []byte(newKubeConfig), 0600)).To(Succeed())

					return nil
				},
			))

			oscChanges.InPlaceUpdates.CertificateAuthoritiesRotation.NodeAgent = true

			Expect(reconciler.performCredentialsRotationInPlace(ctx, log, oscChanges, node)).To(Succeed())

			Expect(oscChanges.InPlaceUpdates.CertificateAuthoritiesRotation.NodeAgent).To(BeFalse())
			Expect(oscChanges.MustRestartNodeAgent).To(BeTrue())

			expectedNodeAgentKubeConfig := getNodeAgentKubeConfig(nodeAgentConfig.APIServer.CABundle, nodeAgentConfig.APIServer.Server, "new-cert")
			test.AssertFileOnDisk(fs, nodeagentconfigv1alpha1.KubeconfigFilePath, expectedNodeAgentKubeConfig, 0600)
		})
	})
})

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
