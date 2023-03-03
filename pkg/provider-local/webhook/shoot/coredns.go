// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

func (m *mutator) mutateCoreDNSDeployment(ctx context.Context, client client.Client, deployment *appsv1.Deployment) error {
	nodeList := corev1.NodeList{}
	if err := client.List(ctx, &nodeList); err != nil {
		return fmt.Errorf("failed to list the nodes")
	}

	addPodAntiAffinity := func(podAffinityTerms []corev1.WeightedPodAffinityTerm, podAffinityTerm corev1.WeightedPodAffinityTerm) []corev1.WeightedPodAffinityTerm {
		for _, affinityTerms := range podAffinityTerms {
			if affinityTerms.PodAffinityTerm.TopologyKey == corev1.LabelHostname {
				return podAffinityTerms
			}
		}

		return append(podAffinityTerms, podAffinityTerm)
	}

	if len(nodeList.Items) > 1 {
		if deployment.Spec.Template.Spec.Affinity == nil {
			deployment.Spec.Template.Spec.Affinity = &corev1.Affinity{}
		}

		if deployment.Spec.Template.Spec.Affinity.PodAntiAffinity == nil {
			deployment.Spec.Template.Spec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
		}

		labelSelector := metav1.LabelSelector{MatchLabels: deployment.Spec.Template.Labels}
		podAffinityTerm := corev1.WeightedPodAffinityTerm{
			Weight: 100,
			PodAffinityTerm: corev1.PodAffinityTerm{
				LabelSelector: &labelSelector,
				TopologyKey:   corev1.LabelHostname,
			},
		}

		podAffinityTerms := addPodAntiAffinity(deployment.Spec.Template.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution, podAffinityTerm)
		deployment.Spec.Template.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = podAffinityTerms
	}

	return nil
}

func (m *mutator) mutateCoreDNSHpa(ctx context.Context, shootClient client.Client) error {
	hpa := autoscalingv2.HorizontalPodAutoscaler{}
	if err := shootClient.Get(ctx, types.NamespacedName{Name: "coredns", Namespace: metav1.NamespaceSystem}, &hpa); client.IgnoreNotFound(err) != nil {
		return err
	}

	nodeList := corev1.NodeList{}
	if err := shootClient.List(ctx, &nodeList); err != nil {
		return fmt.Errorf("failed to list the nodes")
	}

	requiredReplicas := 2
	if len(nodeList.Items) > 2 {
		requiredReplicas = len(nodeList.Items)
	}

	patch := client.MergeFrom(hpa.DeepCopy())
	hpa.Annotations = utils.MergeStringMaps(hpa.Annotations, map[string]string{
		resourcesv1alpha1.HighAvailabilityConfigReplicas: fmt.Sprintf("%v", requiredReplicas),
	})

	return shootClient.Patch(ctx, &hpa, patch)
}

func (m *mutator) mutateCoreDNSService(_ context.Context, service *corev1.Service) error {
	serviceInternalTrafficPolicy := corev1.ServiceInternalTrafficPolicyLocal
	service.Spec.InternalTrafficPolicy = &serviceInternalTrafficPolicy

	return nil
}
