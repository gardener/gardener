// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/gardener/gardener/pkg/component/autoscaling/vpa"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	extensioncrds "github.com/gardener/gardener/pkg/component/extensions/crds"
	"github.com/gardener/gardener/pkg/component/observability/logging/fluentoperator"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheusoperator"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// ReconcileCustomResourceDefinitions reconciles the custom resource definitions.
func (b *AutonomousBotanist) ReconcileCustomResourceDefinitions(ctx context.Context) error {
	vpaCRDDeployer, err := vpa.NewCRD(b.SeedClientSet.Client(), b.SeedClientSet.Applier(), nil)
	if err != nil {
		return fmt.Errorf("failed creating VPA CRD deployer: %w", err)
	}

	prometheusCRDDeployer, err := prometheusoperator.NewCRDs(b.SeedClientSet.Client(), b.SeedClientSet.Applier())
	if err != nil {
		return fmt.Errorf("failed creating Prometheus CRD deployer: %w", err)
	}

	fluentCRDDeployer, err := fluentoperator.NewCRDs(b.SeedClientSet.Client(), b.SeedClientSet.Applier())
	if err != nil {
		return fmt.Errorf("failed creating fluent CRD deployer: %w", err)
	}

	extensionCRDDeployer, err := extensioncrds.NewCRD(b.SeedClientSet.Client(), b.SeedClientSet.Applier(), true, true)
	if err != nil {
		return fmt.Errorf("failed creating extension CRD deployer: %w", err)
	}

	etcdCRDDeployer, err := etcd.NewCRD(b.SeedClientSet.Client(), b.SeedClientSet.Applier(), b.Shoot.KubernetesVersion)
	if err != nil {
		return fmt.Errorf("failed creating etcd CRD deployer: %w", err)
	}

	for description, deploy := range map[string]func(context.Context) error{
		"VPA":        vpaCRDDeployer.Deploy,
		"Prometheus": prometheusCRDDeployer.Deploy,
		"Fluent":     fluentCRDDeployer.Deploy,
		"Extension":  extensionCRDDeployer.Deploy,
		"ETCD":       etcdCRDDeployer.Deploy,
	} {
		if err := deploy(ctx); err != nil {
			return fmt.Errorf("failed to deploy CustomResourceDefinition related to %s: %w", description, err)
		}
	}

	return nil
}

// EnsureCustomResourceDefinitionsReady ensures that the custom resource definitions are ready.
func (b *AutonomousBotanist) EnsureCustomResourceDefinitionsReady(ctx context.Context) error {
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
