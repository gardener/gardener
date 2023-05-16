// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package care

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/Masterminds/semver"
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var (
	requiredControlPlaneDeployments = sets.New(
		v1beta1constants.DeploymentNameGardenerResourceManager,
		v1beta1constants.DeploymentNameKubeAPIServer,
		v1beta1constants.DeploymentNameKubeControllerManager,
		v1beta1constants.DeploymentNameKubeScheduler,
	)

	requiredControlPlaneEtcds = sets.New(
		v1beta1constants.ETCDMain,
		v1beta1constants.ETCDEvents,
	)

	requiredMonitoringSeedDeployments = sets.New(
		v1beta1constants.DeploymentNamePlutono,
		v1beta1constants.DeploymentNameKubeStateMetrics,
	)

	requiredLoggingStatefulSets = sets.New(
		v1beta1constants.StatefulSetNameVali,
	)

	requiredLoggingDeployments = sets.New(
		v1beta1constants.DeploymentNameEventLogger,
	)
)

func mustGardenRoleLabelSelector(gardenRoles ...string) labels.Selector {
	if len(gardenRoles) == 1 {
		return labels.SelectorFromSet(map[string]string{v1beta1constants.GardenRole: gardenRoles[0]})
	}

	selector := labels.NewSelector()
	requirement, err := labels.NewRequirement(v1beta1constants.GardenRole, selection.In, gardenRoles)
	if err != nil {
		panic(err)
	}

	return selector.Add(*requirement)
}

var (
	controlPlaneSelector = mustGardenRoleLabelSelector(v1beta1constants.GardenRoleControlPlane)
	monitoringSelector   = mustGardenRoleLabelSelector(v1beta1constants.GardenRoleMonitoring)
	loggingSelector      = mustGardenRoleLabelSelector(v1beta1constants.GardenRoleLogging)
)

// HealthChecker contains the condition thresholds.
type HealthChecker struct {
	reader                              client.Reader
	clock                               clock.Clock
	conditionThresholds                 map[gardencorev1beta1.ConditionType]time.Duration
	staleExtensionHealthCheckThreshold  *metav1.Duration
	managedResourceProgressingThreshold *metav1.Duration
	lastOperation                       *gardencorev1beta1.LastOperation
	kubernetesVersion                   *semver.Version
	gardenerVersion                     *semver.Version
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(
	reader client.Reader,
	clock clock.Clock,
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration,
	healthCheckOutdatedThreshold *metav1.Duration,
	managedResourceProgressingThreshold *metav1.Duration,
	lastOperation *gardencorev1beta1.LastOperation,
	kubernetesVersion *semver.Version,
	gardenerVersion *semver.Version,
) *HealthChecker {
	return &HealthChecker{
		reader:                              reader,
		clock:                               clock,
		conditionThresholds:                 conditionThresholds,
		staleExtensionHealthCheckThreshold:  healthCheckOutdatedThreshold,
		managedResourceProgressingThreshold: managedResourceProgressingThreshold,
		lastOperation:                       lastOperation,
		kubernetesVersion:                   kubernetesVersion,
		gardenerVersion:                     gardenerVersion,
	}
}

func (b *HealthChecker) checkRequiredResourceNames(condition gardencorev1beta1.Condition, requiredNames, names sets.Set[string], reason, message string) *gardencorev1beta1.Condition {
	if missingNames := requiredNames.Difference(names); missingNames.Len() != 0 {
		c := b.FailedCondition(condition, reason, fmt.Sprintf("%s: %v", message, sets.List(missingNames)))
		return &c
	}

	return nil
}

func (b *HealthChecker) checkRequiredDeployments(condition gardencorev1beta1.Condition, requiredNames sets.Set[string], objects []appsv1.Deployment) *gardencorev1beta1.Condition {
	actualNames := sets.New[string]()
	for _, object := range objects {
		actualNames.Insert(object.Name)
	}

	return b.checkRequiredResourceNames(condition, requiredNames, actualNames, "DeploymentMissing", "Missing required deployments")
}

func (b *HealthChecker) checkDeployments(condition gardencorev1beta1.Condition, objects []appsv1.Deployment) *gardencorev1beta1.Condition {
	for _, object := range objects {
		if err := health.CheckDeployment(&object); err != nil {
			c := b.FailedCondition(condition, "DeploymentUnhealthy", fmt.Sprintf("Deployment %q is unhealthy: %v", object.Name, err.Error()))
			return &c
		}
	}

	return nil
}

func (b *HealthChecker) checkRequiredEtcds(condition gardencorev1beta1.Condition, requiredNames sets.Set[string], objects []druidv1alpha1.Etcd) *gardencorev1beta1.Condition {
	actualNames := sets.New[string]()
	for _, object := range objects {
		actualNames.Insert(object.Name)
	}

	return b.checkRequiredResourceNames(condition, requiredNames, actualNames, "EtcdMissing", "Missing required etcds")
}

func (b *HealthChecker) checkEtcds(condition gardencorev1beta1.Condition, objects []druidv1alpha1.Etcd) *gardencorev1beta1.Condition {
	for _, object := range objects {
		if err := health.CheckEtcd(&object); err != nil {
			var (
				message = fmt.Sprintf("Etcd extension resource %q is unhealthy: %v", object.Name, err.Error())
				codes   []gardencorev1beta1.ErrorCode
			)

			if lastError := object.Status.LastError; lastError != nil {
				message = fmt.Sprintf("%s (%s)", message, *lastError)
			}

			c := b.FailedCondition(condition, "EtcdUnhealthy", message, codes...)
			return &c
		}
	}

	return nil
}

func (b *HealthChecker) checkRequiredStatefulSets(condition gardencorev1beta1.Condition, requiredNames sets.Set[string], objects []appsv1.StatefulSet) *gardencorev1beta1.Condition {
	actualNames := sets.New[string]()
	for _, object := range objects {
		actualNames.Insert(object.Name)
	}

	return b.checkRequiredResourceNames(condition, requiredNames, actualNames, "StatefulSetMissing", "Missing required stateful sets")
}

func (b *HealthChecker) checkStatefulSets(condition gardencorev1beta1.Condition, objects []appsv1.StatefulSet) *gardencorev1beta1.Condition {
	for _, object := range objects {
		if err := health.CheckStatefulSet(&object); err != nil {
			c := b.FailedCondition(condition, "StatefulSetUnhealthy", fmt.Sprintf("Stateful set %q is unhealthy: %v", object.Name, err.Error()))
			return &c
		}
	}

	return nil
}

func (b *HealthChecker) checkNodes(condition gardencorev1beta1.Condition, nodes []corev1.Node, workerGroupName string, workerGroupKubernetesVersion *semver.Version) *gardencorev1beta1.Condition {
	for _, object := range nodes {
		if err := health.CheckNode(&object); err != nil {
			var (
				errorCodes                 []gardencorev1beta1.ErrorCode
				message                    = fmt.Sprintf("Node %q in worker group %q is unhealthy: %v", object.Name, workerGroupName, err)
				configurationProblemRegexp = regexp.MustCompile(`(?i)(KubeletHasInsufficientMemory|KubeletHasDiskPressure|KubeletHasInsufficientPID)`)
			)

			if configurationProblemRegexp.MatchString(err.Error()) {
				errorCodes = append(errorCodes, gardencorev1beta1.ErrorConfigurationProblem)
			}

			c := b.FailedCondition(condition, "NodeUnhealthy", message, errorCodes...)
			return &c
		}

		sameMajorMinor, err := semver.NewConstraint("~ " + object.Status.NodeInfo.KubeletVersion)
		if err != nil {
			c := b.FailedCondition(condition, "VersionParseError", fmt.Sprintf("Error checking for same major minor Kubernetes version for node %q: %+v", object.Name, err))
			return &c
		}
		if sameMajorMinor.Check(workerGroupKubernetesVersion) {
			equal, err := semver.NewConstraint("= " + object.Status.NodeInfo.KubeletVersion)
			if err != nil {
				c := b.FailedCondition(condition, "VersionParseError", fmt.Sprintf("Error checking for equal Kubernetes versions for node %q: %+v", object.Name, err))
				return &c
			}

			if !equal.Check(workerGroupKubernetesVersion) {
				c := b.FailedCondition(condition, "KubeletVersionMismatch", fmt.Sprintf("The kubelet version for node %q (%s) does not match the desired Kubernetes version (v%s)", object.Name, object.Status.NodeInfo.KubeletVersion, workerGroupKubernetesVersion.Original()))
				return &c
			}
		}
	}

	return nil
}

// CheckManagedResource checks the conditions of the given managed resource and reflects the state in the returned condition.
func (b *HealthChecker) CheckManagedResource(condition gardencorev1beta1.Condition, mr *resourcesv1alpha1.ManagedResource) *gardencorev1beta1.Condition {
	conditionsToCheck := map[gardencorev1beta1.ConditionType]func(condition gardencorev1beta1.Condition) bool{
		resourcesv1alpha1.ResourcesApplied:     defaultSuccessfulCheck(),
		resourcesv1alpha1.ResourcesHealthy:     defaultSuccessfulCheck(),
		resourcesv1alpha1.ResourcesProgressing: resourcesNotProgressingCheck(b.clock, b.managedResourceProgressingThreshold),
	}

	return b.checkManagedResourceConditions(condition, mr, conditionsToCheck)
}

func defaultSuccessfulCheck() func(condition gardencorev1beta1.Condition) bool {
	return func(condition gardencorev1beta1.Condition) bool {
		return condition.Status != gardencorev1beta1.ConditionFalse && condition.Status != gardencorev1beta1.ConditionUnknown
	}
}

func resourcesNotProgressingCheck(clock clock.Clock, threshold *metav1.Duration) func(condition gardencorev1beta1.Condition) bool {
	return func(condition gardencorev1beta1.Condition) bool {
		notProgressing := condition.Status != gardencorev1beta1.ConditionTrue && condition.Status != gardencorev1beta1.ConditionUnknown

		if threshold != nil && !notProgressing && clock.Since(condition.LastTransitionTime.Time) < threshold.Duration {
			// ManagedResource is progressing but the given threshold didn't pass.
			// Hence, return that the ManagedResource is not progressing.
			return true
		}

		return notProgressing
	}
}

func (b *HealthChecker) checkManagedResourceConditions(
	condition gardencorev1beta1.Condition,
	mr *resourcesv1alpha1.ManagedResource,
	conditionsToCheck map[gardencorev1beta1.ConditionType]func(condition gardencorev1beta1.Condition) bool,
) *gardencorev1beta1.Condition {
	if mr.Generation != mr.Status.ObservedGeneration {
		c := b.FailedCondition(condition, gardencorev1beta1.OutdatedStatusError, fmt.Sprintf("observed generation of managed resource '%s/%s' outdated (%d/%d)", mr.Namespace, mr.Name, mr.Status.ObservedGeneration, mr.Generation))

		// check if MangedResource `ResourcesApplied` condition is in failed state
		conditionResourcesApplied := v1beta1helper.GetCondition(mr.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
		if conditionResourcesApplied != nil && conditionResourcesApplied.Status == gardencorev1beta1.ConditionFalse && conditionResourcesApplied.Reason == resourcesv1alpha1.ConditionApplyFailed {
			c = b.FailedCondition(condition, conditionResourcesApplied.Reason, conditionResourcesApplied.Message)
		}

		return &c
	}

	for _, condType := range []gardencorev1beta1.ConditionType{
		resourcesv1alpha1.ResourcesApplied,
		resourcesv1alpha1.ResourcesHealthy,
		resourcesv1alpha1.ResourcesProgressing,
	} {
		cond := v1beta1helper.GetCondition(mr.Status.Conditions, condType)
		if cond == nil {
			continue
		}

		checkConditionStatus, ok := conditionsToCheck[cond.Type]
		if !ok {
			continue
		}
		if !checkConditionStatus(*cond) {
			c := b.FailedCondition(condition, cond.Reason, cond.Message)
			if cond.Type == resourcesv1alpha1.ResourcesProgressing && b.managedResourceProgressingThreshold != nil {
				c = b.FailedCondition(condition, gardencorev1beta1.ManagedResourceProgressingRolloutStuck, fmt.Sprintf("ManagedResource %s is progressing for more than %s", mr.Name, b.managedResourceProgressingThreshold.Duration))
			}
			return &c
		}
		delete(conditionsToCheck, cond.Type)
	}

	if len(conditionsToCheck) > 0 {
		var missing []string
		for cond := range conditionsToCheck {
			missing = append(missing, string(cond))
		}
		c := b.FailedCondition(condition, gardencorev1beta1.ManagedResourceMissingConditionError, fmt.Sprintf("ManagedResource %s is missing the following condition(s), %v", mr.Name, missing))
		return &c
	}

	return nil
}

func shootHibernatedConditions(clock clock.Clock, conditions []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
	hibernationConditions := make([]gardencorev1beta1.Condition, 0, len(conditions))
	for _, cond := range conditions {
		hibernationConditions = append(hibernationConditions, v1beta1helper.UpdatedConditionWithClock(clock, cond, gardencorev1beta1.ConditionTrue, "ConditionNotChecked", "Shoot cluster has been hibernated."))
	}
	return hibernationConditions
}

func shootControlPlaneNotRunningMessage(lastOperation *gardencorev1beta1.LastOperation) string {
	switch {
	case lastOperation == nil || lastOperation.Type == gardencorev1beta1.LastOperationTypeCreate:
		return "Shoot control plane has not been fully created yet."
	case lastOperation.Type == gardencorev1beta1.LastOperationTypeDelete:
		return "Shoot control plane has already been or is about to be deleted."
	}
	return "Shoot control plane is not running at the moment."
}

// This is a hack to quickly do a cloud provider specific check for the required control plane deployments.
func computeRequiredControlPlaneDeployments(shoot *gardencorev1beta1.Shoot) (sets.Set[string], error) {
	shootWantsClusterAutoscaler, err := v1beta1helper.ShootWantsClusterAutoscaler(shoot)
	if err != nil {
		return nil, err
	}

	requiredControlPlaneDeployments := sets.New(requiredControlPlaneDeployments.UnsortedList()...)
	if shootWantsClusterAutoscaler {
		requiredControlPlaneDeployments.Insert(v1beta1constants.DeploymentNameClusterAutoscaler)
	}

	if v1beta1helper.ShootWantsVerticalPodAutoscaler(shoot) {
		for _, vpaDeployment := range v1beta1constants.GetShootVPADeploymentNames() {
			requiredControlPlaneDeployments.Insert(vpaDeployment)
		}
	}

	return requiredControlPlaneDeployments, nil
}

// computeRequiredMonitoringStatefulSets determine the required monitoring statefulsets
// which should exist next to the control plane.
func computeRequiredMonitoringStatefulSets(wantsAlertmanager bool) sets.Set[string] {
	var requiredMonitoringStatefulSets = sets.New(v1beta1constants.StatefulSetNamePrometheus)
	if wantsAlertmanager {
		requiredMonitoringStatefulSets.Insert(v1beta1constants.StatefulSetNameAlertManager)
	}
	return requiredMonitoringStatefulSets
}

// CheckControlPlane checks whether the control plane components in the given listers are complete and healthy.
func (b *HealthChecker) CheckControlPlane(
	ctx context.Context,
	shoot *gardencorev1beta1.Shoot,
	namespace string,
	condition gardencorev1beta1.Condition,
) (
	*gardencorev1beta1.Condition,
	error,
) {
	requiredControlPlaneDeployments, err := computeRequiredControlPlaneDeployments(shoot)
	if err != nil {
		return nil, err
	}

	deploymentList := &appsv1.DeploymentList{}
	if err := b.reader.List(ctx, deploymentList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: controlPlaneSelector}); err != nil {
		return nil, err
	}

	etcdList := &druidv1alpha1.EtcdList{}
	if err := b.reader.List(ctx, etcdList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: controlPlaneSelector}); err != nil {
		return nil, err
	}

	if exitCondition := b.checkRequiredDeployments(condition, requiredControlPlaneDeployments, deploymentList.Items); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := b.checkDeployments(condition, deploymentList.Items); exitCondition != nil {
		return exitCondition, nil
	}

	if exitCondition := b.checkRequiredEtcds(condition, requiredControlPlaneEtcds, etcdList.Items); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := b.checkEtcds(condition, etcdList.Items); exitCondition != nil {
		return exitCondition, nil
	}

	return nil, nil
}

// FailedCondition returns a progressing or false condition depending on the progressing threshold.
func (b *HealthChecker) FailedCondition(condition gardencorev1beta1.Condition, reason, message string, codes ...gardencorev1beta1.ErrorCode) gardencorev1beta1.Condition {
	switch condition.Status {
	case gardencorev1beta1.ConditionTrue:
		if _, ok := b.conditionThresholds[condition.Type]; !ok {
			return v1beta1helper.UpdatedConditionWithClock(b.clock, condition, gardencorev1beta1.ConditionFalse, reason, message, codes...)
		}
		return v1beta1helper.UpdatedConditionWithClock(b.clock, condition, gardencorev1beta1.ConditionProgressing, reason, message, codes...)

	case gardencorev1beta1.ConditionProgressing:
		threshold, ok := b.conditionThresholds[condition.Type]
		if !ok {
			return v1beta1helper.UpdatedConditionWithClock(b.clock, condition, gardencorev1beta1.ConditionFalse, reason, message, codes...)
		}
		if b.lastOperation != nil && b.lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded && b.clock.Now().UTC().Sub(b.lastOperation.LastUpdateTime.UTC()) <= threshold {
			return v1beta1helper.UpdatedConditionWithClock(b.clock, condition, gardencorev1beta1.ConditionProgressing, reason, message, codes...)
		}
		if delta := b.clock.Now().UTC().Sub(condition.LastTransitionTime.Time.UTC()); delta <= threshold {
			return v1beta1helper.UpdatedConditionWithClock(b.clock, condition, gardencorev1beta1.ConditionProgressing, reason, message, codes...)
		}
		return v1beta1helper.UpdatedConditionWithClock(b.clock, condition, gardencorev1beta1.ConditionFalse, reason, message, codes...)

	case gardencorev1beta1.ConditionFalse:
		threshold, ok := b.conditionThresholds[condition.Type]
		if ok &&
			((b.lastOperation != nil && b.lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded && b.clock.Now().UTC().Sub(b.lastOperation.LastUpdateTime.UTC()) <= threshold) ||
				(reason != condition.Reason)) {
			return v1beta1helper.UpdatedConditionWithClock(b.clock, condition, gardencorev1beta1.ConditionProgressing, reason, message, codes...)
		}
	}

	return v1beta1helper.UpdatedConditionWithClock(b.clock, condition, gardencorev1beta1.ConditionFalse, reason, message, codes...)
}

// CheckClusterNodes checks whether cluster nodes in the given listers are healthy and within the desired range.
// Additional checks are executed in the provider extension
func (b *HealthChecker) CheckClusterNodes(
	ctx context.Context,
	shootClient client.Client,
	workers []gardencorev1beta1.Worker,
	condition gardencorev1beta1.Condition,
) (
	*gardencorev1beta1.Condition,
	error,
) {
	workerPoolToNodes, err := botanist.WorkerPoolToNodesMap(ctx, shootClient)
	if err != nil {
		return nil, err
	}

	workerPoolToCloudConfigSecretMeta, err := botanist.WorkerPoolToCloudConfigSecretMetaMap(ctx, shootClient)
	if err != nil {
		return nil, err
	}

	for _, worker := range workers {
		nodes := workerPoolToNodes[worker.Name]

		kubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(b.kubernetesVersion, worker.Kubernetes)
		if err != nil {
			return nil, err
		}

		if exitCondition := b.checkNodes(condition, nodes, worker.Name, kubernetesVersion); exitCondition != nil {
			return exitCondition, nil
		}

		if len(nodes) < int(worker.Minimum) {
			c := b.FailedCondition(condition, "MissingNodes", fmt.Sprintf("Not enough worker nodes registered in worker pool %q to meet minimum desired machine count. (%d/%d).", worker.Name, len(nodes), worker.Minimum))
			return &c, nil
		}
	}

	if err := botanist.CloudConfigUpdatedForAllWorkerPools(workers, workerPoolToNodes, workerPoolToCloudConfigSecretMeta); err != nil {
		c := b.FailedCondition(condition, "CloudConfigOutdated", err.Error())
		return &c, nil
	}

	return nil, nil
}

// CheckMonitoringControlPlane checks whether the monitoring in the given listers are complete and healthy.
func (b *HealthChecker) CheckMonitoringControlPlane(
	ctx context.Context,
	namespace string,
	shootMonitoringEnabled bool,
	wantsAlertmanager bool,
	condition gardencorev1beta1.Condition,
) (
	*gardencorev1beta1.Condition,
	error,
) {
	if !shootMonitoringEnabled {
		return nil, nil
	}

	deploymentList := &appsv1.DeploymentList{}
	if err := b.reader.List(ctx, deploymentList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: monitoringSelector}); err != nil {
		return nil, err
	}

	statefulSetList := &appsv1.StatefulSetList{}
	if err := b.reader.List(ctx, statefulSetList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: monitoringSelector}); err != nil {
		return nil, err
	}

	if exitCondition := b.checkRequiredDeployments(condition, requiredMonitoringSeedDeployments, deploymentList.Items); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := b.checkDeployments(condition, deploymentList.Items); exitCondition != nil {
		return exitCondition, nil
	}

	if exitCondition := b.checkRequiredStatefulSets(condition, computeRequiredMonitoringStatefulSets(wantsAlertmanager), statefulSetList.Items); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := b.checkStatefulSets(condition, statefulSetList.Items); exitCondition != nil {
		return exitCondition, nil
	}

	return nil, nil
}

// CheckLoggingControlPlane checks whether the logging components in the given listers are complete and healthy.
func (b *HealthChecker) CheckLoggingControlPlane(
	ctx context.Context,
	namespace string,
	isTestingShoot bool,
	eventLoggingEnabled bool,
	valiEnabled bool,
	condition gardencorev1beta1.Condition,
) (
	*gardencorev1beta1.Condition,
	error,
) {
	if isTestingShoot {
		return nil, nil
	}

	if valiEnabled {
		statefulSetList := &appsv1.StatefulSetList{}
		if err := b.reader.List(ctx, statefulSetList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: loggingSelector}); err != nil {
			return nil, err
		}

		if exitCondition := b.checkRequiredStatefulSets(condition, requiredLoggingStatefulSets, statefulSetList.Items); exitCondition != nil {
			return exitCondition, nil
		}
		if exitCondition := b.checkStatefulSets(condition, statefulSetList.Items); exitCondition != nil {
			return exitCondition, nil
		}
	}

	if eventLoggingEnabled {
		deploymentList := &appsv1.DeploymentList{}
		if err := b.reader.List(ctx, deploymentList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: loggingSelector}); err != nil {
			return nil, err
		}

		if exitCondition := b.checkRequiredDeployments(condition, requiredLoggingDeployments, deploymentList.Items); exitCondition != nil {
			return exitCondition, nil
		}
		if exitCondition := b.checkDeployments(condition, deploymentList.Items); exitCondition != nil {
			return exitCondition, nil
		}
	}

	return nil, nil
}

// CheckExtensionCondition checks whether the conditions provided by extensions are healthy.
func (b *HealthChecker) CheckExtensionCondition(condition gardencorev1beta1.Condition, extensionsConditions []ExtensionCondition) *gardencorev1beta1.Condition {
	for _, cond := range extensionsConditions {
		// check if the extension controller's last heartbeat time or the condition's LastUpdateTime is older than the configured staleExtensionHealthCheckThreshold
		if b.staleExtensionHealthCheckThreshold != nil {
			lastHeartbeatTime := cond.LastHeartbeatTime
			if lastHeartbeatTime == nil {
				lastHeartbeatTime = &metav1.MicroTime{}
			}
			if b.clock.Now().UTC().Sub(lastHeartbeatTime.UTC()) > b.staleExtensionHealthCheckThreshold.Duration {
				c := v1beta1helper.UpdatedConditionWithClock(b.clock, condition, gardencorev1beta1.ConditionUnknown, fmt.Sprintf("%sOutdatedHealthCheckReport", cond.ExtensionType), fmt.Sprintf("%s extension (%s/%s) reports an outdated health status (last updated: %s ago at %s).", cond.ExtensionType, cond.ExtensionNamespace, cond.ExtensionName, b.clock.Now().UTC().Sub(lastHeartbeatTime.UTC()).Round(time.Minute).String(), lastHeartbeatTime.UTC().Round(time.Minute).String()))
				return &c
			}
		}

		if cond.Condition.Status == gardencorev1beta1.ConditionProgressing {
			c := v1beta1helper.UpdatedConditionWithClock(b.clock, condition, cond.Condition.Status, cond.ExtensionType+cond.Condition.Reason, cond.Condition.Message, cond.Condition.Codes...)
			return &c
		}

		if cond.Condition.Status == gardencorev1beta1.ConditionFalse || cond.Condition.Status == gardencorev1beta1.ConditionUnknown {
			c := b.FailedCondition(condition, fmt.Sprintf("%sUnhealthyReport", cond.ExtensionType), fmt.Sprintf("%s extension (%s/%s) reports failing health check: %s", cond.ExtensionType, cond.ExtensionNamespace, cond.ExtensionName, cond.Condition.Message), cond.Condition.Codes...)
			return &c
		}
	}

	return nil
}

// NewConditionOrError returns the given new condition or returns an unknown error condition if an error occurred or `newCondition` is nil.
func NewConditionOrError(clock clock.Clock, oldCondition gardencorev1beta1.Condition, newCondition *gardencorev1beta1.Condition, err error) gardencorev1beta1.Condition {
	if err != nil || newCondition == nil {
		return v1beta1helper.UpdatedConditionUnknownErrorWithClock(clock, oldCondition, err)
	}
	return *newCondition
}

func isUnstableLastOperation(lastOperation *gardencorev1beta1.LastOperation, lastErrors []gardencorev1beta1.LastError) bool {
	return (isUnstableOperationType(lastOperation.Type) && lastOperation.State != gardencorev1beta1.LastOperationStateSucceeded) ||
		(lastOperation.State == gardencorev1beta1.LastOperationStateProcessing && lastErrors == nil)
}

var unstableOperationTypes = map[gardencorev1beta1.LastOperationType]struct{}{
	gardencorev1beta1.LastOperationTypeCreate: {},
	gardencorev1beta1.LastOperationTypeDelete: {},
}

func isUnstableOperationType(lastOperationType gardencorev1beta1.LastOperationType) bool {
	_, ok := unstableOperationTypes[lastOperationType]
	return ok
}

// PardonConditions pardons the given condition if the Shoot is either in create (except successful create) or delete state.
func PardonConditions(clock clock.Clock, conditions []gardencorev1beta1.Condition, lastOp *gardencorev1beta1.LastOperation, lastErrors []gardencorev1beta1.LastError) []gardencorev1beta1.Condition {
	pardoningConditions := make([]gardencorev1beta1.Condition, 0, len(conditions))
	for _, cond := range conditions {
		if (lastOp == nil || isUnstableLastOperation(lastOp, lastErrors)) && cond.Status == gardencorev1beta1.ConditionFalse {
			pardoningConditions = append(pardoningConditions, v1beta1helper.UpdatedConditionWithClock(clock, cond, gardencorev1beta1.ConditionProgressing, cond.Reason, cond.Message, cond.Codes...))
			continue
		}
		pardoningConditions = append(pardoningConditions, cond)
	}
	return pardoningConditions
}
