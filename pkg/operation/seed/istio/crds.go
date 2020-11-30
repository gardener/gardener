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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type crds struct {
	kubernetes.ChartApplier
	chartPath      string
	client         crclient.Client
	deprecatedCRDs []apiextensionsv1.CustomResourceDefinition
}

// NewIstioCRD can be used to deploy istio CRDs.
// Destroy does nothing.
func NewIstioCRD(
	applier kubernetes.ChartApplier,
	chartsRootPath string,
	client crclient.Client,
) component.DeployWaiter {
	return &crds{
		ChartApplier: applier,
		chartPath:    filepath.Join(chartsRootPath, "istio", "istio-crds"),
		client:       client,
		deprecatedCRDs: []apiextensionsv1.CustomResourceDefinition{
			{ObjectMeta: metav1.ObjectMeta{Name: "attributemanifests.config.istio.io"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "clusterrbacconfigs.rbac.istio.io"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "handlers.config.istio.io"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "httpapispecbindings.config.istio.io"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "httpapispecs.config.istio.io"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "instances.config.istio.io"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "meshpolicies.authentication.istio.io"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "policies.authentication.istio.io"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "quotaspecbindings.config.istio.io"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "quotaspecs.config.istio.io"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "rbacconfigs.rbac.istio.io"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "rules.config.istio.io"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "servicerolebindings.rbac.istio.io"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "serviceroles.rbac.istio.io"}},
		},
	}
}

func (c *crds) Deploy(ctx context.Context) error {
	for _, deprecatedCRD := range c.deprecatedCRDs {
		if err := crclient.IgnoreNotFound(c.client.Delete(ctx, &deprecatedCRD)); err != nil {
			return err
		}
	}

	return c.Apply(ctx, c.chartPath, "", "istio")
}

func (c *crds) Destroy(ctx context.Context) error {
	// istio cannot be safely removed
	return nil
}

func (c *crds) Wait(ctx context.Context) error {
	return nil
}

func (c *crds) WaitCleanup(ctx context.Context) error {
	return nil
}
