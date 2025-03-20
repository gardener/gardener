// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/containerd/containerd"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/pkg/nodeagent/controller/healthcheck"
	fakedbus "github.com/gardener/gardener/pkg/nodeagent/dbus/fake"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Healthcheck controller tests", func() {
	var (
		clock              *testing.FakeClock
		nodeName           string
		node               *corev1.Node
		fakeDBus           *fakedbus.DBus
		interfaceAddresses []string
		containerdClient   *fakeContainerdClient
		kubeletHealthcheck *healthcheck.KubeletHealthChecker
		ts                 *httptest.Server
	)

	BeforeEach(func() {
		testRunID = "test-" + gardenerutils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

		By("Setup manager")
		mgr, err := manager.New(restConfig, manager.Options{
			Metrics: metricsserver.Options{BindAddress: "0"},
			Cache: cache.Options{
				DefaultLabelSelector: labels.SelectorFromSet(labels.Set{testID: testRunID}),
			},
			Controller: config.Controller{
				SkipNameValidation: ptr.To(true),
			},
		})
		Expect(err).NotTo(HaveOccurred())

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
		containerdClient = &fakeContainerdClient{
			returnError: false,
		}
		nodeName = testRunID

		kubeletHealthcheck = healthcheck.NewKubeletHealthChecker(
			mgr.GetClient(), clock, fakeDBus, mgr.GetEventRecorderFor(healthcheck.ControllerName), getAddresses,
		)

		containerdHealthcheck := healthcheck.NewContainerdHealthChecker(
			mgr.GetClient(), containerdClient, clock, fakeDBus, mgr.GetEventRecorderFor(healthcheck.ControllerName),
		)

		By("Register controller")
		Expect((&healthcheck.Reconciler{
			HealthCheckIntervalSeconds: 1,
			HealthCheckers:             []healthcheck.HealthChecker{containerdHealthcheck, kubeletHealthcheck},
		}).AddToManager(mgr, predicate.NewPredicateFuncs(func(obj client.Object) bool { return obj.GetName() == nodeName }))).To(Succeed())

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
		containerdClient.returnError = true
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

	It("Kubelet toggles between Ready and NotReady to fast and triggers a reboot", func() {
		By("Start fake kubelet healthz endpoint")
		kubeletHealthcheck.SetKubeletHealthEndpoint(ts.URL)

		By("Patch Node Status add NodeAddress")
		node.Status.Addresses = []corev1.NodeAddress{
			{
				Type:    corev1.NodeInternalIP,
				Address: "1.2.3.4",
			},
		}
		Expect(testClient.Status().Update(ctx, node)).To(Succeed())
		Eventually(func() bool {
			return kubeletHealthcheck.HasLastInternalIP()
		}).Should(BeTrue())

		for i := 1; i <= 4; i++ {
			clock.Step(2 * time.Second)
			By("Patch Node Status to Ready")
			setNodeCondition(ctx, node, corev1.ConditionTrue)
			Eventually(func() bool {
				return kubeletHealthcheck.NodeReady
			}).Should(BeTrue())
			Eventually(func() int {
				return len(kubeletHealthcheck.KubeletReadinessToggles)
			}).Should(Equal(i))

			clock.Step(2 * time.Second)
			By("Patch Node Status to NotReady")
			setNodeCondition(ctx, node, corev1.ConditionFalse)
			Eventually(func() bool {
				return kubeletHealthcheck.NodeReady
			}).Should(BeFalse())
			Eventually(func() int {
				return len(kubeletHealthcheck.KubeletReadinessToggles)
			}).Should(Equal(i))
		}

		clock.Step(2 * time.Second)
		By("Patch Node Status to Ready")
		setNodeCondition(ctx, node, corev1.ConditionTrue)
		Eventually(func() bool {
			return kubeletHealthcheck.NodeReady
		}).Should(BeTrue())
		Eventually(func() int {
			return len(kubeletHealthcheck.KubeletReadinessToggles)
		}).Should(Equal(5))

		clock.Step(80 * time.Second)
		Eventually(func() []fakedbus.SystemdAction {
			return fakeDBus.Actions
		}).Should(
			ConsistOf(fakedbus.SystemdAction{Action: fakedbus.ActionReboot, UnitNames: []string{"reboot"}}),
		)
	})
})

type fakeContainerdClient struct {
	returnError bool
}

func (f *fakeContainerdClient) Version(_ context.Context) (containerd.Version, error) {
	if f.returnError {
		return containerd.Version{}, errors.New("calling fake containerd socket error")
	}
	return containerd.Version{Version: "fake version"}, nil
}

func setNodeCondition(ctx context.Context, node *corev1.Node, condition corev1.ConditionStatus) {
	node.Status.Conditions = []corev1.NodeCondition{
		{
			Type:   corev1.NodeReady,
			Status: condition,
		},
	}
	Expect(testClient.Status().Update(ctx, node)).To(Succeed())
}
