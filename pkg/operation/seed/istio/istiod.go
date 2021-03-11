// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package istio

import (
	"context"
	"path/filepath"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type istiod struct {
	namespace string
	values    *IstiodValues
	kubernetes.ChartApplier
	chartPath string
	client    crclient.Client
}

// IstiodValues holds values for the istio-istiod chart.
type IstiodValues struct {
	TrustDomain string `json:"trustDomain,omitempty"`
	Image       string `json:"image,omitempty"`
}

// NewIstiod can be used to deploy istio's istiod in a namespace.
// Destroy does nothing.
func NewIstiod(
	values *IstiodValues,
	namespace string,
	applier kubernetes.ChartApplier,
	chartsRootPath string,
	client crclient.Client,
) component.DeployWaiter {
	return &istiod{
		values:       values,
		namespace:    namespace,
		ChartApplier: applier,
		chartPath:    filepath.Join(chartsRootPath, istioReleaseName, "istio-istiod"),
		client:       client,
	}
}

func (i *istiod) Deploy(ctx context.Context) error {
	if err := i.client.Create(
		ctx,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: i.namespace,
				Labels: map[string]string{
					"istio-operator-managed": "Reconcile",
					"istio-injection":        "disabled",
				},
			},
		},
	); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	// TODO (mvladev): Remove this in next release
	if err := client.IgnoreNotFound(i.client.Delete(ctx, &autoscalingv1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "istiod",
			Namespace: i.namespace,
		},
	})); err != nil {
		return err
	}

	applierOptions := kubernetes.CopyApplierOptions(kubernetes.DefaultMergeFuncs)

	return i.Apply(ctx, i.chartPath, i.namespace, istioReleaseName, kubernetes.Values(i.values), applierOptions)
}

func (i *istiod) Destroy(ctx context.Context) error {
	// istio cannot be safely removed
	return nil
}

func (i *istiod) Wait(ctx context.Context) error {
	return nil
}

func (i *istiod) WaitCleanup(ctx context.Context) error {
	return nil
}
