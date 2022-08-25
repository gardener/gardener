// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package extensionscheck_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils/test"
)

const (
	conditionThreshold = 1 * time.Second
	syncPeriod         = 1 * time.Millisecond
)

var _ = Describe("Seed ExtensionsCheck controller tests", func() {
	var (
		seed *gardencorev1beta1.Seed
		ci1  *gardencorev1beta1.ControllerInstallation
		ci2  *gardencorev1beta1.ControllerInstallation
	)

	BeforeEach(func() {
		By("Create Seed")
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Region: "region",
					Type:   "providerType",
				},
				Settings: &gardencorev1beta1.SeedSettings{
					ShootDNS:   &gardencorev1beta1.SeedSettingShootDNS{Enabled: true},
					Scheduling: &gardencorev1beta1.SeedSettingScheduling{Visible: true},
				},
				Networks: gardencorev1beta1.SeedNetworks{
					Pods:     "10.0.0.0/16",
					Services: "10.1.0.0/16",
					Nodes:    pointer.String("10.2.0.0/16"),
					ShootDefaults: &gardencorev1beta1.ShootNetworks{
						Pods:     pointer.String("100.128.0.0/11"),
						Services: pointer.String("100.72.0.0/13"),
					},
				},
				DNS: gardencorev1beta1.SeedDNS{
					IngressDomain: pointer.String("someingress.example.com"),
				},
			},
		}
		Expect(testClient.Create(ctx, seed)).To(Succeed())
		log.Info("Created seed for test", "seed", client.ObjectKeyFromObject(seed))

		DeferCleanup(func() {
			By("Delete Seed")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, seed))).To(Succeed())
		})

		By("Create ControllerInstallations")
		ci1 = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "foo-1-",
			},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				SeedRef: corev1.ObjectReference{
					Name: seed.Name,
				},
				RegistrationRef: corev1.ObjectReference{
					Name: "foo-registration",
				},
				DeploymentRef: &corev1.ObjectReference{
					Name: "foo-deployment",
				},
			},
		}

		ci2 = ci1.DeepCopy()
		ci2.SetGenerateName("foo-2-")

		//This is required so that the ExtensionsReady condition is created with appropriate lastUpdateTimestamp and lastTransitionTimestamp.
		DeferCleanup(test.WithVars(
			&gardencorev1beta1helper.Now, func() metav1.Time { return metav1.Time{Time: fakeClock.Now()} },
		))

		for _, controllerInstallation := range []*gardencorev1beta1.ControllerInstallation{ci1, ci2} {
			Expect(testClient.Create(ctx, controllerInstallation)).To(Succeed())

			controllerInstallation.Status = gardencorev1beta1.ControllerInstallationStatus{
				Conditions: []gardencorev1beta1.Condition{
					{Type: "Valid", Status: gardencorev1beta1.ConditionTrue},
					{Type: "Installed", Status: gardencorev1beta1.ConditionTrue},
					{Type: "Healthy", Status: gardencorev1beta1.ConditionTrue},
				},
			}
			Expect(testClient.Status().Update(ctx, controllerInstallation)).To(Succeed())
			log.Info("Created and updated controllerinstallation for test", "controllerinstallation", client.ObjectKeyFromObject(controllerInstallation))
		}

		DeferCleanup(func() {
			By("Delete ControllerInstallations")
			for _, controllerInstallation := range []*gardencorev1beta1.ControllerInstallation{ci1, ci2} {
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, controllerInstallation))).To(Succeed())
			}
		})

		By("waiting until ExtensionsReady condition is set to True")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
			g.Expect(seed.Status.Conditions).To(containCondition(ofType(gardencorev1beta1.SeedExtensionsReady), withStatus(gardencorev1beta1.ConditionTrue), withReason("AllExtensionsReady")))
		}).Should(Succeed())
	})

	var tests = func(failedConditionType gardencorev1beta1.ConditionType, reason string) {
		It("should set ExtensionsReady to Progressing and eventually to False when condition threshold expires", func() {
			for i, condition := range ci1.Status.Conditions {
				if condition.Type == failedConditionType {
					ci1.Status.Conditions[i].Status = gardencorev1beta1.ConditionFalse
					break
				}
			}
			Expect(testClient.Status().Update(ctx, ci1)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				g.Expect(seed.Status.Conditions).To(containCondition(ofType(gardencorev1beta1.SeedExtensionsReady), withStatus(gardencorev1beta1.ConditionProgressing), withReason(reason)))
			}).Should(Succeed())

			fakeClock.Step(conditionThreshold + 1*time.Second)
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				g.Expect(seed.Status.Conditions).To(containCondition(ofType(gardencorev1beta1.SeedExtensionsReady), withStatus(gardencorev1beta1.ConditionFalse), withReason(reason)))
			}).Should(Succeed())
		})
	}

	Context("when one ControllerInstallation becomes not valid", func() {
		tests(gardencorev1beta1.ControllerInstallationValid, "NotAllExtensionsValid")
	})

	Context("when one controllerinstallation is not installed", func() {
		tests(gardencorev1beta1.ControllerInstallationInstalled, "NotAllExtensionsInstalled")
	})

	Context("when one controllerinstallation is not healthy", func() {
		tests(gardencorev1beta1.ControllerInstallationHealthy, "NotAllExtensionsHealthy")
	})
})

func containCondition(matchers ...gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return ContainElement(And(matchers...))
}

func ofType(conditionType gardencorev1beta1.ConditionType) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Type": Equal(conditionType),
	})
}

func withStatus(status gardencorev1beta1.ConditionStatus) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Status": Equal(status),
	})
}

func withReason(reason string) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Reason": Equal(reason),
	})
}
