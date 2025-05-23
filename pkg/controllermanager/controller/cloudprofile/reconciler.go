// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprofile

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// Reconciler reconciles CloudProfiles.
type Reconciler struct {
	Client   client.Client
	Config   controllermanagerconfigv1alpha1.CloudProfileControllerConfiguration
	Recorder record.EventRecorder
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	cloudProfile := &gardencorev1beta1.CloudProfile{}
	if err := r.Client.Get(ctx, request.NamespacedName, cloudProfile); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// The deletionTimestamp labels the CloudProfile as intended to get deleted. Before deletion, it has to be ensured that
	// no Shoots, Seeds and other NamespacedCloudProfiles are assigned to the CloudProfile anymore.
	// If this is the case then the controller will remove the finalizers from the CloudProfile so that it can be garbage collected.
	if cloudProfile.DeletionTimestamp != nil {
		if !sets.New(cloudProfile.Finalizers...).Has(gardencorev1beta1.GardenerName) {
			return reconcile.Result{}, nil
		}

		namespacedCloudProfileList, err := controllerutils.GetNamespacedCloudProfilesReferencingCloudProfile(ctx, r.Client, cloudProfile.Name)
		if err != nil {
			return reconcile.Result{}, err
		}
		if len(namespacedCloudProfileList.Items) != 0 {
			var associatedNamespacedCloudProfiles []string
			for _, namespacedCloudProfile := range namespacedCloudProfileList.Items {
				associatedNamespacedCloudProfiles = append(associatedNamespacedCloudProfiles, fmt.Sprintf("%s/%s", namespacedCloudProfile.Namespace, namespacedCloudProfile.Name))
			}
			message := fmt.Sprintf("Cannot delete CloudProfile, because the following NamespacedCloudProfiles are still referencing it: %+v", associatedNamespacedCloudProfiles)
			r.Recorder.Event(cloudProfile, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, message)
			return reconcile.Result{}, errors.New(message)
		}
		log.Info("No NamespacedCloudProfiles are referencing the CloudProfile")

		associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(ctx, r.Client, cloudProfile)
		if err != nil {
			return reconcile.Result{}, err
		}

		if len(associatedShoots) == 0 {
			log.Info("No Shoots are referencing the CloudProfile, deletion accepted")

			if controllerutil.ContainsFinalizer(cloudProfile, gardencorev1beta1.GardenerName) {
				log.Info("Removing finalizer")
				if err := controllerutils.RemoveFinalizers(ctx, r.Client, cloudProfile, gardencorev1beta1.GardenerName); err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
				}
			}

			return reconcile.Result{}, nil
		}

		message := fmt.Sprintf("Cannot delete CloudProfile, because the following Shoots are still referencing it: %+v", associatedShoots)
		r.Recorder.Event(cloudProfile, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, message)
		return reconcile.Result{}, errors.New(message)
	}

	if !controllerutil.ContainsFinalizer(cloudProfile, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, cloudProfile, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}
