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
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Port is the port exposed by the kube-apiserver.
	Port = 443

	containerNameKubeAPIServer            = "kube-apiserver"
	containerNameVPNSeed                  = "vpn-seed"
	containerNameAPIServerProxyPodMutator = "apiserver-proxy-pod-mutator"
)

// Interface contains functions for a kube-apiserver deployer.
type Interface interface {
	component.DeployWaiter
	// GetValues returns the current configuration values of the deployer.
	GetValues() Values
	// SetAutoscalingReplicas sets the Replicas field in the AutoscalingConfig of the Values of the deployer.
	SetAutoscalingReplicas(*int32)
}

// Values contains configuration values for the kube-apiserver resources.
type Values struct {
	// Autoscaling contains information for configuring autoscaling settings for the kube-apiserver.
	Autoscaling AutoscalingConfig
	// SNI contains information for configuring SNI settings for the kube-apiserver.
	SNI SNIConfig
}

// AutoscalingConfig contains information for configuring autoscaling settings for the kube-apiserver.
type AutoscalingConfig struct {
	// HVPAEnabled states whether an HVPA object shall be deployed. If false, HPA and VPA will be used.
	HVPAEnabled bool
	// Replicas is the number of pod replicas for the kube-apiserver.
	Replicas *int32
	// MinReplicas are the minimum Replicas for horizontal autoscaling.
	MinReplicas int32
	// MaxReplicas are the maximum Replicas for horizontal autoscaling.
	MaxReplicas int32
	// UseMemoryMetricForHvpaHPA states whether the memory metric shall be used when the HPA is configured in an HVPA
	// resource.
	UseMemoryMetricForHvpaHPA bool
	// ScaleDownDisabledForHvpa states whether scale-down shall be disabled when HPA or VPA are configured in an HVPA
	// resource.
	ScaleDownDisabledForHvpa bool
}

// SNIConfig contains information for configuring SNI settings for the kube-apiserver.
type SNIConfig struct {
	// PodMutatorEnabled states whether the pod mutator is enabled.
	PodMutatorEnabled bool
}

// New creates a new instance of DeployWaiter for the kube-apiserver.
func New(client client.Client, namespace string, values Values) Interface {
	return &kubeAPIServer{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type kubeAPIServer struct {
	client    client.Client
	namespace string
	values    Values
}

func (k *kubeAPIServer) Deploy(ctx context.Context) error {
	var (
		podDisruptionBudget     = k.emptyPodDisruptionBudget()
		horizontalPodAutoscaler = k.emptyHorizontalPodAutoscaler()
		verticalPodAutoscaler   = k.emptyVerticalPodAutoscaler()
		hvpa                    = k.emptyHVPA()
	)

	if err := k.reconcilePodDisruptionBudget(ctx, podDisruptionBudget); err != nil {
		return err
	}

	if err := k.reconcileHorizontalPodAutoscaler(ctx, horizontalPodAutoscaler); err != nil {
		return err
	}

	if err := k.reconcileVerticalPodAutoscaler(ctx, verticalPodAutoscaler); err != nil {
		return err
	}

	if err := k.reconcileHVPA(ctx, hvpa); err != nil {
		return err
	}

	return nil
}

func (k *kubeAPIServer) Destroy(ctx context.Context) error {
	return kutil.DeleteObjects(ctx, k.client,
		k.emptyHorizontalPodAutoscaler(),
		k.emptyVerticalPodAutoscaler(),
		k.emptyHVPA(),
		k.emptyPodDisruptionBudget(),
	)
}

func (k *kubeAPIServer) Wait(_ context.Context) error        { return nil }
func (k *kubeAPIServer) WaitCleanup(_ context.Context) error { return nil }

func (k *kubeAPIServer) GetValues() Values {
	return k.values
}
func (k *kubeAPIServer) SetAutoscalingReplicas(replicas *int32) {
	k.values.Autoscaling.Replicas = replicas
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer,
	}
}
