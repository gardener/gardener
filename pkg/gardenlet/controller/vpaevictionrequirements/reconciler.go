// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

var upscaleOnlyRequirement = []*vpaautoscalingv1.EvictionRequirement{{
	Resources:         []corev1.ResourceName{corev1.ResourceMemory, corev1.ResourceCPU},
	ChangeRequirement: vpaautoscalingv1.TargetHigherThanRequests,
}}

// Reconciler implements the reconciliation logic for adding/removing EvictionRequirements to VPA objects.
type Reconciler struct {
	Config     config.VPAEvictionRequirementsControllerConfiguration
	SeedClient client.Client
	Clock      clock.Clock
}

// Reconcile implements the reconciliation logic for adding/removing EvictionRequirements to VPA objects.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)
	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	vpa := &vpaautoscalingv1.VerticalPodAutoscaler{}
	if err := r.SeedClient.Get(ctx, request.NamespacedName, vpa); err != nil {
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	log.Info("Reconciling")
	if !metav1.HasLabel(vpa.ObjectMeta, constants.LabelVPAEvictionRequirementDownscaleRestriction) {
		err := fmt.Errorf("label %q not found, although marker label %q is present", constants.LabelVPAEvictionRequirementDownscaleRestriction, constants.LabelVPAEvictionRequirementsController)
		log.Error(err, "Error while parsing the label value:")
		// No need to retry reconciling this VPA until it has been updated with the label, therefore not returning the error
		return reconcile.Result{}, nil
	}

	value := vpa.GetLabels()[constants.LabelVPAEvictionRequirementDownscaleRestriction]
	log.Info("Found the label "+constants.LabelVPAEvictionRequirementDownscaleRestriction, "value", value)
	switch value {
	case constants.EvictionRequirementNever:
		return r.reconcileVPAForDownscaleDisabled(ctx, log, vpa)
	case constants.EvictionRequirementInMaintenanceWindowOnly:
		return r.reconcileVPAForDownscaleInMaintenanceOnly(ctx, log, vpa)
	default:
		err := fmt.Errorf("unsupported label value found: %q, supported are only %q and %q", value, constants.EvictionRequirementNever, constants.EvictionRequirementInMaintenanceWindowOnly)
		log.Error(err, "Error while parsing the label value")
		// No need to retry reconciling this VPA until it has been updated with the label, therefore not returning the error
		return reconcile.Result{}, nil
	}
}

func (r *Reconciler) reconcileVPAForDownscaleInMaintenanceOnly(ctx context.Context, log logr.Logger, vpa *vpaautoscalingv1.VerticalPodAutoscaler) (reconcile.Result, error) {
	if !metav1.HasAnnotation(vpa.ObjectMeta, constants.AnnotationShootMaintenanceWindow) {
		err := fmt.Errorf("didn't find maintenance window annotation, but VPA had label to be downscaled in maintenance only")
		log.Error(err, "Error during reconciling for downscaling in maintenance only")
		// No need to retry reconciling this VPA until it has been updated with the annotation, therefore not returning the error
		return reconcile.Result{}, nil
	}

	windowAnnotation := vpa.GetAnnotations()[constants.AnnotationShootMaintenanceWindow]
	splitWindowAnnotation := strings.Split(windowAnnotation, ",")
	if len(splitWindowAnnotation) != 2 {
		err := fmt.Errorf("error during parsing the maintenance window from annotation. Value is not in format '<begin>,<end>': %q", windowAnnotation)
		log.Error(err, "Error during reconciling for downscaling in maintenance only")
		// No need to retry reconciling this VPA until it has been updated with a fixed annotation, therefore not returning the error
		return reconcile.Result{}, nil
	}

	maintenanceTimeWindow, err := timewindow.ParseMaintenanceTimeWindow(splitWindowAnnotation[0], splitWindowAnnotation[1])
	if err != nil {
		log.Error(err, "Error during parsing the maintenance window from start and end time", "begin", splitWindowAnnotation[0], "end", splitWindowAnnotation[1])
		// No need to retry reconciling this VPA until it has been updated with a fixed annotation, therefore not returning the error
		return reconcile.Result{}, nil
	}

	if isNowInMaintenanceTimeWindow := maintenanceTimeWindow.Contains(r.Clock.Now()); isNowInMaintenanceTimeWindow {
		log.Info("Shoot is inside maintenance window, removing the EvictionRequirement to allow downscaling", "maintenanceWindow", maintenanceTimeWindow)

		if _, err := controllerutil.CreateOrUpdate(ctx, r.SeedClient, vpa, func() error {
			vpa.Spec.UpdatePolicy.EvictionRequirements = nil
			return nil
		}); err != nil {
			return reconcile.Result{}, err
		}

		// requeue when the maintenance window ends, such that we can add the EvictionRequirement again
		endTime := maintenanceTimeWindow.AdjustedEnd(r.Clock.Now())
		requeueAfter := endTime.Sub(r.Clock.Now())
		log.Info("Requeueing to the end of the maintenance window", "requeueAfter", requeueAfter)
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}

	log.Info("Shoot is not inside maintenance window, adding EvictionRequirement to deny downscaling", "maintenanceWindow", maintenanceTimeWindow)

	if _, err = controllerutil.CreateOrUpdate(ctx, r.SeedClient, vpa, func() error {
		vpa.Spec.UpdatePolicy.EvictionRequirements = upscaleOnlyRequirement
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// requeue when the next maintenance window begins, such that we can remove the EvictionRequirement
	nextWindowBegin := maintenanceTimeWindow.AdjustedBegin(r.Clock.Now())
	if nextWindowBegin.Before(r.Clock.Now()) {
		nextWindowBegin = nextWindowBegin.AddDate(0, 0, 1)
	}
	requeueAfter := nextWindowBegin.Sub(r.Clock.Now())
	log.Info("Requeueing to the begin of the next maintenance window", "requeueAfter", requeueAfter)
	return reconcile.Result{RequeueAfter: requeueAfter}, nil
}

func (r *Reconciler) reconcileVPAForDownscaleDisabled(ctx context.Context, log logr.Logger, vpa *vpaautoscalingv1.VerticalPodAutoscaler) (reconcile.Result, error) {
	log.Info("Adding EvictionRequirement for vpa to deny downscaling")

	if _, err := controllerutil.CreateOrUpdate(ctx, r.SeedClient, vpa, func() error {
		vpa.Spec.UpdatePolicy.EvictionRequirements = upscaleOnlyRequirement
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Not requeueing VPA")
	return reconcile.Result{}, nil
}
