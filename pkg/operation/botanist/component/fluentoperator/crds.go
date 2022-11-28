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

package fluentoperator

import (
	"context"
	_ "embed"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/flow"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	//go:embed assets/crd-fluentbit.fluent.io_clusterfilters.yaml
	clusterFilterCRD string
	//go:embed assets/crd-fluentbit.fluent.io_clusterfluentbitconfigs.yaml
	clusterFBConfigCRD string
	//go:embed assets/crd-fluentbit.fluent.io_clusterinputs.yaml
	clusterInputCRD string
	//go:embed assets/crd-fluentbit.fluent.io_clusteroutputs.yaml
	clusterOutputCRD string
	//go:embed assets/crd-fluentbit.fluent.io_clusterparsers.yaml
	clusterParserCRD string
	//go:embed assets/crd-fluentbit.fluent.io_fluentbits.yaml
	fluentBitCRD string

	resources []string
)

func init() {
	resources = append(resources,
		clusterFilterCRD,
		clusterFBConfigCRD,
		clusterInputCRD,
		clusterOutputCRD,
		clusterParserCRD,
		fluentBitCRD,
	)
}

type fluentOperatorCRDs struct {
	applier kubernetes.Applier
}

// NewFluentOperatorCRD can be used to deploy fluent operator CRDs.
func NewFluentOperatorCRD(a kubernetes.Applier) component.DeployWaiter {
	return &fluentOperatorCRDs{
		applier: a,
	}
}

// Deploy creates and updates the CRD definitions for the fluent operator.
func (c *fluentOperatorCRDs) Deploy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, resource := range resources {
		r := resource
		fns = append(fns, func(ctx context.Context) error {
			return c.applier.ApplyManifest(ctx, kubernetes.NewManifestReader([]byte(r)), kubernetes.DefaultMergeFuncs)
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// Destroy deletes the CRDs for the fluent operator
func (c *fluentOperatorCRDs) Destroy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, resource := range resources {
		r := resource
		fns = append(fns, func(ctx context.Context) error {
			return client.IgnoreNotFound(c.applier.DeleteManifest(ctx, kubernetes.NewManifestReader([]byte(r))))
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// Wait does nothing
func (c *fluentOperatorCRDs) Wait(ctx context.Context) error {
	return nil
}

// WaitCleanup does nothing
func (c *fluentOperatorCRDs) WaitCleanup(ctx context.Context) error {
	return nil
}
