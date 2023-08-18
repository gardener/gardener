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
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
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
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
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
) *HealthChecker {
	return &HealthChecker{
		reader:                              reader,
		clock:                               clock,
		conditionThresholds:                 conditionThresholds,
		staleExtensionHealthCheckThreshold:  healthCheckOutdatedThreshold,
		managedResourceProgressingThreshold: managedResourceProgressingThreshold,
		lastOperation:                       lastOperation,
		kubernetesVersion:                   kubernetesVersion,
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

// kubeletConfigProblemRegex is used to check if an error occurred due to a kubelet configuration problem.
var kubeletConfigProblemRegex = regexp.MustCompile(`(?i)(KubeletHasInsufficientMemory|KubeletHasDiskPressure|KubeletHasInsufficientPID)`)

func (b *HealthChecker) checkNodes(condition gardencorev1beta1.Condition, nodes []corev1.Node, workerGroupName string, workerGroupKubernetesVersion *semver.Version) *gardencorev1beta1.Condition {
	for _, object := range nodes {
		if err := health.CheckNode(&object); err != nil {
			var (
				errorCodes []gardencorev1beta1.ErrorCode
				message    = fmt.Sprintf("Node %q in worker group %q is unhealthy: %v", object.Name, workerGroupName, err)
			)

			if kubeletConfigProblemRegex.MatchString(err.Error()) {
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

// annotationKeyNotManagedByMCM is a constant for an annotation on the node resource that indicates that the node is not
// handled by machine-controller-manager.
const annotationKeyNotManagedByMCM = "node.machine.sapcloud.io/not-managed-by-mcm"

func convertWorkerPoolToNodesMappingToNodeList(workerPoolToNodes map[string][]corev1.Node) *corev1.NodeList {
	nodeList := &corev1.NodeList{}

	for _, nodes := range workerPoolToNodes {
		nodeList.Items = append(nodeList.Items, nodes...)
	}

	return nodeList
}

func getDesiredMachineCount(machineDeployments []machinev1alpha1.MachineDeployment) int {
	desiredMachines := 0
	for _, deployment := range machineDeployments {
		if deployment.DeletionTimestamp == nil {
			desiredMachines += int(deployment.Spec.Replicas)
		}
	}
	return desiredMachines
}

// CheckNodesScalingUp returns an error of nodes are being scaled up.
func CheckNodesScalingUp(machineList *machinev1alpha1.MachineList, readyNodes, desiredMachines int) error {
	if readyNodes == desiredMachines {
		return nil
	}

	if machineObjects := len(machineList.Items); machineObjects < desiredMachines {
		return fmt.Errorf("not enough machine objects created yet (%d/%d)", machineObjects, desiredMachines)
	}

	var pendingMachines, erroneousMachines int
	for _, machine := range machineList.Items {
		switch machine.Status.CurrentStatus.Phase {
		case machinev1alpha1.MachineRunning, machinev1alpha1.MachineAvailable:
			// machine is already running fine
			continue
		case machinev1alpha1.MachinePending, "": // https://github.com/gardener/machine-controller-manager/issues/466
			// machine is in the process of being created
			pendingMachines++
		default:
			// undesired machine phase
			erroneousMachines++
		}
	}

	if erroneousMachines > 0 {
		return fmt.Errorf("%s erroneous", cosmeticMachineMessage(erroneousMachines))
	}
	if pendingMachines == 0 {
		return fmt.Errorf("not enough ready worker nodes registered in the cluster (%d/%d)", readyNodes, desiredMachines)
	}

	return fmt.Errorf("%s provisioning and should join the cluster soon", cosmeticMachineMessage(pendingMachines))
}

// CheckNodesScalingDown returns an error if nodes are being scaled down.
func CheckNodesScalingDown(machineList *machinev1alpha1.MachineList, nodeList *corev1.NodeList, registeredNodes, desiredMachines int) error {
	if registeredNodes == desiredMachines {
		return nil
	}

	// Check if all nodes that are cordoned map to machines with a deletion timestamp. This might be the case during
	// a rolling update.
	nodeNameToMachine := map[string]machinev1alpha1.Machine{}
	for _, machine := range machineList.Items {
		if machine.Labels != nil && machine.Labels["node"] != "" {
			nodeNameToMachine[machine.Labels["node"]] = machine
		}
	}

	var cordonedNodes int
	for _, node := range nodeList.Items {
		if metav1.HasAnnotation(node.ObjectMeta, annotationKeyNotManagedByMCM) && node.Annotations[annotationKeyNotManagedByMCM] == "1" {
			continue
		}
		if node.Spec.Unschedulable {
			machine, ok := nodeNameToMachine[node.Name]
			if !ok {
				return fmt.Errorf("machine object for cordoned node %q not found", node.Name)
			}
			if machine.DeletionTimestamp == nil {
				return fmt.Errorf("cordoned node %q found but corresponding machine object does not have a deletion timestamp", node.Name)
			}
			cordonedNodes++
		}
	}

	// If there are still more nodes than desired then report an error.
	if registeredNodes-cordonedNodes != desiredMachines {
		return fmt.Errorf("too many worker nodes are registered. Exceeding maximum desired machine count (%d/%d)", registeredNodes, desiredMachines)
	}

	return fmt.Errorf("%s waiting to be completely drained from pods. If this persists, check your pod disruption budgets and pending finalizers. Please note, that nodes that fail to be drained will be deleted automatically", cosmeticMachineMessage(cordonedNodes))
}

func cosmeticMachineMessage(numberOfMachines int) string {
	if numberOfMachines == 1 {
		return fmt.Sprintf("%d machine is", numberOfMachines)
	}
	return fmt.Sprintf("%d machines are", numberOfMachines)
}

// CheckManagedResource checks the conditions of the given managed resource and reflects the state in the returned condition.
func (b *HealthChecker) CheckManagedResource(condition gardencorev1beta1.Condition, mr *resourcesv1alpha1.ManagedResource) *gardencorev1beta1.Condition {
	conditionsToCheck := map[gardencorev1beta1.ConditionType]func(condition gardencorev1beta1.Condition) bool{
		resourcesv1alpha1.ResourcesApplied:     DefaultSuccessfulCheck(),
		resourcesv1alpha1.ResourcesHealthy:     DefaultSuccessfulCheck(),
		resourcesv1alpha1.ResourcesProgressing: ResourcesNotProgressingCheck(b.clock, b.managedResourceProgressingThreshold),
	}

	return b.CheckManagedResourceConditions(condition, mr, conditionsToCheck)
}

// DefaultSuccessfulCheck returns a function that checks whether the condition status is successful.
func DefaultSuccessfulCheck() func(condition gardencorev1beta1.Condition) bool {
	return func(condition gardencorev1beta1.Condition) bool {
		return condition.Status != gardencorev1beta1.ConditionFalse && condition.Status != gardencorev1beta1.ConditionUnknown
	}
}

// ResourcesNotProgressingCheck returns a function that checks a condition is not progressing.
func ResourcesNotProgressingCheck(clock clock.Clock, threshold *metav1.Duration) func(condition gardencorev1beta1.Condition) bool {
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

// CheckManagedResourceConditions checks the given conditions at the ManagedResource.
func (b *HealthChecker) CheckManagedResourceConditions(
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

// CheckControlPlane checks whether the control plane components in the given listers are complete and healthy.
func (b *HealthChecker) CheckControlPlane(
	ctx context.Context,
	namespace string,
	requiredControlPlaneDeployments sets.Set[string],
	requiredControlPlaneEtcds sets.Set[string],
	condition gardencorev1beta1.Condition,
) (
	*gardencorev1beta1.Condition,
	error,
) {
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
	seedNamespace string,
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

	for _, pool := range workers {
		nodes := workerPoolToNodes[pool.Name]

		kubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(b.kubernetesVersion, pool.Kubernetes)
		if err != nil {
			return nil, err
		}

		if exitCondition := b.checkNodes(condition, nodes, pool.Name, kubernetesVersion); exitCondition != nil {
			return exitCondition, nil
		}

		if len(nodes) < int(pool.Minimum) {
			c := b.FailedCondition(condition, "MissingNodes", fmt.Sprintf("Not enough worker nodes registered in worker pool %q to meet minimum desired machine count. (%d/%d).", pool.Name, len(nodes), pool.Minimum))
			return &c, nil
		}
	}

	if err := botanist.CloudConfigUpdatedForAllWorkerPools(workers, workerPoolToNodes, workerPoolToCloudConfigSecretMeta); err != nil {
		c := b.FailedCondition(condition, "CloudConfigOutdated", err.Error())
		return &c, nil
	}

	if !features.DefaultFeatureGate.Enabled(features.MachineControllerManagerDeployment) {
		return nil, nil
	}

	machineDeploymentList := &machinev1alpha1.MachineDeploymentList{}
	if err := b.reader.List(ctx, machineDeploymentList, client.InNamespace(seedNamespace)); err != nil {
		return nil, err
	}

	var (
		nodeList            = convertWorkerPoolToNodesMappingToNodeList(workerPoolToNodes)
		readyNodes          int
		registeredNodes     = len(nodeList.Items)
		desiredMachines     = getDesiredMachineCount(machineDeploymentList.Items)
		nodeNotManagedByMCM int
	)

	for _, node := range nodeList.Items {
		if metav1.HasAnnotation(node.ObjectMeta, annotationKeyNotManagedByMCM) && node.Annotations[annotationKeyNotManagedByMCM] == "1" {
			nodeNotManagedByMCM++
			continue
		}
		if node.Spec.Unschedulable {
			continue
		}
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				readyNodes++
			}
		}
	}

	// only nodes that are managed by MCM is considered
	registeredNodes = registeredNodes - nodeNotManagedByMCM

	machineList := &machinev1alpha1.MachineList{}
	if registeredNodes != desiredMachines || readyNodes != desiredMachines {
		if err := b.reader.List(ctx, machineList, client.InNamespace(seedNamespace)); err != nil {
			return nil, err
		}
	}

	// First check if the MachineDeployments report failed machines. If false then check if the MachineDeployments are
	// "available". If false then check if there is a regular scale-up happening or if there are machines with an erroneous
	// phase. Only then check the other MachineDeployment conditions. As last check, check if there is a scale-down happening
	// (e.g., in case of an rolling-update).

	checkScaleUp := false
	for _, deployment := range machineDeploymentList.Items {
		if len(deployment.Status.FailedMachines) > 0 {
			break
		}

		for _, condition := range deployment.Status.Conditions {
			if condition.Type == machinev1alpha1.MachineDeploymentAvailable && condition.Status != machinev1alpha1.ConditionTrue {
				checkScaleUp = true
				break
			}
		}
	}

	if checkScaleUp {
		if err := CheckNodesScalingUp(machineList, readyNodes, desiredMachines); err != nil {
			c := b.FailedCondition(condition, "NodesScalingUp", err.Error())
			return &c, nil
		}
	}

	if err := CheckNodesScalingDown(machineList, nodeList, registeredNodes, desiredMachines); err != nil {
		c := b.FailedCondition(condition, "NodesScalingDown", err.Error())
		return &c, nil
	}

	return nil, nil
}

// CheckMonitoringControlPlane checks the monitoring components of the control-plane.
func (b *HealthChecker) CheckMonitoringControlPlane(
	ctx context.Context,
	namespace string,
	requiredMonitoringDeployments sets.Set[string],
	requiredMonitoringStatefulSets sets.Set[string],
	appsSelector labels.Selector,
	condition gardencorev1beta1.Condition,
) (
	*gardencorev1beta1.Condition,
	error,
) {
	deploymentList := &appsv1.DeploymentList{}
	if err := b.reader.List(ctx, deploymentList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: appsSelector}); err != nil {
		return nil, err
	}

	statefulSetList := &appsv1.StatefulSetList{}
	if err := b.reader.List(ctx, statefulSetList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: appsSelector}); err != nil {
		return nil, err
	}

	if exitCondition := b.checkRequiredDeployments(condition, requiredMonitoringDeployments, deploymentList.Items); exitCondition != nil {
		return exitCondition, nil
	}

	if exitCondition := b.checkDeployments(condition, deploymentList.Items); exitCondition != nil {
		return exitCondition, nil
	}

	if exitCondition := b.checkRequiredStatefulSets(condition, requiredMonitoringStatefulSets, statefulSetList.Items); exitCondition != nil {
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
