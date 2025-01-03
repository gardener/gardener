// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package progressing

import (
	"context"
	"fmt"

	certv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/health/utils"
	resourcemanagerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// Reconciler performs progressing checks for resources managed as part of ManagedResources.
type Reconciler struct {
	SourceClient client.Client
	TargetClient client.Client
	Config       resourcemanagerconfigv1alpha1.HealthControllerConfig
	Clock        clock.Clock
	ClassFilter  *resourcemanagerpredicate.ClassFilter
}

// Reconcile performs the progressing checks.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	// timeout for all calls (e.g. status updates), give status updates a bit of headroom if checks
	// themselves run into timeouts, so that we will still update the status with that timeout error
	var cancel context.CancelFunc
	ctx, cancel = controllerutils.GetMainReconciliationContext(ctx, r.Config.SyncPeriod.Duration)
	defer cancel()

	mr := &resourcesv1alpha1.ManagedResource{}
	if err := r.SourceClient.Get(ctx, req.NamespacedName, mr); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if utils.IsIgnored(mr) {
		log.Info("Skipping checks since ManagedResource is ignored")
		return reconcile.Result{}, nil
	}

	// Check responsibility
	if responsible := r.ClassFilter.Responsible(mr); !responsible {
		log.Info("Stopping checks as the responsibility changed")
		return reconcile.Result{}, nil
	}

	if !mr.DeletionTimestamp.IsZero() {
		log.Info("Stopping checks for ManagedResource as it is marked for deletion")
		return reconcile.Result{}, nil
	}

	// skip checks until ManagedResource has been reconciled completely successfully to prevent updating status while
	// resource controller is still applying the resources (this might lead to wrongful results inconsistent with the
	// actual set of applied resources and causes a myriad of conflicts)
	conditionResourcesApplied := v1beta1helper.GetCondition(mr.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
	if conditionResourcesApplied == nil || conditionResourcesApplied.Status == gardencorev1beta1.ConditionProgressing || conditionResourcesApplied.Status == gardencorev1beta1.ConditionFalse {
		log.Info("Skipping checks for ManagedResource as the resources were not applied yet")
		return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
	}

	return r.reconcile(ctx, log, mr)
}

func (r *Reconciler) reconcile(ctx context.Context, log logr.Logger, mr *resourcesv1alpha1.ManagedResource) (reconcile.Result, error) {
	log.V(1).Info("Starting ManagedResource progressing checks")
	// don't block workers if calls timeout for some reason
	checkCtx, cancel := controllerutils.GetChildReconciliationContext(ctx, r.Config.SyncPeriod.Duration)
	defer cancel()

	conditionResourcesProgressing := v1beta1helper.GetOrInitConditionWithClock(r.Clock, mr.Status.Conditions, resourcesv1alpha1.ResourcesProgressing)

	for _, ref := range mr.Status.Resources {
		// Skip API groups that are irrelevant for progressing checks.
		if !sets.New(appsv1.GroupName, monitoring.GroupName, certv1alpha1.GroupName).Has(ref.GroupVersionKind().Group) {
			continue
		}

		var obj client.Object

		switch ref.Kind {
		case "Deployment":
			obj = &appsv1.Deployment{}
		case "StatefulSet":
			obj = &appsv1.StatefulSet{}
		case "DaemonSet":
			obj = &appsv1.DaemonSet{}
		case "Prometheus":
			obj = &monitoringv1.Prometheus{}
		case "Alertmanager":
			obj = &monitoringv1.Alertmanager{}
		case "Certificate":
			obj = &certv1alpha1.Certificate{}
		case "Issuer":
			obj = &certv1alpha1.Issuer{}
		default:
			continue
		}

		var (
			objectKey = client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}
			objectLog = log.WithValues("object", objectKey, "objectGVK", ref.GroupVersionKind())
		)

		if err := r.TargetClient.Get(checkCtx, objectKey, obj); err != nil {
			if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
				// missing objects already handled by health controller, skip
				continue
			}
			return reconcile.Result{}, err
		}

		if progressing, description, err := r.checkProgressing(ctx, obj); err != nil {
			return reconcile.Result{}, err
		} else if progressing {
			var (
				reason  = ref.Kind + "Progressing"
				message = fmt.Sprintf("%s %q is progressing: %s", ref.Kind, objectKey.String(), description)
			)

			objectLog.Info("ManagedResource rollout is progressing, detected progressing object", "status", "progressing", "reason", reason, "message", message)

			conditionResourcesProgressing = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesProgressing, gardencorev1beta1.ConditionTrue, reason, message)
			mr.Status.Conditions = v1beta1helper.MergeConditions(mr.Status.Conditions, conditionResourcesProgressing)
			if err := r.SourceClient.Status().Update(ctx, mr); err != nil {
				return reconcile.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
			}

			return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
		}
	}

	b, err := v1beta1helper.NewConditionBuilder(resourcesv1alpha1.ResourcesProgressing)
	if err != nil {
		return reconcile.Result{}, err
	}

	var needsUpdate bool
	conditionResourcesProgressing, needsUpdate = b.WithOldCondition(conditionResourcesProgressing).
		WithStatus(gardencorev1beta1.ConditionFalse).WithReason("ResourcesRolledOut").
		WithMessage("All resources have been fully rolled out.").
		Build()

	if needsUpdate {
		log.Info("ManagedResource has been fully rolled out", "status", "rolled out")
		mr.Status.Conditions = v1beta1helper.MergeConditions(mr.Status.Conditions, conditionResourcesProgressing)
		if err := r.SourceClient.Status().Update(ctx, mr); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
		}
	}

	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}

// checkProgressing checks whether the given object is progressing. It returns a bool indicating whether the object is
// progressing, a reason for it if so and an error if the check failed.
func (r *Reconciler) checkProgressing(ctx context.Context, obj client.Object) (bool, string, error) {
	if obj.GetAnnotations()[resourcesv1alpha1.SkipHealthCheck] == "true" {
		return false, "", nil
	}

	var (
		progressing bool
		reason      string
	)

	switch o := obj.(type) {
	case *appsv1.Deployment:
		progressing, reason = health.IsDeploymentProgressing(o)
		if progressing {
			return true, reason, nil
		}

		// health.IsDeploymentProgressing might return false even if there are still (terminating) pods in the system
		// belonging to an older ReplicaSet of the Deployment, hence, we have to check for this explicitly.
		exactNumberOfPods, err := health.DeploymentHasExactNumberOfPods(ctx, r.TargetClient, o)
		if err != nil {
			return progressing, reason, err
		}
		if !exactNumberOfPods {
			return true, "there are still non-terminated old pods", nil
		}

	case *appsv1.StatefulSet:
		progressing, reason = health.IsStatefulSetProgressing(o)

	case *appsv1.DaemonSet:
		progressing, reason = health.IsDaemonSetProgressing(o)

	case *monitoringv1.Prometheus:
		progressing, reason = health.IsPrometheusProgressing(o)

	case *monitoringv1.Alertmanager:
		progressing, reason = health.IsAlertmanagerProgressing(o)

	case *certv1alpha1.Certificate:
		progressing, reason = health.IsCertificateProgressing(o)

	case *certv1alpha1.Issuer:
		progressing, reason = health.IsCertificateIssuerProgressing(o)
	}

	return progressing, reason, nil
}
