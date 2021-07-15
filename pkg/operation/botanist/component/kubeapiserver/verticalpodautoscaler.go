// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeapiserver

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
)

func (k *kubeAPIServer) emptyVerticalPodAutoscaler() *autoscalingv1beta2.VerticalPodAutoscaler {
	return &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer + "-vpa", Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileVerticalPodAutoscaler(ctx context.Context, verticalPodAutoscaler *autoscalingv1beta2.VerticalPodAutoscaler, deployment *appsv1.Deployment) error {
	if k.values.Autoscaling.HVPAEnabled {
		return kutil.DeleteObject(ctx, k.client.Client(), verticalPodAutoscaler)
	}

	vpaUpdateMode := autoscalingv1beta2.UpdateModeOff

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), verticalPodAutoscaler, func() error {
		verticalPodAutoscaler.Spec = autoscalingv1beta2.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       deployment.Name,
			},
			UpdatePolicy: &autoscalingv1beta2.PodUpdatePolicy{
				UpdateMode: &vpaUpdateMode,
			},
		}
		return nil
	})
	return err
}
