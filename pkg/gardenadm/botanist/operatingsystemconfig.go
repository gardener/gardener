// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	operatingsystemconfigcontroller "github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/nodeagent/registry"
)

// DeployOperatingSystemConfigSecretForNodeAgent deploys the OperatingSystemConfig resource and adds its content into
// a Secret so that gardener-node-agent can read it and reconcile its content.
func (b *AutonomousBotanist) DeployOperatingSystemConfigSecretForNodeAgent(ctx context.Context) error {
	oscData, controlPlaneWorkerPoolName, err := b.deployOperatingSystemConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed deploying OperatingSystemConfig: %w", err)
	}

	b.operatingSystemConfigSecret, err = nodeagent.OperatingSystemConfigSecret(ctx, b.SeedClientSet.Client(), oscData.Object, oscData.GardenerNodeAgentSecretName, controlPlaneWorkerPoolName)
	if err != nil {
		return fmt.Errorf("failed computing the OperatingSystemConfig secret for gardener-node-agent for pool %q: %w", controlPlaneWorkerPoolName, err)
	}

	return b.SeedClientSet.Client().Create(ctx, b.operatingSystemConfigSecret)
}

func (b *AutonomousBotanist) deployOperatingSystemConfig(ctx context.Context) (*operatingsystemconfig.Data, string, error) {
	files, err := b.filesForStaticControlPlanePods(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed computing files for static control plane pods: %w", err)
	}

	if err := b.Botanist.DeployOperatingSystemConfig(ctx); err != nil {
		return nil, "", fmt.Errorf("failed deploying OperatingSystemConfig resource: %w", err)
	}

	controlPlaneWorkerPool := v1beta1helper.ControlPlaneWorkerPoolForShoot(b.Shoot.GetInfo())
	if controlPlaneWorkerPool == nil {
		return nil, "", fmt.Errorf("failed fetching the control plane worker pool for the shoot")
	}

	oscData, ok := b.Shoot.Components.Extensions.OperatingSystemConfig.WorkerPoolNameToOperatingSystemConfigsMap()[controlPlaneWorkerPool.Name]
	if !ok {
		return nil, "", fmt.Errorf("failed fetching the generated OperatingSystemConfig data for the control plane worker pool %q", controlPlaneWorkerPool.Name)
	}
	osc := oscData.Original.Object

	patch := client.MergeFrom(osc.DeepCopy())
	osc.Spec.Files = append(osc.Spec.Files, files...)
	if err := b.SeedClientSet.Client().Patch(ctx, osc, patch); err != nil {
		return nil, "", fmt.Errorf("failed patching OperatingSystemConfig with additional files for static control plane pods: %w", err)
	}

	return &oscData.Original, controlPlaneWorkerPool.Name, nil
}

// ApplyOperatingSystemConfig runs gardener-node-agent's reconciliation logic in order to apply the
// OperatingSystemConfig.
func (b *AutonomousBotanist) ApplyOperatingSystemConfig(ctx context.Context) error {
	if b.operatingSystemConfigSecret == nil {
		return fmt.Errorf("operating system config secret is nil, make sure to call DeployOperatingSystemConfigSecretForNodeAgent first")
	}

	reconcilerCtx, cancelFunc := context.WithCancel(ctx)
	reconcilerCtx = log.IntoContext(reconcilerCtx, b.Logger.WithName("operatingsystemconfig-reconciler"))

	_, err := (&operatingsystemconfigcontroller.Reconciler{
		Client: b.SeedClientSet.Client(),
		Config: nodeagentconfigv1alpha1.OperatingSystemConfigControllerConfig{
			SyncPeriod:        &metav1.Duration{Duration: time.Minute},
			SecretName:        b.operatingSystemConfigSecret.Name,
			KubernetesVersion: b.Shoot.KubernetesVersion,
		},
		CancelContext: cancelFunc,
		Recorder:      &record.FakeRecorder{},
		Extractor:     registry.NewExtractor(),
		HostName:      b.HostName,
		DBus:          b.DBus,
		FS:            b.FS,
	}).Reconcile(reconcilerCtx, reconcile.Request{NamespacedName: types.NamespacedName{Name: b.operatingSystemConfigSecret.Name, Namespace: b.operatingSystemConfigSecret.Namespace}})
	return err
}
