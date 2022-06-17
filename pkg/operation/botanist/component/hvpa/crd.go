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

package hvpa

import (
	"context"
	_ "embed"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:embed templates/crd.tpl.yaml
var crd string

type crdDeployer struct {
	applier kubernetes.Applier
}

// NewCRD can be used to deploy the CRD definitions for the HVPA controller.
func NewCRD(applier kubernetes.Applier) component.Deployer {
	return &crdDeployer{applier: applier}
}

func (v *crdDeployer) Deploy(ctx context.Context) error {
	return v.applier.ApplyManifest(ctx, kubernetes.NewManifestReader([]byte(crd)), kubernetes.DefaultMergeFuncs)
}

func (v *crdDeployer) Destroy(ctx context.Context) error {
	return client.IgnoreNotFound(v.applier.DeleteManifest(ctx, kubernetes.NewManifestReader([]byte(crd))))
}
