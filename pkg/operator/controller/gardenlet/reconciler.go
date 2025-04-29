// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet

import (
	"context"
	"fmt"
	"time"

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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controller/gardenletdeployer"
	"github.com/gardener/gardener/pkg/controllerutils"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/oci"
)

// RequeueDurationSeedIsNotYetRegistered is the duration after which the Seed registration is checked
// when gardenlet was just deployed. Exposed for testing.
var RequeueDurationSeedIsNotYetRegistered = 30 * time.Second

// Reconciler reconciles the Gardenlet.
type Reconciler struct {
	RuntimeCluster        cluster.Cluster
	VirtualConfig         *rest.Config
	VirtualClient         client.Client
	Config                operatorconfigv1alpha1.GardenletDeployerControllerConfig
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

	// Deletion is not implemented - once gardenlet got deployed by gardener-operator, it doesn't care about it ever
	// again (unless a forceful re-deployment is requested by the user). Gardenlet will perform self-upgrades. Seed
	// deprovisioning must be handled by human operators.
	return r.reconcile(ctx, log, gardenlet, r.newActuator(gardenlet))
}

func (r *Reconciler) newActuator(gardenlet *seedmanagementv1alpha1.Gardenlet) gardenletdeployer.Interface {
	return &gardenletdeployer.Actuator{
		GardenConfig: r.VirtualConfig,
		GardenClient: r.VirtualClient,
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
				r.VirtualClient,
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
			// TODO(vpnachev): Add support for WorkloadIdentity
			return kubernetesutils.GetSecretByObjectReference(ctx, r.VirtualClient, seedTemplate.Spec.Backup.CredentialsRef)
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
	result reconcile.Result,
	err error,
) {
	status := gardenlet.Status.DeepCopy()
	status.ObservedGeneration = gardenlet.Generation

	log.V(1).Info("Reconciling")
	if !hasForceRedeployOperationAnnotation(gardenlet) && !r.seedDoesNotExist(ctx, gardenlet) {
		if err := r.cleanupKubeconfigSecret(ctx, log, gardenlet); err != nil {
			status.Conditions = updateCondition(r.Clock, status.Conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconcileError, err.Error())
		} else {
			log.Info("Gardenlet deployment handling completed successfully")
			status.Conditions = updateCondition(r.Clock, status.Conditions, gardencorev1beta1.ConditionTrue, gardencorev1beta1.EventReconciled, fmt.Sprintf("Gardenlet deployed and Seed %q registered", gardenlet.Name))
		}

		return result, r.updateStatus(ctx, gardenlet, status)
	}

	status.Conditions, err = actuator.Reconcile(ctx, log, gardenlet, status.Conditions, &gardenlet.Spec.Deployment.GardenletDeployment, &gardenlet.Spec.Config, seedmanagementv1alpha1.BootstrapToken, false)
	if err != nil {
		if updateErr := r.updateStatus(ctx, gardenlet, status); updateErr != nil {
			log.Error(updateErr, "Could not update status", "status", status)
		}
		return result, fmt.Errorf("could not reconcile Gardenlet %s creation: %w", client.ObjectKeyFromObject(gardenlet), err)
	}

	status.Conditions = updateCondition(r.Clock, status.Conditions, gardencorev1beta1.ConditionProgressing, gardencorev1beta1.EventReconcileError, "Waiting for seed registration")
	if err := r.updateStatus(ctx, gardenlet, status); err != nil {
		return result, fmt.Errorf("failed updating Gardenlet status after successful deployment: %w", err)
	}

	log.Info("Gardenlet deployment finished successfully. Request is requeued to check seed registration")
	return reconcile.Result{RequeueAfter: RequeueDurationSeedIsNotYetRegistered}, r.removeForceRedeployAnnotationIfNeeded(ctx, log, gardenlet)
}

func updateCondition(clock clock.Clock, conditions []gardencorev1beta1.Condition, cs gardencorev1beta1.ConditionStatus, reason, message string) []gardencorev1beta1.Condition {
	condition := v1beta1helper.GetOrInitConditionWithClock(clock, conditions, seedmanagementv1alpha1.SeedRegistered)
	condition = v1beta1helper.UpdatedConditionWithClock(clock, condition, cs, reason, message)
	return v1beta1helper.MergeConditions(conditions, condition)
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

func (r *Reconciler) removeForceRedeployAnnotationIfNeeded(ctx context.Context, log logr.Logger, gardenlet *seedmanagementv1alpha1.Gardenlet) error {
	if !hasForceRedeployOperationAnnotation(gardenlet) {
		return nil
	}

	log.Info("Removing force-redeploy operation annotation")

	patch := client.MergeFrom(gardenlet.DeepCopy())
	delete(gardenlet.Annotations, v1beta1constants.GardenerOperation)
	if err := r.VirtualClient.Patch(ctx, gardenlet, patch); err != nil {
		return fmt.Errorf("could not remove force-redeploy operation annotation: %w", err)
	}

	return nil
}
