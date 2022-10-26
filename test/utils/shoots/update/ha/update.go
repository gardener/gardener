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

package ha

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/test/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RunTest runs the HA control-plane upgrade tests for an existing shoot cluster.
func RunTest(
	ctx context.Context,
	f *framework.ShootFramework,
	haType gardencorev1beta1.FailureToleranceType,

) {
	By("updating shoot's control plane failTolearanceType to " + string(haType))
	Expect(f.UpdateShoot(ctx, func(shoot *gardencorev1beta1.Shoot) error {
		shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
			HighAvailability: &gardencorev1beta1.HighAvailability{
				FailureTolerance: gardencorev1beta1.FailureTolerance{
					Type: haType,
				},
			},
		}
		return nil
	})).To(Succeed())

	By("verifying shoot's control plane components for failTolearanceType: " + string(haType))
	verifyTSC(ctx, f.SeedClient, f.Shoot, f.ShootSeedNamespace())
	verifyEtcdAffinity(ctx, f.SeedClient, f.Shoot, f.ShootSeedNamespace())
}

func getTSCForZone(labels map[string]string) corev1.TopologySpreadConstraint {
	return corev1.TopologySpreadConstraint{
		TopologyKey:       corev1.LabelTopologyZone,
		MaxSkew:           1,
		WhenUnsatisfiable: corev1.DoNotSchedule,
		LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
	}
}

func getTSCForNode(labels map[string]string) corev1.TopologySpreadConstraint {
	return corev1.TopologySpreadConstraint{
		TopologyKey:       corev1.LabelHostname,
		MaxSkew:           1,
		WhenUnsatisfiable: corev1.DoNotSchedule,
		LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
	}
}

func verifyTSC(ctx context.Context, seedClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot, namespace string) {
	c := seedClient.Client()
	commponents := map[string]map[string]string{
		"gardener-resource-manager": {
			"app":                 "gardener-resource-manager",
			"gardener.cloud/role": "controlplane",
		},
		"kube-apiserver": {
			"app":  "kubernetes",
			"role": "apiserver",
		},
	}

	for name, l := range commponents {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
		Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

		constraints := []corev1.TopologySpreadConstraint{getTSCForNode(l)}
		if gardencorev1beta1helper.IsMultiZonalShootControlPlane(shoot) {
			constraints = append(constraints, getTSCForZone(l))
		}

		if len(deployment.Spec.Template.GetLabels()["checksum/pod-template"]) > 0 {
			l["checksum/pod-template"] = deployment.Spec.Template.GetLabels()["checksum/pod-template"]
		}

		Expect(deployment.Spec.Template.Spec.TopologySpreadConstraints).To(ConsistOf(constraints))
	}
}

func getAffinity(topologyKey, role string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					TopologyKey: topologyKey,
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"gardener.cloud/role": "controlplane",
							"role":                role,
						},
					},
				},
			},
		},
	}
}

func verifyEtcdAffinity(ctx context.Context, seedClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot, namespace string) error {
	var affinity *corev1.Affinity
	c := seedClient.Client()
	for _, componentName := range []string{"events", "main"} {

		if gardencorev1beta1helper.IsMultiZonalShootControlPlane(shoot) {
			affinity = getAffinity(corev1.LabelTopologyZone, componentName)
		} else {
			affinity = getAffinity(corev1.LabelHostname, componentName)
		}

		sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{
			Name:      "etcd-" + componentName,
			Namespace: namespace,
		}}
		Expect(c.Get(ctx, client.ObjectKeyFromObject(sts), sts)).To(Succeed())
		Expect(sts.Spec.Template.Spec.Affinity).Should(BeEquivalentTo(affinity))
	}
	return nil
}
