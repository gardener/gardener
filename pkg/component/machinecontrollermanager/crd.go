// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package machinecontrollermanager

import (
	"context"
	_ "embed"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var (
	//go:embed templates/crd-machine.sapcloud.io_machineclasses.yaml
	machineClassCRD string
	//go:embed templates/crd-machine.sapcloud.io_machinedeployments.yaml
	machineDeploymentCRD string
	//go:embed templates/crd-machine.sapcloud.io_machinesets.yaml
	machineSetCRD string
	//go:embed templates/crd-machine.sapcloud.io_machines.yaml
	machineCRD string

	crdResources []string
)

func init() {
	crdResources = []string{
		machineClassCRD,
		machineDeploymentCRD,
		machineSetCRD,
		machineCRD,
	}
}

type crd struct {
	client  client.Client
	applier kubernetes.Applier
}

// NewCRD can be used to deploy the CRD definitions for the machine-controller-manager.
func NewCRD(client client.Client, applier kubernetes.Applier) component.Deployer {
	return &crd{
		client:  client,
		applier: applier,
	}
}

// Deploy creates and updates the CRD definitions for the machine-controller-manager.
func (c *crd) Deploy(ctx context.Context) error {
	for _, resource := range crdResources {
		if err := c.applier.ApplyManifest(ctx, kubernetes.NewManifestReader([]byte(resource)), kubernetes.DefaultMergeFuncs); err != nil {
			return err
		}
	}

	return c.deleteLegacyCRDs(ctx)
}

func (c *crd) Destroy(ctx context.Context) error {
	for _, resource := range crdResources {
		reader := kubernetes.NewManifestReader([]byte(resource))

		obj, err := reader.Read()
		if err != nil {
			return fmt.Errorf("failed reading manifest: %w", err)
		}

		if err := gardenerutils.ConfirmDeletion(ctx, c.client, obj); client.IgnoreNotFound(err) != nil {
			return err
		}

		if err := c.applier.DeleteManifest(ctx, reader); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return c.deleteLegacyCRDs(ctx)
}

// TODO(rfranzke): Remove this code after Gardener v1.83 has been released.
func (c *crd) deleteLegacyCRDs(ctx context.Context) error {
	for _, name := range []string{
		"alicloudmachineclasses.machine.sapcloud.io",
		"awsmachineclasses.machine.sapcloud.io",
		"azuremachineclasses.machine.sapcloud.io",
		"gcpmachineclasses.machine.sapcloud.io",
		"openstackmachineclasses.machine.sapcloud.io",
		"packetmachineclasses.machine.sapcloud.io",
	} {
		obj := &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: name}}
		if err := gardenerutils.ConfirmDeletion(ctx, c.client, obj); client.IgnoreNotFound(err) != nil {
			return err
		}
		if err := kubernetesutils.DeleteObject(ctx, c.client, obj); err != nil {
			return err
		}
	}

	return nil
}
