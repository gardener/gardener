// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/pkg/api/indexer"
	fakecontainerdclient "github.com/gardener/gardener/pkg/nodeagent/containerd/fake"
	"github.com/gardener/gardener/pkg/nodeagent/controller/healthcheck"
	fakedbus "github.com/gardener/gardener/pkg/nodeagent/dbus/fake"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Healthcheck controller tests", func() {
	var (
		mgr                manager.Manager
		clock              *testing.FakeClock
		nodeName           string
		node               *corev1.Node
		fakeDBus           *fakedbus.DBus
		interfaceAddresses []string
		containerdClient   *fakecontainerdclient.Client
		kubeletHealthcheck *healthcheck.KubeletHealthChecker
		healthCheckers     []healthcheck.HealthChecker
		isControlPlaneNode bool
		ts                 *httptest.Server
	)

	BeforeEach(func() {
		testRunID = "test-" + gardenerutils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

		By("Setup manager")
		var err error
		mgr, err = manager.New(restConfig, manager.Options{
			Metrics: metricsserver.Options{BindAddress: "0"},
			Cache: cache.Options{
				DefaultLabelSelector: labels.SelectorFromSet(labels.Set{testID: testRunID}),
			},
			Controller: config.Controller{
				SkipNameValidation: new(true),
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(indexer.AddPodNodeName(ctx, mgr.GetFieldIndexer())).To(Succeed())

		clock = testing.NewFakeClock(time.Now())
		fakeDBus = fakedbus.New()
		getAddresses := func() ([]net.Addr, error) {
			var result []net.Addr
			for _, addr := range interfaceAddresses {
				_, ip, err := net.ParseCIDR(addr)
				Expect(err).NotTo(HaveOccurred())
				result = append(result, ip)
			}
			return result, nil
		}
		containerdClient = fakecontainerdclient.NewClient()

		nodeName = testRunID

		kubeletHealthcheck = healthcheck.NewKubeletHealthChecker(
			mgr.GetClient(), clock, fakeDBus, mgr.GetEventRecorder(healthcheck.ControllerName), getAddresses,
		)

		containerdHealthcheck := healthcheck.NewContainerdHealthChecker(
			mgr.GetClient(), containerdClient, clock, fakeDBus, mgr.GetEventRecorder(healthcheck.ControllerName),
		)

		healthCheckers = []healthcheck.HealthChecker{containerdHealthcheck, kubeletHealthcheck}
		isControlPlaneNode = false
	})

	JustBeforeEach(func() {
		By("Register controller")
		Expect((&healthcheck.Reconciler{
			HealthCheckIntervalSeconds: 1,
			HealthCheckers:             healthCheckers,
		}).AddToManager(mgr, predicate.NewPredicateFuncs(func(obj client.Object) bool { return obj.GetName() == nodeName }), isControlPlaneNode)).To(Succeed())

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

		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   nodeName,
				Labels: map[string]string{testID: testRunID},
			},
		}

		By("Create Node")
		Expect(testClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(func() {
			By("Delete Node")
			Expect(testClient.Delete(ctx, node)).To(Succeed())
			By("Cleanup fakeDBUS")
		})

		ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			n, err := fmt.Fprintln(w, "OK")
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(BeNumerically(">", 0))
		}))

		DeferCleanup(func() {
			ts.Close()
		})
	})

	It("Containerd health should be true", func() {
		By("Start fake kubelet healthz endpoint")
		kubeletHealthcheck.SetKubeletHealthEndpoint(ts.URL)

		Consistently(func() []fakedbus.SystemdAction {
			return fakeDBus.Actions
		}).Should(BeEmpty())
	})

	It("Containerd health should be false", func() {
		By("Start fake kubelet healthz endpoint")
		kubeletHealthcheck.SetKubeletHealthEndpoint(ts.URL)
		containerdClient.SetReturnError(true)
		clock.Step(80 * time.Second)
		Eventually(func() []fakedbus.SystemdAction {
			return fakeDBus.Actions
		}).Should(
			ConsistOf(fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{"containerd.service"}}),
		)
	})

	It("Kubelet health should be false", func() {
		clock.Step(80 * time.Second)
		Eventually(func() []fakedbus.SystemdAction {
			return fakeDBus.Actions
		}).Should(
			ConsistOf(fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{"kubelet.service"}}),
		)
	})

	It("Kubelet health should be true", func() {
		kubeletHealthcheck.SetKubeletHealthEndpoint(ts.URL)
		clock.Step(80 * time.Second)
		Eventually(func() []fakedbus.SystemdAction {
			return fakeDBus.Actions
		}).Should(BeEmpty())
	})

	It("Node InternalIP went away and came back", func() {
		kubeletHealthcheck.SetKubeletHealthEndpoint(ts.URL)

		interfaceAddresses = []string{"1.2.3.4/32"}
		By("Patch Node Status add NodeAddress")
		node.Status.Addresses = []corev1.NodeAddress{
			{
				Type:    corev1.NodeInternalIP,
				Address: "1.2.3.4",
			},
		}
		Expect(testClient.Status().Update(ctx, node)).To(Succeed())

		Eventually(func() []corev1.NodeAddress {
			Expect(testClient.Get(ctx, types.NamespacedName{Name: nodeName}, node)).To(Succeed())
			return node.Status.Addresses
		}).Should(ConsistOf(corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "1.2.3.4"}))

		Eventually(func() bool {
			return kubeletHealthcheck.HasLastInternalIP()
		}).Should(BeTrue())

		By("Update Node Status, remove NodeAddresses")
		node.Status.Addresses = []corev1.NodeAddress{}
		Expect(testClient.Status().Update(ctx, node)).To(Succeed())

		By("Wait for reappearing NodeAddress")
		Eventually(func() []corev1.NodeAddress {
			Expect(testClient.Get(ctx, types.NamespacedName{Name: nodeName}, node)).To(Succeed())
			return node.Status.Addresses
		}).Should(ConsistOf(corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "1.2.3.4"}))
	})

	When("the node is a control plane node and runs critical static pods", func() {
		var pod *corev1.Pod

		criticalStaticPodCondition := func(g Gomega) *corev1.NodeCondition {
			g.Expect(testClient.Get(ctx, types.NamespacedName{Name: nodeName}, node)).To(Succeed())
			for i, condition := range node.Status.Conditions {
				if condition.Type == "CriticalControlPlanePodsUnhealthy" {
					return &node.Status.Conditions[i]
				}
			}
			return nil
		}

		setPodReadiness := func(status corev1.ConditionStatus, phase corev1.PodPhase) {
			pod.Status = corev1.PodStatus{
				Phase:      phase,
				Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: status}},
			}
			Expect(testClient.Status().Update(ctx, pod)).To(Succeed())
		}

		BeforeEach(func() {
			isControlPlaneNode = true
			healthCheckers = append(healthCheckers, healthcheck.NewCriticalStaticPodsChecker(mgr.GetClient(), clock))
			kubeletHealthcheck.SetKubeletHealthEndpoint(ts.URL)

			// The pod name carries the unique testRunID because envtest has no kubelet to finalize pod
			// deletion, so a pod with a fixed name would linger in terminating state and collide with the
			// next spec's creation.
			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "etcd-main-" + testRunID,
					Namespace: metav1.NamespaceSystem,
					Labels: map[string]string{
						testID: testRunID,
						"shoot.gardener.cloud/is-critical-static-pod": "true",
					},
				},
				Spec: corev1.PodSpec{
					NodeName:   nodeName,
					Containers: []corev1.Container{{Name: "etcd", Image: "etcd"}},
				},
			}
		})

		It("should report the condition healthy when no critical static pods exist", func() {
			Eventually(criticalStaticPodCondition).Should(PointTo(MatchFields(IgnoreExtras, Fields{
				"Status": Equal(corev1.ConditionFalse),
				"Reason": Equal("AllCriticalControlPlanePodsHealthy"),
			})))
		})

		It("should report the condition healthy when the critical static pod is running and ready", func() {
			By("Create critical static pod")
			Expect(testClient.Create(ctx, pod)).To(Succeed())
			DeferCleanup(func() { Expect(testClient.Delete(ctx, pod)).To(Succeed()) })
			setPodReadiness(corev1.ConditionTrue, corev1.PodRunning)

			Eventually(criticalStaticPodCondition).Should(PointTo(MatchFields(IgnoreExtras, Fields{
				"Status": Equal(corev1.ConditionFalse),
				"Reason": Equal("AllCriticalControlPlanePodsHealthy"),
			})))
		})

		It("should report the condition unhealthy when the critical static pod is not ready", func() {
			By("Create critical static pod")
			Expect(testClient.Create(ctx, pod)).To(Succeed())
			DeferCleanup(func() { Expect(testClient.Delete(ctx, pod)).To(Succeed()) })
			setPodReadiness(corev1.ConditionFalse, corev1.PodRunning)

			Eventually(criticalStaticPodCondition).Should(PointTo(MatchFields(IgnoreExtras, Fields{
				"Status": Equal(corev1.ConditionTrue),
				"Reason": Equal("SomeCriticalControlPlanePodsUnhealthy"),
			})))
		})

		It("should recover the condition when the critical static pod becomes ready again", func() {
			By("Create unhealthy critical static pod")
			Expect(testClient.Create(ctx, pod)).To(Succeed())
			DeferCleanup(func() { Expect(testClient.Delete(ctx, pod)).To(Succeed()) })
			setPodReadiness(corev1.ConditionFalse, corev1.PodRunning)

			By("Wait for unhealthy condition")
			Eventually(func(g Gomega) corev1.ConditionStatus {
				if condition := criticalStaticPodCondition(g); condition != nil {
					return condition.Status
				}
				return corev1.ConditionUnknown
			}).Should(Equal(corev1.ConditionTrue))

			By("Make critical static pod ready")
			setPodReadiness(corev1.ConditionTrue, corev1.PodRunning)

			By("Wait for healthy condition")
			Eventually(func(g Gomega) corev1.ConditionStatus {
				if condition := criticalStaticPodCondition(g); condition != nil {
					return condition.Status
				}
				return corev1.ConditionUnknown
			}).Should(Equal(corev1.ConditionFalse))
		})

		It("should ignore critical static pods running on other nodes", func() {
			By("Create unhealthy critical static pod on another node")
			pod.Spec.NodeName = "other-node"
			Expect(testClient.Create(ctx, pod)).To(Succeed())
			DeferCleanup(func() { Expect(testClient.Delete(ctx, pod)).To(Succeed()) })
			setPodReadiness(corev1.ConditionFalse, corev1.PodRunning)

			Consistently(func(g Gomega) corev1.ConditionStatus {
				if condition := criticalStaticPodCondition(g); condition != nil {
					return condition.Status
				}
				return corev1.ConditionUnknown
			}).ShouldNot(Equal(corev1.ConditionTrue))
		})
	})
})
