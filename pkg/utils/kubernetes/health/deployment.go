// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"context"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/retry"
)

func getDeploymentCondition(conditions []appsv1.DeploymentCondition, conditionType appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

var (
	trueDeploymentConditionTypes = []appsv1.DeploymentConditionType{
		appsv1.DeploymentAvailable,
	}

	trueOptionalDeploymentConditionTypes = []appsv1.DeploymentConditionType{
		appsv1.DeploymentProgressing,
	}

	falseOptionalDeploymentConditionTypes = []appsv1.DeploymentConditionType{
		appsv1.DeploymentReplicaFailure,
	}
)

// CheckDeployment checks whether the given Deployment is healthy.
// A deployment is considered healthy if the controller observed its current revision and
// if the number of updated replicas is equal to the number of replicas.
func CheckDeployment(deployment *appsv1.Deployment) error {
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", deployment.Status.ObservedGeneration, deployment.Generation)
	}

	for _, trueConditionType := range trueDeploymentConditionTypes {
		conditionType := string(trueConditionType)
		condition := getDeploymentCondition(deployment.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(conditionType)
		}
		if err := checkConditionState(string(condition.Type), string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, trueOptionalConditionType := range trueOptionalDeploymentConditionTypes {
		condition := getDeploymentCondition(deployment.Status.Conditions, trueOptionalConditionType)
		if condition == nil {
			continue
		}
		if err := checkConditionState(string(condition.Type), string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, falseOptionalConditionType := range falseOptionalDeploymentConditionTypes {
		condition := getDeploymentCondition(deployment.Status.Conditions, falseOptionalConditionType)
		if condition == nil {
			continue
		}
		if err := checkConditionState(string(condition.Type), string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}

// IsDeploymentProgressing returns false if the Deployment has been fully rolled out. Otherwise, it returns true along
// with a reason, why the Deployment is not considered to be fully rolled out.
func IsDeploymentProgressing(deployment *appsv1.Deployment) (bool, string) {
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return true, fmt.Sprintf("observed generation outdated (%d/%d)", deployment.Status.ObservedGeneration, deployment.Generation)
	}

	// if the observed generation is up-to-date, we can rely on the progressing condition to reflect the current status
	condition := getDeploymentCondition(deployment.Status.Conditions, appsv1.DeploymentProgressing)
	if condition == nil {
		return true, fmt.Sprintf("condition %q is missing", appsv1.DeploymentProgressing)
	}

	if condition.Status != corev1.ConditionTrue || condition.Reason != "NewReplicaSetAvailable" {
		// only if Progressing is in status True with reason NewReplicaSetAvailable, the Deployment has been fully rolled out
		// note: old pods or excess pods (scale-down) might still be terminating, but there is no way to tell this from the
		// Deployment's status, see https://github.com/kubernetes/kubernetes/issues/110171
		return true, condition.Message
	}

	return false, "Deployment is fully rolled out"
}

// IsDeploymentUpdated returns a function which can be used for retry.Until. It checks if the deployment is fully
// updated, i.e. if it is no longer progressing, healthy, and has the exact number of desired replicas.
func IsDeploymentUpdated(reader client.Reader, deployment *appsv1.Deployment) func(context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		if err := reader.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
			return retry.SevereError(err)
		}

		// Check if Deployment is still progressing.
		if progressing, reason := IsDeploymentProgressing(deployment); progressing {
			return retry.MinorError(errors.New(reason))
		}

		// If Deployment is no longer progressing then check if it is healthy.
		if err := CheckDeployment(deployment); err != nil {
			return retry.MinorError(err)
		}

		// Now there might be still pods in the system belonging to an older ReplicaSet of the Deployment.
		exactNumberOfPods, err := DeploymentHasExactNumberOfPods(ctx, reader, deployment)
		if err != nil {
			return retry.SevereError(err)
		}
		if !exactNumberOfPods {
			return retry.MinorError(errors.New("there are still non-terminated old pods"))
		}

		return retry.Ok()
	}
}

// DeploymentHasExactNumberOfPods returns true when there are exactly as many pods as the .spec.replicas field of the
// deployment mandates.
func DeploymentHasExactNumberOfPods(ctx context.Context, reader client.Reader, deployment *appsv1.Deployment) (bool, error) {
	podList := &corev1.PodList{}
	if err := reader.List(ctx, podList, client.InNamespace(deployment.Namespace), client.MatchingLabels(deployment.Spec.Selector.MatchLabels)); err != nil {
		return false, err
	}

	var numberOfRelevantPods int32
	for _, pod := range podList.Items {
		if !IsPodStale(pod.Status.Reason) && !IsPodCompleted(pod.Status.Conditions) {
			numberOfRelevantPods++
		}
	}

	return numberOfRelevantPods == ptr.Deref(deployment.Spec.Replicas, 1), nil
}
