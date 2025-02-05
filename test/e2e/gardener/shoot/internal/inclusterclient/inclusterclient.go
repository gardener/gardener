// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package inclusterclient

import (
	"context"
	"io"
	"maps"
	"net"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
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
// - direct path using the KUBERNETES_SERVICE_HOST env var injected by gardener
// - via the API server proxy's clusterIP
// - via the API server proxy's hostname kubernetes.default.svc.cluster.local
// For this, it deploys pods with the hyperkube image that contains the kubectl binary serving as a test client.
// See docs/usage/networking/shoot_kubernetes_service_host_injection.md and docs/proposals/08-shoot-apiserver-via-sni.md
func VerifyInClusterAccessToAPIServer(parentCtx context.Context, f *framework.ShootFramework) {
	ctx, cancel := context.WithTimeout(parentCtx, 2*time.Minute)
	defer cancel()

	defer prepareObjects(ctx, f.ShootClient.Client(), f.Shoot.Spec.Kubernetes.Version)()

	By("Verify in-cluster access to API server via direct path")
	// this pod connects to the API server directly, i.e., uses the KUBERNETES_SERVICE_HOST env var injected by gardener
	verifyAccessFromPod(ctx, f, podNameDirect, getInternalAPIServerAddress(f.Shoot))

	By("Verify in-cluster access to API server via API server proxy IP")
	// this pod connects via the API server proxy using the KUBERNETES_SERVICE_HOST env var injected by kubelet, i.e.,
	// via the clusterIP of kubernetes.default.svc.cluster.local
	verifyAccessFromPod(ctx, f, podNameAPIServerProxyIP, getInClusterAPIServerAddress(ctx, f.ShootClient.Client()))

	By("Verify in-cluster access to API server via API server proxy hostname")
	// this pod connects via the API server proxy hostname, i.e., via kubernetes.default.svc.cluster.local
	verifyAccessFromPod(ctx, f, podNameAPIServerProxyHostname, "https://kubernetes.default.svc.cluster.local:443")
}

func verifyAccessFromPod(ctx context.Context, f *framework.ShootFramework, podName, expectedAddress string) {
	By("Verify we are using the expected path")
	Eventually(
		execute(ctx, f.ShootClient, podName, "/kubectl", "cluster-info"),
	).Should(Say(
		"Kubernetes control plane is running at %s", expectedAddress,
	))

	By("Verify a typical API request works")
	Eventually(
		execute(ctx, f.ShootClient, podName, "/kubectl", "get", "service", "kubernetes"),
	).Should(Say(
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
		}).Should(Succeed(), "should create %T %q", obj, client.ObjectKeyFromObject(obj))
	}

	By("Wait for test pods to be ready")
	for _, obj := range objects {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			continue
		}

		Eventually(func(g Gomega) {
			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			g.Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
		}).WithContext(ctx).WithTimeout(time.Minute).Should(Succeed(), "%T %q should get ready", obj, client.ObjectKeyFromObject(obj))
	}

	return func() {
		By("Cleaning up test objects for verifying in-cluster access to API server")
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		for _, obj := range objects {
			Eventually(func() error {
				return client.IgnoreNotFound(c.Delete(cleanupCtx, obj))
			}).Should(Succeed(), "should delete %T %q", obj, client.ObjectKeyFromObject(obj))
		}
	}
}

func execute(ctx context.Context, clientSet kubernetes.Interface, podName string, command ...string) *Buffer {
	GinkgoHelper()
	var stdOutBuffer *Buffer

	// Retry the command execution to reduce flakiness.
	// Initialize a fresh output buffer on every try for asserting only the results of the last (hopefully successful)
	// command execution.
	// Both stdOut and stdErr will be forwarded to the ginkgo output on every try to ensure test failures can be debugged.
	Eventually(func() error {
		stdOutBuffer = NewBuffer()

		return clientSet.PodExecutor().ExecuteWithStreams(
			ctx,
			namespace,
			podName,
			containerName,
			nil,
			io.MultiWriter(stdOutBuffer, gexec.NewPrefixedWriter("[out] ", GinkgoWriter)),
			gexec.NewPrefixedWriter("[err] ", GinkgoWriter),
			command...,
		)
	}).Should(Succeed())

	return stdOutBuffer
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
