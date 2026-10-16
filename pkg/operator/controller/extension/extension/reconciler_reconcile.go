// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	gardenerapiserver "github.com/gardener/gardener/pkg/component/gardener/apiserver"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/gardener/operator"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// RequeueGardenResourceNotReady is the time after which an extension will be requeued, if the Garden resource was not ready during its reconciliation. Exposed for testing.
var RequeueGardenResourceNotReady = 10 * time.Second

// RequeueGardenAPIServerNotAvailable is the time after which an extension will be requeued, if the garden API servers
// were not yet available for injecting the garden kubeconfig during its reconciliation. Exposed for testing.
var RequeueGardenAPIServerNotAvailable = 2 * time.Second

func (r *Reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	garden *gardenInfo,
	extension *operatorv1alpha1.Extension,
) (
	reconcile.Result,
	error,
) {
	conditions := NewConditions(r.Clock, extension.Status)

	if garden.garden == nil {
		if conditions.installed.Status == gardencorev1beta1.ConditionFalse && conditions.installed.Reason == ReasonDeleteSuccessful {
			// Retain condition if the extension was previously uninstalled due to a garden deletion
			return reconcile.Result{}, nil
		}

		conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionFalse, ReasonNoGardenFound, "No garden found")
		return reconcile.Result{}, r.updateExtensionStatus(ctx, log, extension, conditions)
	}

	if !controllerutil.ContainsFinalizer(extension, operatorv1alpha1.FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.RuntimeClientSet.Client(), extension, operatorv1alpha1.FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	apiServerDeploymentsFound, err := r.gardenAPIServersAvailable(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to check if the garden API servers are available: %w", err)
	}

	var (
		virtualClusterClientSet kubernetes.Interface
		reconcileResult         reconcile.Result
		g                       = flow.NewGraph("Extension reconciliation")

		createVirtualGardenClientSet = g.Add(flow.Task{
			Name: "Creating virtual garden-client",
			Fn: func(ctx context.Context) error {
				clientSet, err := r.GardenClientMap.GetClient(ctx, keys.ForGarden(garden.garden))
				if err != nil {
					return fmt.Errorf("error retrieving virtual cluster client set: %w", err)
				}

				virtualClusterClientSet = clientSet
				return nil
			},
			SkipIf: !apiServerDeploymentsFound,
		})

		deployExtensionInRuntime = g.Add(flow.Task{
			Name: "Deploying extension in runtime cluster",
			Fn: func(ctx context.Context) error {
				var err error
				reconcileResult, err = r.deployExtensionInRuntime(ctx, log, virtualClusterClientSet, extension)
				return err
			},
			Dependencies: flow.NewTaskIDs(createVirtualGardenClientSet),
		})

		checkGarden = g.Add(flow.Task{
			Name: "Checking if garden is reconciled and up-to-date",
			Fn: func(_ context.Context) error {
				if !garden.reconciled || !garden.upToDate {
					log.Info("Garden is not yet in 'Reconcile Succeeded' state or not up-to-date, re-queueing", "requeueAfter", RequeueGardenResourceNotReady)
					reconcileResult = reconcile.Result{RequeueAfter: RequeueGardenResourceNotReady}
					return fmt.Errorf("garden is not yet successfully reconciled or not up-to-date")
				}
				return nil
			},
			Dependencies: flow.NewTaskIDs(deployExtensionInRuntime),
		})

		_ = g.Add(flow.Task{
			Name: "Deploying Admission Controller",
			Fn: func(ctx context.Context) error {
				if virtualClusterClientSet == nil && !apiServerDeploymentsFound {
					log.Info("Virtual cluster client not yet available")
					reconcileResult = reconcile.Result{RequeueAfter: RequeueGardenAPIServerNotAvailable}
					return nil
				}
				if garden.genericTokenKubeconfigSecretName == nil {
					return fmt.Errorf("generic kubeconfig secret name is not set for garden")
				}
				return r.admission.Reconcile(ctx, log, virtualClusterClientSet, *garden.genericTokenKubeconfigSecretName, extension)
			},
			Dependencies: flow.NewTaskIDs(createVirtualGardenClientSet),
		})

		_ = g.Add(flow.Task{
			Name: "Deploying ControllerRegistration and ControllerDeployment",
			Fn: func(ctx context.Context) error {
				return r.controllerRegistration.Reconcile(ctx, log, extension)
			},
			Dependencies: flow.NewTaskIDs(checkGarden),
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log: log,
	}); err != nil {
		conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionFalse, ReasonReconcileFailed, err.Error())
		if updateErr := r.updateExtensionStatus(ctx, log, extension, conditions); updateErr != nil {
			return reconcile.Result{}, errors.Join(err, fmt.Errorf("failed to update extension status: %w", updateErr))
		}
		if !reflect.DeepEqual(reconcileResult, reconcile.Result{}) {
			return reconcileResult, nil
		}
		return reconcile.Result{}, err
	}

	conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionTrue, ReasonReconcileSuccess, "Extension has been reconciled successfully")
	return reconcileResult, r.updateExtensionStatus(ctx, log, extension, conditions)
}

func (r *Reconciler) deployExtensionInRuntime(ctx context.Context, log logr.Logger, virtualClusterClientSet kubernetes.Interface, extension *operatorv1alpha1.Extension) (reconcile.Result, error) {
	if !operator.IsExtensionInRuntimeRequired(extension) {
		log.V(1).Info("Deployment in runtime cluster not required")
		return r.runtime.Delete(ctx, log, extension)
	}
	log.V(1).Info("Deployment in runtime cluster required")

	return r.runtime.Reconcile(ctx, log, virtualClusterClientSet, extension)
}

// gardenAPIServersAvailable reports whether both the virtual garden kube-apiserver and the Gardener API server
// Deployments are available in the garden namespace of the runtime cluster.
func (r *Reconciler) gardenAPIServersAvailable(ctx context.Context) (bool, error) {
	for _, name := range []string{
		operatorv1alpha1.DeploymentNameVirtualGardenKubeAPIServer,
		gardenerapiserver.DeploymentName,
	} {
		deployment := &appsv1.Deployment{}
		if err := r.RuntimeClientSet.Client().Get(ctx, client.ObjectKey{Namespace: r.GardenNamespace, Name: name}, deployment); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, fmt.Errorf("failed getting deployment %q: %w", name, err)
		}
		if err := health.CheckDeployment(deployment); err != nil {
			return false, fmt.Errorf("deployment %q is not healthy: %w", name, err)
		}
	}
	return true, nil
}
