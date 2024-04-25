// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hvpa

import (
	"context"
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
)

//go:embed templates/crd-autoscaling.k8s.io_hvpas.yaml
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
