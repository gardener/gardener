// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/retry"
)

func requiredConditionMissing(conditionType string) error {
	return fmt.Errorf("condition %q is missing", conditionType)
}

func checkConditionState(conditionType string, expected, actual, reason, message string) error {
	if expected != actual {
		return fmt.Errorf("condition %q has invalid status %s (expected %s) due to %s: %s",
			conditionType, actual, expected, reason, message)
	}
	return nil
}

func getDeploymentCondition(conditions []appsv1.DeploymentCondition, conditionType appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

func getNodeCondition(conditions []corev1.NodeCondition, conditionType corev1.NodeConditionType) *corev1.NodeCondition {
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
		if err := checkConditionState(conditionType, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, trueOptionalConditionType := range trueOptionalDeploymentConditionTypes {
		conditionType := string(trueOptionalConditionType)
		condition := getDeploymentCondition(deployment.Status.Conditions, trueOptionalConditionType)
		if condition == nil {
			continue
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, falseOptionalConditionType := range falseOptionalDeploymentConditionTypes {
		conditionType := string(falseOptionalConditionType)
		condition := getDeploymentCondition(deployment.Status.Conditions, falseOptionalConditionType)
		if condition == nil {
			continue
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}

// CheckStatefulSet checks whether the given StatefulSet is healthy.
// A StatefulSet is considered healthy if its controller observed its current revision,
// it is not in an update (i.e. UpdateRevision is empty) and if its current replicas are equal to
// its desired replicas.
func CheckStatefulSet(statefulSet *appsv1.StatefulSet) error {
	if statefulSet.Status.ObservedGeneration < statefulSet.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", statefulSet.Status.ObservedGeneration, statefulSet.Generation)
	}

	replicas := int32(1)
	if statefulSet.Spec.Replicas != nil {
		replicas = *statefulSet.Spec.Replicas
	}

	if statefulSet.Status.ReadyReplicas < replicas {
		return fmt.Errorf("not enough ready replicas (%d/%d)", statefulSet.Status.ReadyReplicas, replicas)
	}
	return nil
}

// CheckEtcd checks whether the given Etcd is healthy.
// A Etcd is considered healthy if its ready field in status is true.
func CheckEtcd(etcd *druidv1alpha1.Etcd) error {
	if !utils.IsTrue(etcd.Status.Ready) {
		return fmt.Errorf("etcd %s is not ready yet", etcd.Name)
	}
	return nil
}

func daemonSetMaxUnavailable(daemonSet *appsv1.DaemonSet) int32 {
	if daemonSet.Status.DesiredNumberScheduled == 0 || daemonSet.Spec.UpdateStrategy.Type != appsv1.RollingUpdateDaemonSetStrategyType {
		return 0
	}

	rollingUpdate := daemonSet.Spec.UpdateStrategy.RollingUpdate
	if rollingUpdate == nil {
		return 0
	}

	maxUnavailable, err := intstr.GetValueFromIntOrPercent(rollingUpdate.MaxUnavailable, int(daemonSet.Status.DesiredNumberScheduled), false)
	if err != nil {
		return 0
	}

	return int32(maxUnavailable)
}

// CheckDaemonSet checks whether the given DaemonSet is healthy.
// A DaemonSet is considered healthy if its controller observed its current revision and if
// its desired number of scheduled pods is equal to its updated number of scheduled pods.
func CheckDaemonSet(daemonSet *appsv1.DaemonSet) error {
	if daemonSet.Status.ObservedGeneration < daemonSet.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", daemonSet.Status.ObservedGeneration, daemonSet.Generation)
	}

	maxUnavailable := daemonSetMaxUnavailable(daemonSet)

	if requiredAvailable := daemonSet.Status.DesiredNumberScheduled - maxUnavailable; daemonSet.Status.CurrentNumberScheduled < requiredAvailable {
		return fmt.Errorf("not enough available replicas (%d/%d)", daemonSet.Status.CurrentNumberScheduled, requiredAvailable)
	}
	return nil
}

// NodeOutOfDisk is deprecated NodeConditionType.
// It is no longer reported by kubelet >= 1.13. See https://github.com/kubernetes/kubernetes/pull/70111.
// +deprecated
const NodeOutOfDisk = "OutOfDisk"

var (
	trueNodeConditionTypes = []corev1.NodeConditionType{
		corev1.NodeReady,
	}

	falseNodeConditionTypes = []corev1.NodeConditionType{
		corev1.NodeDiskPressure,
		corev1.NodeMemoryPressure,
		corev1.NodeNetworkUnavailable,
		corev1.NodePIDPressure,
		NodeOutOfDisk,
	}
)

// CheckNode checks whether the given Node is healthy.
// A node is considered healthy if it has a `corev1.NodeReady` condition and this condition reports
// `corev1.ConditionTrue`.
func CheckNode(node *corev1.Node) error {
	for _, trueConditionType := range trueNodeConditionTypes {
		conditionType := string(trueConditionType)
		condition := getNodeCondition(node.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(conditionType)
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, falseConditionType := range falseNodeConditionTypes {
		conditionType := string(falseConditionType)
		condition := getNodeCondition(node.Status.Conditions, falseConditionType)
		if condition == nil {
			continue
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}

var (
	trueSeedConditionTypes = []gardencorev1beta1.ConditionType{
		gardencorev1beta1.SeedGardenletReady,
		gardencorev1beta1.SeedBootstrapped,
	}
)

// CheckSeed checks if the Seed is up-to-date and if its extensions have been successfully bootstrapped.
func CheckSeed(seed *gardencorev1beta1.Seed, identity *gardencorev1beta1.Gardener) error {
	if !apiequality.Semantic.DeepEqual(seed.Status.Gardener, identity) {
		return fmt.Errorf("observing Gardener version not up to date (%v/%v)", seed.Status.Gardener, identity)
	}

	return checkSeed(seed, identity)
}

// CheckSeedForMigration checks if the Seed is up-to-date (comparing only the versions) and if its extensions have been successfully bootstrapped.
func CheckSeedForMigration(seed *gardencorev1beta1.Seed, identity *gardencorev1beta1.Gardener) error {
	if seed.Status.Gardener.Version != identity.Version {
		return fmt.Errorf("observing Gardener version not up to date (%s/%s)", seed.Status.Gardener.Version, identity.Version)
	}

	return checkSeed(seed, identity)
}

// checkSeed checks if the seed.Status.ObservedGeneration ObservedGeneration is not outdated and if its extensions have been successfully bootstrapped.
func checkSeed(seed *gardencorev1beta1.Seed, identity *gardencorev1beta1.Gardener) error {
	if seed.Status.ObservedGeneration < seed.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", seed.Status.ObservedGeneration, seed.Generation)
	}

	for _, trueConditionType := range trueSeedConditionTypes {
		conditionType := string(trueConditionType)
		condition := gardencorev1beta1helper.GetCondition(seed.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(conditionType)
		}
		if err := checkConditionState(conditionType, string(gardencorev1beta1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}

var (
	managedSeedConditionTypes = []gardencorev1beta1.ConditionType{
		seedmanagementv1alpha1.ManagedSeedShootReconciled,
		seedmanagementv1alpha1.ManagedSeedSeedRegistered,
	}
)

// CheckManagedSeed checks if the given ManagedSeed is up-to-date and if its Seed has been registered.
func CheckManagedSeed(managedSeed *seedmanagementv1alpha1.ManagedSeed) error {
	if managedSeed.Status.ObservedGeneration < managedSeed.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", managedSeed.Status.ObservedGeneration, managedSeed.Generation)
	}

	for _, conditionType := range managedSeedConditionTypes {
		condition := gardencorev1beta1helper.GetCondition(managedSeed.Status.Conditions, conditionType)
		if condition == nil {
			return requiredConditionMissing(string(conditionType))
		}
		if err := checkConditionState(string(conditionType), string(gardencorev1beta1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}

// CheckExtensionObject checks if an extension Object is healthy or not.
// An extension object is healthy if
// * Its observed generation is up-to-date
// * No gardener.cloud/operation is set
// * No lastError is in the status
// * A last operation is state succeeded is present
func CheckExtensionObject(o client.Object) error {
	obj, ok := o.(extensionsv1alpha1.Object)
	if !ok {
		return fmt.Errorf("expected extensionsv1alpha1.Object but got %T", o)
	}

	status := obj.GetExtensionStatus()
	return checkExtensionObject(obj.GetGeneration(), status.GetObservedGeneration(), obj.GetAnnotations(), status.GetLastError(), status.GetLastOperation())
}

// ExtensionOperationHasBeenUpdatedSince returns a health check function that checks if an extension Object's last
// operation has been updated since `lastUpdateTime`.
func ExtensionOperationHasBeenUpdatedSince(lastUpdateTime metav1.Time) Func {
	return func(o client.Object) error {
		obj, ok := o.(extensionsv1alpha1.Object)
		if !ok {
			return fmt.Errorf("expected extensionsv1alpha1.Object but got %T", o)
		}

		lastOperation := obj.GetExtensionStatus().GetLastOperation()
		if lastOperation == nil || !lastOperation.LastUpdateTime.After(lastUpdateTime.Time) {
			return fmt.Errorf("extension operation was not updated yet")
		}
		return nil
	}
}

// CheckBackupBucket checks if an backup bucket Object is healthy or not.
func CheckBackupBucket(bb client.Object) error {
	obj, ok := bb.(*gardencorev1beta1.BackupBucket)
	if !ok {
		return fmt.Errorf("expected gardencorev1beta1.BackupBucket but got %T", bb)
	}
	return checkExtensionObject(obj.Generation, obj.Status.ObservedGeneration, obj.Annotations, obj.Status.LastError, obj.Status.LastOperation)
}

// checkExtensionObject checks if an extension Object is healthy or not.
func checkExtensionObject(generation int64, observedGeneration int64, annotations map[string]string, lastError *gardencorev1beta1.LastError, lastOperation *gardencorev1beta1.LastOperation) error {
	if lastError != nil {
		return gardencorev1beta1helper.NewErrorWithCodes(fmt.Sprintf("extension encountered error during reconciliation: %s", lastError.Description), lastError.Codes...)
	}

	if observedGeneration != generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", observedGeneration, generation)
	}

	if op, ok := annotations[v1beta1constants.GardenerOperation]; ok {
		return fmt.Errorf("gardener operation %q is not yet picked up by controller", op)
	}

	if lastOperation == nil {
		return fmt.Errorf("extension did not record a last operation yet")
	}

	if lastOperation.State != gardencorev1beta1.LastOperationStateSucceeded {
		return fmt.Errorf("extension state is not succeeded but %v", lastOperation.State)
	}

	return nil
}

// Now determines the current time.
var Now = time.Now

// ConditionerFunc to update a condition with type and message
type conditionerFunc func(conditionType string, message string) gardencorev1beta1.Condition

// CheckAPIServerAvailability checks if the API server of a cluster is reachable and measure the response time.
func CheckAPIServerAvailability(ctx context.Context, condition gardencorev1beta1.Condition, restClient rest.Interface, conditioner conditionerFunc, log logrus.FieldLogger) gardencorev1beta1.Condition {
	now := Now()
	response := restClient.Get().AbsPath("/healthz").Do(ctx)
	responseDurationText := fmt.Sprintf("[response_time:%dms]", Now().Sub(now).Nanoseconds()/time.Millisecond.Nanoseconds())
	if response.Error() != nil {
		message := fmt.Sprintf("Request to API server /healthz endpoint failed. %s (%s)", responseDurationText, response.Error().Error())
		return conditioner("HealthzRequestFailed", message)
	}

	// Determine the status code of the response.
	var statusCode int
	response.StatusCode(&statusCode)

	if statusCode != http.StatusOK {
		var body string
		bodyRaw, err := response.Raw()
		if err != nil {
			body = fmt.Sprintf("Could not parse response body: %s", err.Error())
		} else {
			body = string(bodyRaw)
		}
		message := fmt.Sprintf("API server /healthz endpoint check returned a non ok status code %d. (%s)", statusCode, body)
		log.Error(message)
		return conditioner("HealthzRequestError", message)
	}

	message := "API server /healthz endpoint responded with success status code."
	return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionTrue, "HealthzRequestSucceeded", message)
}

var (
	trueManagedResourceConditionTypes = []resourcesv1alpha1.ConditionType{
		resourcesv1alpha1.ResourcesApplied,
		resourcesv1alpha1.ResourcesHealthy,
	}
)

// CheckManagedResource checks whether the given ManagedResource is healthy.
// A ManagedResource is considered healthy if its controller observed its current revision,
// and if the required conditions are healthy.
func CheckManagedResource(managedResource *resourcesv1alpha1.ManagedResource) error {
	if managedResource.Status.ObservedGeneration < managedResource.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", managedResource.Status.ObservedGeneration, managedResource.Generation)
	}

	for _, trueConditionType := range trueManagedResourceConditionTypes {
		conditionType := string(trueConditionType)
		condition := getManagedResourceCondition(managedResource.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(conditionType)
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}

func getManagedResourceCondition(conditions []resourcesv1alpha1.ManagedResourceCondition, conditionType resourcesv1alpha1.ConditionType) *resourcesv1alpha1.ManagedResourceCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

// CheckTunnelConnection checks if the tunnel connection between the control plane and the shoot networks
// is established.
func CheckTunnelConnection(ctx context.Context, shootClient kubernetes.Interface, logger logrus.FieldLogger, tunnelName string) (bool, error) {
	podList := &corev1.PodList{}
	if err := shootClient.Client().List(ctx, podList, client.InNamespace(metav1.NamespaceSystem), client.MatchingLabels{"app": tunnelName}); err != nil {
		return retry.SevereError(err)
	}

	var tunnelPod *corev1.Pod
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			tunnelPod = &pod
			break
		}
	}

	if tunnelPod == nil {
		logger.Infof("Waiting until a running %s pod exists in the Shoot cluster...", tunnelName)
		return retry.MinorError(fmt.Errorf("no running %s pod found yet in the shoot cluster", tunnelName))
	}
	if err := shootClient.CheckForwardPodPort(tunnelPod.Namespace, tunnelPod.Name, 0, 22); err != nil {
		logger.Info("Waiting until the tunnel connection has been established...")
		return retry.MinorError(fmt.Errorf("could not forward to %s pod: %v", tunnelName, err))
	}

	logger.Info("Tunnel connection has been established.")
	return retry.Ok()
}
