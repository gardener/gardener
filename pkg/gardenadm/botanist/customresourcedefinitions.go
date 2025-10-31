// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/autoscaling/vpa"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	extensioncrds "github.com/gardener/gardener/pkg/component/extensions/crds"
	"github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/component/observability/logging/fluentoperator"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheusoperator"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// ReconcileCustomResourceDefinitions reconciles the custom resource definitions.
func (b *GardenadmBotanist) ReconcileCustomResourceDefinitions(ctx context.Context) error {
	vpaCRDDeployer, err := vpa.NewCRD(b.SeedClientSet.Client(), nil)
	if err != nil {
		return fmt.Errorf("failed creating VPA CRD deployer: %w", err)
	}

	prometheusCRDDeployer, err := prometheusoperator.NewCRDs(b.SeedClientSet.Client())
	if err != nil {
		return fmt.Errorf("failed creating Prometheus CRD deployer: %w", err)
	}

	fluentCRDDeployer, err := fluentoperator.NewCRDs(b.SeedClientSet.Client())
	if err != nil {
		return fmt.Errorf("failed creating fluent CRD deployer: %w", err)
	}

	extensionCRDDeployer, err := extensioncrds.NewCRD(b.SeedClientSet.Client(), true, true)
	if err != nil {
		return fmt.Errorf("failed creating extension CRD deployer: %w", err)
	}

	etcdCRDDeployer, err := etcd.NewCRD(b.SeedClientSet.Client(), b.Shoot.KubernetesVersion)
	if err != nil {
		return fmt.Errorf("failed creating etcd CRD deployer: %w", err)
	}

	deployers := map[string]component.Deployer{
		"VPA":        vpaCRDDeployer,
		"Prometheus": prometheusCRDDeployer,
		"Fluent":     fluentCRDDeployer,
		"Extension":  extensionCRDDeployer,
		"ETCD":       etcdCRDDeployer,
	}

	if b.Shoot.HasManagedInfrastructure() {
		deployers["Machine"], err = machinecontrollermanager.NewCRD(b.SeedClientSet.Client())
		if err != nil {
			return fmt.Errorf("failed creating machine CRD deployer: %w", err)
		}
	}

	for description, d := range deployers {
		if err := d.Deploy(ctx); err != nil {
			return fmt.Errorf("failed to deploy CustomResourceDefinition related to %s: %w", description, err)
		}
	}

	return nil
}

// EnsureCustomResourceDefinitionsReady ensures that the custom resource definitions are ready.
func (b *GardenadmBotanist) EnsureCustomResourceDefinitionsReady(ctx context.Context) error {
	crdList := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := b.SeedClientSet.Client().List(ctx, crdList); err != nil {
		return fmt.Errorf("failed to list CustomResourceDefinitions: %w", err)
	}

	for _, crd := range crdList.Items {
		if err := health.CheckCustomResourceDefinition(&crd); err != nil {
			return fmt.Errorf("CustomResourceDefinition %s is not ready yet: %w", crd.Name, err)
		}
	}

	return nil
}
