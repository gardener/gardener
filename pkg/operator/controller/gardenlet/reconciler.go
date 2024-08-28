// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controller/gardenletdeployer"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/oci"
)

// Reconciler reconciles the Gardenlet.
type Reconciler struct {
	RuntimeCluster        cluster.Cluster
	RuntimeClient         client.Client
	VirtualConfig         *rest.Config
	VirtualAPIReader      client.Reader
	VirtualClient         client.Client
	Config                config.GardenletDeployerControllerConfig
	Clock                 clock.Clock
	Recorder              record.EventRecorder
	HelmRegistry          oci.Interface
	GardenNamespaceTarget string
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	gardenlet := &seedmanagementv1alpha1.Gardenlet{}
	if err := r.VirtualClient.Get(ctx, request.NamespacedName, gardenlet); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if !r.seedDoesNotExist(ctx, gardenlet) {
		return reconcile.Result{}, r.cleanupKubeconfigSecret(ctx, log, gardenlet)
	}

	// Deletion is not implemented - once gardenlet got deployed by gardener-operator, it doesn't care about it ever
	// again. Gardenlet will perform self-upgrades. Seed deprovisioning must be handled by human operators.
	return reconcile.Result{}, r.reconcile(ctx, log, gardenlet, r.newActuator(gardenlet))
}

func (r *Reconciler) newActuator(gardenlet *seedmanagementv1alpha1.Gardenlet) gardenletdeployer.Interface {
	return &gardenletdeployer.Actuator{
		GardenConfig:    r.VirtualConfig,
		GardenAPIReader: r.VirtualAPIReader,
		GardenClient:    r.VirtualClient,
		GetTargetClientFunc: func(ctx context.Context) (kubernetes.Interface, error) {
			if gardenlet.Spec.KubeconfigSecretRef == nil {
				return kubernetes.NewWithConfig(
					kubernetes.WithRESTConfig(r.RuntimeCluster.GetConfig()),
					kubernetes.WithRuntimeAPIReader(r.RuntimeCluster.GetAPIReader()),
					kubernetes.WithRuntimeClient(r.RuntimeCluster.GetClient()),
					kubernetes.WithRuntimeCache(r.RuntimeCluster.GetCache()),
					kubernetes.WithDisabledCachedClient(),
				)
			}
			return kubernetes.NewClientFromSecret(
				ctx,
				r.RuntimeClient,
				gardenlet.Namespace,
				gardenlet.Spec.KubeconfigSecretRef.Name,
				kubernetes.WithDisabledCachedClient(),
			)
		},
		CheckIfVPAAlreadyExists: func(_ context.Context) (bool, error) {
			return false, nil
		},
		GetInfrastructureSecret: func(ctx context.Context) (*corev1.Secret, error) {
			seedTemplate, _, err := helper.ExtractSeedTemplateAndGardenletConfig(gardenlet.GetName(), &gardenlet.Spec.Config)
			if err != nil {
				return nil, fmt.Errorf("failed to extract seed template and gardenlet config: %w", err)
			}

			if seedTemplate.Spec.Backup == nil {
				return nil, nil
			}
			return kubernetesutils.GetSecretByReference(ctx, r.VirtualClient, &seedTemplate.Spec.Backup.SecretRef)
		},
		GetTargetDomain: func() string {
			return ""
		},
		ApplyGardenletChart: func(ctx context.Context, targetChartApplier kubernetes.ChartApplier, values map[string]interface{}) error {
			archive, err := r.HelmRegistry.Pull(ctx, &gardenlet.Spec.Deployment.Helm.OCIRepository)
			if err != nil {
				return fmt.Errorf("failed pulling Helm chart from OCI repository: %w", err)
			}

			return targetChartApplier.ApplyFromArchive(ctx, archive, r.GardenNamespaceTarget, "gardenlet", kubernetes.Values(values))
		},
		Clock:                 r.Clock,
		ValuesHelper:          gardenletdeployer.NewValuesHelper(nil),
		Recorder:              r.Recorder,
		GardenNamespaceTarget: r.GardenNamespaceTarget,
	}
}

func (r *Reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	gardenlet *seedmanagementv1alpha1.Gardenlet,
	actuator gardenletdeployer.Interface,
) (
	err error,
) {
	status := gardenlet.Status.DeepCopy()
	status.ObservedGeneration = gardenlet.Generation

	log.V(1).Info("Reconciling")
	status.Conditions, err = actuator.Reconcile(ctx, log, gardenlet, status.Conditions, &gardenlet.Spec.Deployment.GardenletDeployment, &gardenlet.Spec.Config, seedmanagementv1alpha1.BootstrapToken, false)
	if err != nil {
		if updateErr := r.updateStatus(ctx, gardenlet, status); updateErr != nil {
			log.Error(updateErr, "Could not update status", "status", status)
		}
		return fmt.Errorf("could not reconcile Gardenlet %s creation: %w", client.ObjectKeyFromObject(gardenlet), err)
	}

	log.Info("Reconciliation finished")
	if err := r.cleanupKubeconfigSecret(ctx, log, gardenlet); err != nil {
		return err
	}

	return r.updateStatus(ctx, gardenlet, status)
}

func (r *Reconciler) updateStatus(ctx context.Context, gardenlet *seedmanagementv1alpha1.Gardenlet, status *seedmanagementv1alpha1.GardenletStatus) error {
	if status == nil {
		return nil
	}
	patch := client.StrategicMergeFrom(gardenlet.DeepCopy())
	gardenlet.Status = *status
	return r.VirtualClient.Status().Patch(ctx, gardenlet, patch)
}

func (r *Reconciler) cleanupKubeconfigSecret(ctx context.Context, log logr.Logger, gardenlet *seedmanagementv1alpha1.Gardenlet) error {
	if gardenlet.Spec.KubeconfigSecretRef == nil {
		return nil
	}

	log.Info("Deleting kubeconfig secret and removing reference in spec")
	if err := kubernetesutils.DeleteObject(ctx, r.VirtualClient, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: gardenlet.Spec.KubeconfigSecretRef.Name, Namespace: gardenlet.Namespace}}); err != nil {
		return fmt.Errorf("could not delete kubeconfig secret: %w", err)
	}

	patch := client.MergeFrom(gardenlet.DeepCopy())
	gardenlet.Spec.KubeconfigSecretRef = nil
	if err := r.VirtualClient.Patch(ctx, gardenlet, patch); err != nil {
		return fmt.Errorf("could not remove kubeconfig secret ref: %w", err)
	}

	return nil
}
