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
	"github.com/gardener/gardener/test/framework"
)

const (
	name      = "e2e-test-in-cluster-client"
	namespace = metav1.NamespaceDefault

	podNameDirect                 = name + "-direct"
	podNameAPIServerProxyIP       = name + "-apiserver-proxy-ip"
	podNameAPIServerProxyHostname = name + "-apiserver-proxy-hostname"
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
func VerifyInClusterAccessToAPIServer(parentCtx context.Context, f *framework.ShootFramework) {
	if gardencorev1beta1.IsIPv6SingleStack(f.Shoot.Spec.Networking.IPFamilies) && len(f.Shoot.Spec.Provider.Workers) > 1 {
		// On local IPv6 single-stack clusters, the in-cluster DNS resolution can fail if it requires cross-node pod-to-pod
		// communication, see https://github.com/gardener/gardener/pull/11287#discussion_r1950320268 and
		// https://github.com/gardener/gardener/pull/11148#issuecomment-2653202171.
		// Until that is fixed, we skip checking in-cluster access for IPv6 single-stack clusters with multiple worker
		// pools.
		return
	}

	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Minute)
	defer cancel()

	defer prepareObjects(ctx, f.ShootClient.Client(), f.Shoot.Spec.Kubernetes.Version)()

	By("Verify access via direct path")
	// this pod connects to the API server directly, i.e., uses the KUBERNETES_SERVICE_HOST env var injected by gardener
	verifyAccessFromPod(ctx, f, podNameDirect, getInternalAPIServerAddress(f.Shoot))

	By("Verify access via API server proxy via the kubernetes service's clusterIP")
	// this pod connects via the API server proxy using the KUBERNETES_SERVICE_HOST env var injected by kubelet, i.e.,
	// via the clusterIP of kubernetes.default.svc.cluster.local
	verifyAccessFromPod(ctx, f, podNameAPIServerProxyIP, getInClusterAPIServerAddress(ctx, f.ShootClient.Client()))

	By("Verify access via API server proxy via the kubernetes service's hostname")
	// this pod connects via the API server proxy via the kubernetes.default.svc.cluster.local hostname
	verifyAccessFromPod(ctx, f, podNameAPIServerProxyHostname, "https://kubernetes.default.svc.cluster.local:443")
}

func verifyAccessFromPod(ctx context.Context, f *framework.ShootFramework, podName, expectedAddress string) {
	By("Verify we are using the expected path")
	executeKubectl(ctx, f.ShootClient, podName, []string{"/kubectl", "cluster-info"}, Say(
		"Kubernetes control plane is running at %s", regexp.QuoteMeta(expectedAddress),
	))

	By("Verify a typical API request works")
	executeKubectl(ctx, f.ShootClient, podName, []string{"/kubectl", "get", "service", "kubernetes"}, Say(
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
	Expect(address).NotTo(BeEmpty())

	return address + ":443"
}

func getInClusterAPIServerAddress(ctx context.Context, c client.Client) string {
	GinkgoHelper()

	service := corev1.Service{}
	Expect(c.Get(ctx, client.ObjectKey{Name: "kubernetes", Namespace: metav1.NamespaceDefault}, &service)).To(Succeed())

	clusterIP := service.Spec.ClusterIP
	Expect(clusterIP).NotTo(BeEmpty())

	var port int32
	for _, p := range service.Spec.Ports {
		if p.Name == "https" {
			port = p.Port
			break
		}
	}
	Expect(port).NotTo(BeZero())

	return "https://" + net.JoinHostPort(clusterIP, strconv.FormatInt(int64(port), 10))
}

func prepareObjects(ctx context.Context, c client.Client, kubernetesVersion string) func() {
	objects := getObjects(kubernetesVersion)

	By("Create test objects for verifying in-cluster access to API server")
	for _, obj := range objects {
		Eventually(func() error {
			return client.IgnoreAlreadyExists(c.Create(ctx, obj))
		}).WithContext(ctx).Should(Succeed(), "should create %T %q", obj, client.ObjectKeyFromObject(obj))
	}

	By("Wait for test pods to be ready")
	Eventually(func(g Gomega) {
		for _, obj := range objects {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				continue
			}

			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			g.Expect(health.IsPodReady(pod)).To(BeTrue(), "%T %q should get ready", obj, client.ObjectKeyFromObject(obj))
		}
	}).WithContext(ctx).WithTimeout(time.Minute).Should(Succeed())

	return func() {
		By("Cleaning up test objects for verifying in-cluster access to API server")
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		for _, obj := range objects {
			Eventually(func() error {
				return client.IgnoreNotFound(c.Delete(cleanupCtx, obj))
			}).WithContext(cleanupCtx).Should(Succeed(), "should delete %T %q", obj, client.ObjectKeyFromObject(obj))
		}
	}
}

func executeKubectl(ctx context.Context, clientSet kubernetes.Interface, podName string, command []string, matcher gomegatypes.GomegaMatcher) {
	GinkgoHelper()

	// Retry the command execution with a short timeout to reduce flakiness. We better timeout quickly and succeed on the
	// next try than being stuck in on try.
	Eventually(func(g Gomega, ctx context.Context) {
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
	}).WithContext(ctx).WithTimeout(5 * time.Minute).Should(Succeed())
}

func getObjects(kubernetesVersion string) []client.Object {
	objects := getRBACObjects()

	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameHyperkube, imagevectorutils.TargetVersion(kubernetesVersion))
	Expect(err).NotTo(HaveOccurred())

	podDirect := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podNameDirect,
			Namespace: namespace,
			Labels:    maps.Clone(labels),
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
	objects = append(objects, podDirect)

	podAPIServerProxyIP := podDirect.DeepCopy()
	podAPIServerProxyIP.Name = podNameAPIServerProxyIP
	// disable gardener's injection of the KUBERNETES_SERVICE_HOST env var
	podAPIServerProxyIP.Labels[resourcesv1alpha1.KubernetesServiceHostInject] = "disable"
	objects = append(objects, podAPIServerProxyIP)

	podAPIServerProxyHostname := podDirect.DeepCopy()
	podAPIServerProxyHostname.Name = podNameAPIServerProxyHostname
	// manually set the KUBERNETES_SERVICE_HOST env var, gardener does not overwrite it if present
	podAPIServerProxyHostname.Spec.Containers[0].Env = append(podAPIServerProxyHostname.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  "KUBERNETES_SERVICE_HOST",
		Value: "kubernetes.default.svc.cluster.local",
	})
	objects = append(objects, podAPIServerProxyHostname)

	return objects
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
