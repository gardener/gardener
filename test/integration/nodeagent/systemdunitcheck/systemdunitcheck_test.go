// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package systemdunitcheck_test

import (
	"time"

	systemddbus "github.com/coreos/go-systemd/v22/dbus"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("SystemdUnitCheck controller tests", func() {
	var node *corev1.Node

	BeforeEach(func() {
		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   nodeName,
				Labels: map[string]string{testID: testID},
			},
		}

		By("Create Node")
		Expect(testClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(func() {
			By("Delete Node")
			Expect(testClient.Delete(ctx, node)).To(Succeed())
		})
	})

	AfterEach(func() {
		fakeDBus.SetUnits()
		Expect(fakeFS.Remove(nodeagentconfigv1alpha1.LastAppliedOperatingSystemConfigFilePath)).To(Or(Succeed(), MatchError(afero.ErrFileNotFound)))
	})

	writeOSC := func(osc *extensionsv1alpha1.OperatingSystemConfig) {
		data, err := runtime.Encode(oscCodec, osc)
		Expect(err).NotTo(HaveOccurred())
		Expect(fakeFS.WriteFile(nodeagentconfigv1alpha1.LastAppliedOperatingSystemConfigFilePath, data, 0600)).To(Succeed())
	}

	getNodeCondition := func(g Gomega) *corev1.NodeCondition {
		g.Expect(testClient.Get(ctx, types.NamespacedName{Name: nodeName}, node)).To(Succeed())
		for _, condition := range node.Status.Conditions {
			if condition.Type == nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady {
				return &condition
			}
		}
		return nil
	}

	It("should not set condition when no OSC file exists", func() {
		Consistently(func(g Gomega) *corev1.NodeCondition {
			return getNodeCondition(g)
		}).Should(BeNil())
	})

	It("should report all units healthy when all are active", func() {
		writeOSC(&extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Units: []extensionsv1alpha1.Unit{
					{Name: "kubelet.service", Enable: ptr.To(true)},
					{Name: "containerd.service", Enable: ptr.To(true)},
				},
			},
		})

		fakeDBus.AddUnitsToList(
			systemddbus.UnitStatus{Name: "kubelet.service", ActiveState: "active"},
			systemddbus.UnitStatus{Name: "containerd.service", ActiveState: "active"},
		)

		Eventually(func(g Gomega) *corev1.NodeCondition {
			return getNodeCondition(g)
		}).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady),
			"Status": Equal(corev1.ConditionTrue),
			"Reason": Equal("AllUnitsHealthy"),
		})))
	})

	It("should report unhealthy when a unit has failed", func() {
		writeOSC(&extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Units: []extensionsv1alpha1.Unit{
					{Name: "kubelet.service", Enable: ptr.To(true)},
					{Name: "bad.service", Enable: ptr.To(true)},
				},
			},
		})

		fakeDBus.AddUnitsToList(
			systemddbus.UnitStatus{Name: "kubelet.service", ActiveState: "active"},
			systemddbus.UnitStatus{Name: "bad.service", ActiveState: "failed"},
		)

		Eventually(func(g Gomega) *corev1.NodeCondition {
			return getNodeCondition(g)
		}).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":    Equal(nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady),
			"Status":  Equal(corev1.ConditionFalse),
			"Reason":  Equal("UnhealthyUnits"),
			"Message": ContainSubstring("bad.service: failed"),
		})))
	})

	It("should report unhealthy when a unit is stuck in activating beyond threshold", func() {
		writeOSC(&extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Units: []extensionsv1alpha1.Unit{
					{Name: "stuck.service", Enable: ptr.To(true)},
				},
			},
		})

		fakeDBus.AddUnitsToList(
			systemddbus.UnitStatus{Name: "stuck.service", ActiveState: "activating"},
		)
		fakeDBus.SetUnitStateChangeTimestamp("stuck.service", fakeClock.Now().Add(-10*time.Minute))

		Eventually(func(g Gomega) *corev1.NodeCondition {
			return getNodeCondition(g)
		}).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":    Equal(nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady),
			"Status":  Equal(corev1.ConditionFalse),
			"Reason":  Equal("UnhealthyUnits"),
			"Message": ContainSubstring("stuck.service: stuck in activating"),
		})))
	})

	It("should report progressing when a unit is activating within threshold", func() {
		writeOSC(&extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Units: []extensionsv1alpha1.Unit{
					{Name: "starting.service", Enable: ptr.To(true)},
				},
			},
		})

		fakeDBus.AddUnitsToList(
			systemddbus.UnitStatus{Name: "starting.service", ActiveState: "activating"},
		)
		fakeDBus.SetUnitStateChangeTimestamp("starting.service", fakeClock.Now().Add(-30*time.Second))

		Eventually(func(g Gomega) *corev1.NodeCondition {
			return getNodeCondition(g)
		}).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady),
			"Status": Equal(corev1.ConditionTrue),
			"Reason": Equal("ProgressingUnits"),
		})))
	})

	It("should not report progressing for a service in auto-restart between runs", func() {
		writeOSC(&extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Units: []extensionsv1alpha1.Unit{
					{Name: "sshd-ensurer.service", Enable: ptr.To(true)},
				},
			},
		})

		fakeDBus.AddUnitsToList(
			systemddbus.UnitStatus{Name: "sshd-ensurer.service", ActiveState: "activating", SubState: "auto-restart"},
		)

		Eventually(func(g Gomega) *corev1.NodeCondition {
			return getNodeCondition(g)
		}).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady),
			"Status": Equal(corev1.ConditionTrue),
			"Reason": Equal("AllUnitsHealthy"),
		})))
	})

	It("should report unhealthy when an enabled non-oneshot unit is inactive", func() {
		writeOSC(&extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Units: []extensionsv1alpha1.Unit{
					{Name: "stopped.service", Enable: ptr.To(true)},
				},
			},
		})

		fakeDBus.AddUnitsToList(
			systemddbus.UnitStatus{Name: "stopped.service", ActiveState: "inactive", SubState: "dead"},
		)

		Eventually(func(g Gomega) *corev1.NodeCondition {
			return getNodeCondition(g)
		}).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":    Equal(nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady),
			"Status":  Equal(corev1.ConditionFalse),
			"Reason":  Equal("UnhealthyUnits"),
			"Message": ContainSubstring("stopped.service: inactive but should be enabled"),
		})))
	})

	It("should not report unhealthy for an inactive triggered service that exited successfully", func() {
		writeOSC(&extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Units: []extensionsv1alpha1.Unit{
					{Name: "gardener-user.service", Enable: ptr.To(true)},
					{Name: "kubelet.service", Enable: ptr.To(true)},
				},
			},
		})

		fakeDBus.AddUnitsToList(
			systemddbus.UnitStatus{Name: "gardener-user.service", ActiveState: "inactive", SubState: "dead"},
			systemddbus.UnitStatus{Name: "kubelet.service", ActiveState: "active"},
		)
		fakeDBus.SetTriggeredBy("gardener-user.service", []string{"gardener-user.path"})

		Eventually(func(g Gomega) *corev1.NodeCondition {
			return getNodeCondition(g)
		}).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady),
			"Status": Equal(corev1.ConditionTrue),
			"Reason": Equal("AllUnitsHealthy"),
		})))
	})

	It("should not report unhealthy for an inactive oneshot service that exited successfully", func() {
		writeOSC(&extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Units: []extensionsv1alpha1.Unit{
					{Name: "updatecacerts.service", Enable: ptr.To(true)},
				},
			},
		})

		fakeDBus.AddUnitsToList(
			systemddbus.UnitStatus{Name: "updatecacerts.service", ActiveState: "inactive", SubState: "dead"},
		)
		fakeDBus.SetServiceType("updatecacerts.service", "oneshot")

		Eventually(func(g Gomega) *corev1.NodeCondition {
			return getNodeCondition(g)
		}).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady),
			"Status": Equal(corev1.ConditionTrue),
			"Reason": Equal("AllUnitsHealthy"),
		})))
	})

	It("should not report unhealthy for an inactive unit that is disabled", func() {
		writeOSC(&extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Units: []extensionsv1alpha1.Unit{
					{Name: "optional.service", Enable: ptr.To(false)},
				},
			},
		})

		fakeDBus.AddUnitsToList(
			systemddbus.UnitStatus{Name: "optional.service", ActiveState: "inactive"},
		)

		Eventually(func(g Gomega) *corev1.NodeCondition {
			return getNodeCondition(g)
		}).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady),
			"Status": Equal(corev1.ConditionTrue),
			"Reason": Equal("AllUnitsHealthy"),
		})))
	})

	It("should also monitor gardener-node-agent's own units", func() {
		writeOSC(&extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Units: []extensionsv1alpha1.Unit{
					{Name: nodeagentconfigv1alpha1.UnitName, Enable: ptr.To(true)},
					{Name: nodeagentconfigv1alpha1.InitUnitName, Enable: ptr.To(true)},
					{Name: "kubelet.service", Enable: ptr.To(true)},
				},
			},
		})

		fakeDBus.AddUnitsToList(
			systemddbus.UnitStatus{Name: nodeagentconfigv1alpha1.UnitName, ActiveState: "active"},
			systemddbus.UnitStatus{Name: nodeagentconfigv1alpha1.InitUnitName, ActiveState: "active"},
			systemddbus.UnitStatus{Name: "kubelet.service", ActiveState: "active"},
		)

		Eventually(func(g Gomega) *corev1.NodeCondition {
			return getNodeCondition(g)
		}).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady),
			"Status": Equal(corev1.ConditionTrue),
			"Reason": Equal("AllUnitsHealthy"),
		})))
	})

	It("should report unhealthy when an enabled unit is not loaded", func() {
		writeOSC(&extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Units: []extensionsv1alpha1.Unit{
					{Name: "missing.service", Enable: ptr.To(true)},
				},
			},
		})

		Eventually(func(g Gomega) *corev1.NodeCondition {
			return getNodeCondition(g)
		}).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":    Equal(nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady),
			"Status":  Equal(corev1.ConditionFalse),
			"Reason":  Equal("UnhealthyUnits"),
			"Message": ContainSubstring("missing.service: not loaded"),
		})))
	})

	It("should transition from unhealthy to healthy when unit recovers", func() {
		writeOSC(&extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Units: []extensionsv1alpha1.Unit{
					{Name: "recovering.service", Enable: ptr.To(true)},
				},
			},
		})

		fakeDBus.AddUnitsToList(
			systemddbus.UnitStatus{Name: "recovering.service", ActiveState: "failed"},
		)

		By("Wait for unhealthy condition")
		Eventually(func(g Gomega) *corev1.NodeCondition {
			return getNodeCondition(g)
		}).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Status": Equal(corev1.ConditionFalse),
		})))

		By("Simulate unit recovery")
		fakeDBus.SetUnits(
			systemddbus.UnitStatus{Name: "recovering.service", ActiveState: "active"},
		)

		By("Wait for healthy condition")
		Eventually(func(g Gomega) *corev1.NodeCondition {
			return getNodeCondition(g)
		}).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Status": Equal(corev1.ConditionTrue),
			"Reason": Equal("AllUnitsHealthy"),
		})))
	})

	It("should handle extension units with overrides", func() {
		writeOSC(&extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Units: []extensionsv1alpha1.Unit{
					{Name: "base.service", Enable: ptr.To(true)},
				},
			},
			Status: extensionsv1alpha1.OperatingSystemConfigStatus{
				ExtensionUnits: []extensionsv1alpha1.Unit{
					{Name: "base.service", Enable: ptr.To(false)},
					{Name: "extension.service", Enable: ptr.To(true)},
				},
			},
		})

		fakeDBus.AddUnitsToList(
			systemddbus.UnitStatus{Name: "base.service", ActiveState: "inactive"},
			systemddbus.UnitStatus{Name: "extension.service", ActiveState: "active"},
		)

		Eventually(func(g Gomega) *corev1.NodeCondition {
			return getNodeCondition(g)
		}).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady),
			"Status": Equal(corev1.ConditionTrue),
			"Reason": Equal("AllUnitsHealthy"),
		})))
	})

	It("should monitor an extension unit with drop-ins even without explicit enable", func() {
		writeOSC(&extensionsv1alpha1.OperatingSystemConfig{
			Status: extensionsv1alpha1.OperatingSystemConfigStatus{
				ExtensionUnits: []extensionsv1alpha1.Unit{
					{
						Name: "systemd-user-sessions.service",
						DropIns: []extensionsv1alpha1.DropIn{
							{Name: "dependencies.conf", Content: "[Unit]\nWants=systemd-tmpfiles-setup.service"},
						},
					},
				},
			},
		})

		fakeDBus.AddUnitsToList(
			systemddbus.UnitStatus{Name: "systemd-user-sessions.service", ActiveState: "inactive", SubState: "dead"},
		)

		Eventually(func(g Gomega) *corev1.NodeCondition {
			return getNodeCondition(g)
		}).Should(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":    Equal(nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady),
			"Status":  Equal(corev1.ConditionFalse),
			"Reason":  Equal("UnhealthyUnits"),
			"Message": ContainSubstring("systemd-user-sessions.service: inactive but should be enabled"),
		})))
	})
})
