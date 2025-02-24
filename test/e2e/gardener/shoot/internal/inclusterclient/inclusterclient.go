// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package inclusterclient

import (
	"context"
	"io"
	"maps"
	"net"
	"regexp"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

const (
	name      = "e2e-test-in-cluster-client"
	namespace = metav1.NamespaceDefault

	podNameDirect                 = name + "-direct-"
	podNameAPIServerProxyIP       = name + "-apiserver-proxy-ip-"
	podNameAPIServerProxyHostname = name + "-apiserver-proxy-hostname-"
	containerName                 = "kubectl"
)

var labels = map[string]string{"e2e-test": "in-cluster-client"}

// VerifyInClusterAccessToAPIServer verifies access to the shoot API server from in-cluster clients.
// It verifies the following paths:
// - direct path: connecting to the API server's internal domain (injected by gardener into the KUBERNETES_SERVICE_HOST env var)
// - via the API server proxy: connecting to the kubernetes service's clusterIP
// - via the API server proxy: connecting to the kubernetes service's hostname (kubernetes.default.svc.cluster.local)
// For this, it deploys three pods with the hyperkube image that contains the kubectl binary serving as a test client.
// Apart from testing the actual connection, the test also verifies gardener's injection of the KUBERNETES_SERVICE_HOST
// env var:
// - one pod uses the injected env var
// - one pod disables injection and relies on the default service link env vars
// - one pod explicitly overwrites the env var
// See docs/usage/networking/shoot_kubernetes_service_host_injection.md and docs/proposals/08-shoot-apiserver-via-sni.md
func VerifyInClusterAccessToAPIServer(s *ShootContext) {
	GinkgoHelper()

	if gardencorev1beta1.IsIPv6SingleStack(s.Shoot.Spec.Networking.IPFamilies) && len(s.Shoot.Spec.Provider.Workers) > 1 {
		// On local IPv6 single-stack clusters, the in-cluster DNS resolution can fail if it requires cross-node pod-to-pod
		// communication, see https://github.com/gardener/gardener/pull/11287#discussion_r1950320268 and
		// https://github.com/gardener/gardener/pull/11148#issuecomment-2653202171.
		// Until that is fixed, we skip checking in-cluster access for IPv6 single-stack clusters with multiple worker
		// pools.
		return
	}

	Describe("in-cluster access to API server", func() {
		It("should create test objects", func(ctx SpecContext) {
			for _, obj := range getRBACObjects() {
				Eventually(ctx, func() error {
					return s.ShootClient.Create(ctx, obj)
				}).Should(Or(Succeed(), BeAlreadyExistsError()), "should create %T %q", obj, client.ObjectKeyFromObject(obj))
			}
		}, SpecTimeout(time.Minute))

		var (
			pods     []*corev1.Pod
			podNames map[string]string
		)

		It("should create test pods", func(ctx SpecContext) {
			pods = getPods(s.Shoot.Spec.Kubernetes.Version)
			podNames = make(map[string]string, len(pods))

			for _, pod := range pods {
				Eventually(ctx, func(g Gomega) {
					// if pod has already been created, delete it and try again with a new generated name
					if pod.Name != "" {
						g.Expect(s.ShootClient.Delete(ctx, pod)).To(Or(Succeed(), BeNotFoundError()))
						pod.Name = ""
						pod.ResourceVersion = ""
					}

					g.Expect(s.ShootClient.Create(ctx, pod)).To(Succeed())
					podNames[pod.GenerateName] = pod.Name

					if pod.Labels[resourcesv1alpha1.KubernetesServiceHostInject] != "disable" {
						// Verify that gardener successfully injected the KUBERNETES_SERVICE_HOST env var (if not disabled).
						// The webhook has failurePolicy=Ignore, so we might need to delete the pod and try again until the injection
						// succeeds.
						g.Expect(pod.Spec.Containers).To(ConsistOf(
							HaveField("Env", ContainElement(
								HaveField("Name", "KUBERNETES_SERVICE_HOST"),
							)),
						), "gardener should inject the KUBERNETES_SERVICE_HOST env var into the containers of pod %s", pod.Name)
					}
				}).WithPolling(2*time.Second).Should(Succeed(), "should create pod %q", pod.GenerateName)
			}
		}, SpecTimeout(time.Minute))

		It("should wait for test pods to be ready", func(ctx SpecContext) {
			for _, pod := range pods {
				Eventually(ctx, func(g Gomega) {
					g.Expect(s.ShootKomega.Get(pod)()).To(Succeed())
					g.Expect(health.IsPodReady(pod)).To(BeTrue())
				}).Should(Succeed(), "pod %q should get ready", client.ObjectKeyFromObject(pod))
			}
		}, SpecTimeout(time.Minute))

		It("should access the API server via direct path", func(ctx SpecContext) {
			// this pod connects to the API server directly, i.e., uses the KUBERNETES_SERVICE_HOST env var injected by gardener
			expectedAddress := getInternalAPIServerAddress(s.Shoot)
			verifyAccessFromPod(ctx, s.ShootClientSet, podNames[podNameDirect], expectedAddress)
		}, SpecTimeout(time.Minute))

		It("should access the API server via the kubernetes service's clusterIP", func(ctx SpecContext) {
			// this pod connects via the API server proxy using the KUBERNETES_SERVICE_HOST env var injected by kubelet, i.e.,
			// via the clusterIP of kubernetes.default.svc.cluster.local
			expectedAddress := getInClusterAPIServerAddress(ctx, s)
			verifyAccessFromPod(ctx, s.ShootClientSet, podNames[podNameAPIServerProxyIP], expectedAddress)
		}, SpecTimeout(time.Minute))

		It("should access the API server via the kubernetes service's hostname", func(ctx SpecContext) {
			// this pod connects via the API server proxy via the kubernetes.default.svc.cluster.local hostname
			verifyAccessFromPod(ctx, s.ShootClientSet, podNames[podNameAPIServerProxyHostname], "https://kubernetes.default.svc.cluster.local:443")
		}, SpecTimeout(time.Minute))

		AfterAll(func(ctx SpecContext) {
			By("Clean up test objects")
			for _, obj := range getRBACObjects() {
				Eventually(ctx, func() error {
					return s.ShootClient.Delete(ctx, obj)
				}).Should(Or(Succeed(), BeNotFoundError()), "should delete %T %q", obj, client.ObjectKeyFromObject(obj))
			}

			By("Clean up test pods")
			Eventually(ctx, func() error {
				return s.ShootClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(namespace), client.MatchingLabels(labels))
			}).Should(Succeed(), "should delete all test pods")
		}, NodeTimeout(time.Minute))
	})
}

func verifyAccessFromPod(ctx context.Context, clientSet kubernetes.Interface, podName, expectedAddress string) {
	By("Verify we are using the expected path")
	executeKubectl(ctx, clientSet, podName, []string{"/kubectl", "cluster-info"}, Say(
		"Kubernetes control plane is running at %s", regexp.QuoteMeta(expectedAddress),
	))

	By("Verify a typical API request works")
	executeKubectl(ctx, clientSet, podName, []string{"/kubectl", "get", "service", "kubernetes"}, Say(
		`NAME.+\nkubernetes.+\n`,
	))
}

func getInternalAPIServerAddress(shoot *gardencorev1beta1.Shoot) string {
	GinkgoHelper()

	var address string
	for _, a := range shoot.Status.AdvertisedAddresses {
		if a.Name == v1beta1constants.AdvertisedAddressInternal {
			address = a.URL
			break
		}
	}
	Expect(address).NotTo(BeEmpty(), "shoot should have an internal API server address")

	return address + ":443"
}

func getInClusterAPIServerAddress(ctx context.Context, s *ShootContext) string {
	GinkgoHelper()

	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "kubernetes", Namespace: metav1.NamespaceDefault}}
	Eventually(ctx, s.ShootKomega.Get(service)).Should(Succeed())

	clusterIP := service.Spec.ClusterIP
	Expect(clusterIP).NotTo(BeEmpty(), "kubernetes service should have a ClusterIP")

	var port int32
	for _, p := range service.Spec.Ports {
		if p.Name == "https" {
			port = p.Port
			break
		}
	}
	Expect(port).NotTo(BeZero(), "kubernetes service should have a port named https")

	return "https://" + net.JoinHostPort(clusterIP, strconv.FormatInt(int64(port), 10))
}

func executeKubectl(ctx context.Context, clientSet kubernetes.Interface, podName string, command []string, matcher gomegatypes.GomegaMatcher) {
	GinkgoHelper()

	// Retry the command execution with a short timeout to reduce flakiness. We better timeout quickly and succeed on the
	// next try than being stuck in on try.
	Eventually(ctx, func(g Gomega) {
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()

		stdOutBuffer := NewBuffer()

		g.Expect(clientSet.PodExecutor().ExecuteWithStreams(
			timeoutCtx,
			namespace,
			podName,
			containerName,
			nil,
			// forward both stdout and stderr to the ginkgo output to ensure test failures can be debugged
			io.MultiWriter(stdOutBuffer, gexec.NewPrefixedWriter("[out] ", GinkgoWriter)),
			gexec.NewPrefixedWriter("[err] ", GinkgoWriter),
			command...,
		)).To(Succeed())

		// we don't need Eventually here, because the buffer is already closed
		g.Expect(stdOutBuffer).To(matcher)
	}).Should(Succeed())
}

func getPods(kubernetesVersion string) []*corev1.Pod {
	var pods []*corev1.Pod

	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameHyperkube, imagevectorutils.TargetVersion(kubernetesVersion))
	Expect(err).NotTo(HaveOccurred())

	podDirect := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: podNameDirect,
			Namespace:    namespace,
			Labels:       maps.Clone(labels),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  containerName,
				Image: image.String(),
				Env: []corev1.EnvVar{{
					// disable color output of `kubectl cluster-info` to make it simpler to assert
					// ref https://github.com/daviddengcn/go-colortext/blob/v1.0.0/ct_ansi.go#L12
					Name:  "TERM",
					Value: "dumb",
				}},
				// allow running this pod on the "unprivileged" shoot
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
					RunAsUser:    ptr.To[int64](65532),
					RunAsGroup:   ptr.To[int64](65532),
					RunAsNonRoot: ptr.To(true),
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			}},
			ServiceAccountName: name,
		},
	}
	pods = append(pods, podDirect)

	podAPIServerProxyIP := podDirect.DeepCopy()
	podAPIServerProxyIP.GenerateName = podNameAPIServerProxyIP
	// disable gardener's injection of the KUBERNETES_SERVICE_HOST env var
	podAPIServerProxyIP.Labels[resourcesv1alpha1.KubernetesServiceHostInject] = "disable"
	pods = append(pods, podAPIServerProxyIP)

	podAPIServerProxyHostname := podDirect.DeepCopy()
	podAPIServerProxyHostname.GenerateName = podNameAPIServerProxyHostname
	// manually set the KUBERNETES_SERVICE_HOST env var, gardener does not overwrite it if present
	podAPIServerProxyHostname.Spec.Containers[0].Env = append(podAPIServerProxyHostname.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  "KUBERNETES_SERVICE_HOST",
		Value: "kubernetes.default.svc.cluster.local",
	})
	pods = append(pods, podAPIServerProxyHostname)

	return pods
}

func getRBACObjects() []client.Object {
	var objects []client.Object

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    maps.Clone(labels),
		},
	}
	objects = append(objects, serviceAccount)

	// permissions used by the test command: kubectl get service kubernetes
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    maps.Clone(labels),
		},
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{""},
			Resources: []string{"services"},
			Verbs:     []string{"get"},
		}},
	}
	objects = append(objects, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    maps.Clone(labels),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     name,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      name,
			Namespace: namespace,
		}},
	}
	objects = append(objects, roleBinding)

	// permissions used by the test command: kubectl cluster-info
	roleSystem := role.DeepCopy()
	roleSystem.Namespace = metav1.NamespaceSystem
	roleSystem.Rules = []rbacv1.PolicyRule{{
		APIGroups: []string{""},
		Resources: []string{"services"},
		Verbs:     []string{"list"},
	}}
	objects = append(objects, roleSystem)

	roleBindingSystem := roleBinding.DeepCopy()
	roleBindingSystem.Namespace = metav1.NamespaceSystem
	objects = append(objects, roleBindingSystem)

	return objects
}
