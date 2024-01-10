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

package vpaevictionrequirements

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

var cpuAndMemory = []corev1.ResourceName{corev1.ResourceMemory, corev1.ResourceCPU}
var upscaleOnlyRequirement = []*vpaautoscalingv1.EvictionRequirement{
	{
		Resources:         cpuAndMemory,
		ChangeRequirement: vpaautoscalingv1.TargetHigherThanRequests,
	},
}

func removeAllEvictionRequirements(vpa *vpaautoscalingv1.VerticalPodAutoscaler) {
	vpa.Spec.UpdatePolicy.EvictionRequirements = []*vpaautoscalingv1.EvictionRequirement{}
}

func addDenyDownscalingEvictionRequirement(vpa *vpaautoscalingv1.VerticalPodAutoscaler) {
	vpa.Spec.UpdatePolicy.EvictionRequirements = upscaleOnlyRequirement
}

// Reconciler implements the reconciliation logic for adding/removing EvictionRequirements to VPA objects.
type Reconciler struct {
	Config     config.VPAEvictionRequirementsControllerConfiguration
	SeedClient client.Client
	Clock      clock.Clock
}

// Reconcile implements the reconciliation logic for adding/removing EvictionRequirements to VPA objects.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)
	seedCtx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	vpa := &vpaautoscalingv1.VerticalPodAutoscaler{}
	err := r.SeedClient.Get(seedCtx, request.NamespacedName, vpa)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	log.Info("Reconciling vpa")

	// double check just for fun: does the vpa have our label?
	if metav1.HasLabel(vpa.ObjectMeta, constants.LabelVPAEvictionRequirementDownscaleRestriction) {
		value := vpa.GetLabels()[constants.LabelVPAEvictionRequirementDownscaleRestriction]
		log.Info("Found the label "+constants.LabelVPAEvictionRequirementDownscaleRestriction, "value", value)
		switch value {
		case constants.Never:
			return r.reconcileVPAForDownscaleDisabled(ctx, vpa)
		case constants.InMaintenanceWindowOnly:
			return r.reconcileVPAForDownscaleInMaintenanceOnly(ctx, vpa)
		default:
			err := fmt.Errorf("unsupported label value found: %q, supported are only %q and %q", value, constants.Never, constants.InMaintenanceWindowOnly)
			log.Error(err, "Error while parsing the label value:")
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) reconcileVPAForDownscaleInMaintenanceOnly(ctx context.Context, vpa *vpaautoscalingv1.VerticalPodAutoscaler) (reconcile.Result, error) {
	log := logf.FromContext(ctx)
	if !metav1.HasAnnotation(vpa.ObjectMeta, constants.AnnotationShootMaintenanceWindow) {
		err := fmt.Errorf("didn't find maintenance window annotation, but VPA had label to be downscaled in maintenance only")
		log.Error(err, "Error during reconciling for downscaling in maintenance only:")
		return reconcile.Result{}, err
	}

	windowAnnotation := vpa.GetAnnotations()[constants.AnnotationShootMaintenanceWindow]
	splitWindowAnnotation := strings.Split(windowAnnotation, ",")
	if len(splitWindowAnnotation) != 2 {
		err := fmt.Errorf("error during parsing the maintenance window from annotation. Value is not in format '<begin>,<end>': %q", windowAnnotation)
		log.Error(err, "Error during reconciling for downscaling in maintenance only:")
		return reconcile.Result{}, err
	}
	maintenanceTimeWindow, err := timewindow.ParseMaintenanceTimeWindow(splitWindowAnnotation[0], splitWindowAnnotation[1])
	if err != nil {
		log.Error(err, "Error during parsing the maintenance window from start and end time", "begin", splitWindowAnnotation[0], "end", splitWindowAnnotation[1])
		return reconcile.Result{}, err
	}

	if isNowInMaintenanceTimeWindow := maintenanceTimeWindow.Contains(r.Clock.Now()); isNowInMaintenanceTimeWindow {
		log.Info("Shoot is inside maintenance window, removing the EvictionRequirement to allow downscaling", "shoot-namespace", vpa.GetNamespace(), "maintenanceWindow", maintenanceTimeWindow)

		if err := r.patchVPA(ctx, vpa, removeAllEvictionRequirements); err != nil {
			return reconcile.Result{}, err
		}

		// requeue when the maintenance window ends, such that we can add the EvictionRequirement again
		endTime := maintenanceTimeWindow.AdjustedEnd(r.Clock.Now())
		requeueAfter := endTime.Sub(r.Clock.Now())
		log.Info("Requeueing VPA", "namespace", vpa.Namespace, "name", vpa.Name, "requeueAfter", requeueAfter)
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	} else {
		log.Info("Shoot is not inside maintenance window, adding EvictionRequirement to deny downscaling", "shoot-namespace", vpa.GetNamespace(), "maintenanceWindow", maintenanceTimeWindow)

		if err := r.patchVPA(ctx, vpa, addDenyDownscalingEvictionRequirement); err != nil {
			return reconcile.Result{}, err
		}

		// requeue when the next maintenance window begins, such that we can remove the EvictionRequirement
		nextWindowBegin := maintenanceTimeWindow.AdjustedBegin(r.Clock.Now())
		if nextWindowBegin.Before(r.Clock.Now()) {
			nextWindowBegin = nextWindowBegin.AddDate(0, 0, 1)
		}
		requeueAfter := nextWindowBegin.Sub(r.Clock.Now())
		log.Info("Requeueing VPA", "requeueAfter", requeueAfter)
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}
}

func (r *Reconciler) reconcileVPAForDownscaleDisabled(ctx context.Context, vpa *vpaautoscalingv1.VerticalPodAutoscaler) (reconcile.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Adding EvictionRequirement for vpa to deny downscaling")

	existing := vpa.DeepCopyObject()

	addDenyDownscalingEvictionRequirement(vpa)

	if equality.Semantic.DeepEqual(existing, vpa) {
		return reconcile.Result{}, nil
	}

	if err := r.SeedClient.Update(ctx, vpa); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Not requeueing VPA")
	return reconcile.Result{}, nil
}

func (r *Reconciler) patchVPA(ctx context.Context, vpa *vpaautoscalingv1.VerticalPodAutoscaler, patchFn func(autoscaler *vpaautoscalingv1.VerticalPodAutoscaler)) error {
	existing := vpa.DeepCopyObject()

	patchFn(vpa)

	if equality.Semantic.DeepEqual(existing, vpa) {
		return nil
	}

	return r.SeedClient.Update(ctx, vpa)
}
