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

package botanist

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenerextensions "github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/botanist/systemcomponents/metricsserver"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	"github.com/Masterminds/semver"
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func mustGardenRoleLabelSelector(gardenRoles ...string) labels.Selector {
	if len(gardenRoles) == 1 {
		return labels.SelectorFromSet(map[string]string{v1beta1constants.DeprecatedGardenRole: gardenRoles[0]})
	}

	selector := labels.NewSelector()
	requirement, err := labels.NewRequirement(v1beta1constants.DeprecatedGardenRole, selection.In, gardenRoles)
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

// Now determines the current time.
var Now = time.Now

// HealthChecker contains the condition thresholds.
type HealthChecker struct {
	conditionThresholds                map[gardencorev1beta1.ConditionType]time.Duration
	staleExtensionHealthCheckThreshold *metav1.Duration
	lastOperation                      *gardencorev1beta1.LastOperation
	kubernetesVersion                  *semver.Version
	gardenerVersion                    *semver.Version
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration, healthCheckOutdatedThreshold *metav1.Duration, lastOperation *gardencorev1beta1.LastOperation, kubernetesVersion *semver.Version, gardenerVersion *semver.Version) *HealthChecker {
	return &HealthChecker{
		conditionThresholds:                conditionThresholds,
		staleExtensionHealthCheckThreshold: healthCheckOutdatedThreshold,
		lastOperation:                      lastOperation,
		kubernetesVersion:                  kubernetesVersion,
		gardenerVersion:                    gardenerVersion,
	}
}

func (b *HealthChecker) checkRequiredResourceNames(condition gardencorev1beta1.Condition, requiredNames, names sets.String, reason, message string) *gardencorev1beta1.Condition {
	if missingNames := requiredNames.Difference(names); missingNames.Len() != 0 {
		c := b.FailedCondition(condition, reason, fmt.Sprintf("%s: %v", message, missingNames.List()))
		return &c
	}

	return nil
}

func (b *HealthChecker) checkRequiredDeployments(condition gardencorev1beta1.Condition, requiredNames sets.String, objects []*appsv1.Deployment) *gardencorev1beta1.Condition {
	actualNames := sets.NewString()
	for _, object := range objects {
		actualNames.Insert(object.Name)
	}

	return b.checkRequiredResourceNames(condition, requiredNames, actualNames, "DeploymentMissing", "Missing required deployments")
}

func (b *HealthChecker) checkDeployments(condition gardencorev1beta1.Condition, objects []*appsv1.Deployment) *gardencorev1beta1.Condition {
	for _, object := range objects {
		if err := health.CheckDeployment(object); err != nil {
			c := b.FailedCondition(condition, "DeploymentUnhealthy", fmt.Sprintf("Deployment %s is unhealthy: %v", object.Name, err.Error()))
			return &c
		}
	}

	return nil
}

func (b *HealthChecker) checkRequiredEtcds(condition gardencorev1beta1.Condition, requiredNames sets.String, objects []*druidv1alpha1.Etcd) *gardencorev1beta1.Condition {
	actualNames := sets.NewString()
	for _, object := range objects {
		actualNames.Insert(object.Name)
	}

	return b.checkRequiredResourceNames(condition, requiredNames, actualNames, "EtcdMissing", "Missing required etcds")
}

func (b *HealthChecker) checkEtcds(condition gardencorev1beta1.Condition, objects []*druidv1alpha1.Etcd) *gardencorev1beta1.Condition {
	for _, object := range objects {
		if err := health.CheckEtcd(object); err != nil {
			var (
				message = fmt.Sprintf("Etcd %s is unhealthy: %v", object.Name, err.Error())
				codes   []gardencorev1beta1.ErrorCode
			)

			if lastError := object.Status.LastError; lastError != nil {
				codes = gardencorev1beta1helper.ExtractErrorCodes(gardencorev1beta1helper.DetermineError(errors.New(*lastError), ""))
				message = fmt.Sprintf("%s (%s)", message, *lastError)
			}

			c := b.FailedCondition(condition, "EtcdUnhealthy", message, codes...)
			return &c
		}
	}

	return nil
}

func (b *HealthChecker) checkRequiredStatefulSets(condition gardencorev1beta1.Condition, requiredNames sets.String, objects []*appsv1.StatefulSet) *gardencorev1beta1.Condition {
	actualNames := sets.NewString()
	for _, object := range objects {
		actualNames.Insert(object.Name)
	}

	return b.checkRequiredResourceNames(condition, requiredNames, actualNames, "StatefulSetMissing", "Missing required stateful sets")
}

func (b *HealthChecker) checkStatefulSets(condition gardencorev1beta1.Condition, objects []*appsv1.StatefulSet) *gardencorev1beta1.Condition {
	for _, object := range objects {
		if err := health.CheckStatefulSet(object); err != nil {
			c := b.FailedCondition(condition, "StatefulSetUnhealthy", fmt.Sprintf("Stateful set %s is unhealthy: %v", object.Name, err.Error()))
			return &c
		}
	}

	return nil
}

func (b *HealthChecker) checkNodes(condition gardencorev1beta1.Condition, objects []*corev1.Node, workerGroupName string) *gardencorev1beta1.Condition {
	for _, object := range objects {
		if err := health.CheckNode(object); err != nil {
			var (
				message = fmt.Sprintf("Node '%s' in worker group '%s' is unhealthy: %v", object.Name, workerGroupName, err)
				codes   = gardencorev1beta1helper.ExtractErrorCodes(gardencorev1beta1helper.DetermineError(err, ""))
			)

			c := b.FailedCondition(condition, "NodeUnhealthy", message, codes...)
			return &c
		}

		sameMajorMinor, err := semver.NewConstraint("~ " + object.Status.NodeInfo.KubeletVersion)
		if err != nil {
			c := b.FailedCondition(condition, "VersionParseError", fmt.Sprintf("Error checking for same major minor Kubernetes version for node %q: %+v", object.Name, err))
			return &c
		}

		if sameMajorMinor.Check(b.kubernetesVersion) {
			equal, err := semver.NewConstraint("= " + object.Status.NodeInfo.KubeletVersion)
			if err != nil {
				c := b.FailedCondition(condition, "VersionParseError", fmt.Sprintf("Error checking for equal Kubernetes versions for node %q: %+v", object.Name, err))
				return &c
			}

			if !equal.Check(b.kubernetesVersion) {
				c := b.FailedCondition(condition, "KubeletVersionMismatch", fmt.Sprintf("The kubelet version for node %q (%s) does not match the desired Kubernetes version (v%s)", object.Name, object.Status.NodeInfo.KubeletVersion, b.kubernetesVersion.Original()))
				return &c
			}
		}
	}

	return nil
}

// CheckManagedResource checks the conditions of the given managed resource and reflects the state in the returned condition.
func (b *HealthChecker) CheckManagedResource(condition gardencorev1beta1.Condition, mr *resourcesv1alpha1.ManagedResource) *gardencorev1beta1.Condition {
	if mr.Generation != mr.Status.ObservedGeneration {
		c := b.FailedCondition(condition, gardencorev1beta1.OutdatedStatusError, fmt.Sprintf("observed generation of managed resource %s/%s outdated (%d/%d)", mr.Namespace, mr.Name, mr.Status.ObservedGeneration, mr.Generation))
		return &c
	}

	toProcess := map[resourcesv1alpha1.ConditionType]struct{}{
		resourcesv1alpha1.ResourcesApplied: {},
		resourcesv1alpha1.ResourcesHealthy: {},
	}

	for _, cond := range mr.Status.Conditions {
		_, ok := toProcess[cond.Type]
		if !ok {
			continue
		}
		if cond.Status == resourcesv1alpha1.ConditionFalse {
			c := b.FailedCondition(condition, cond.Reason, cond.Message)
			return &c
		}
		delete(toProcess, cond.Type)
	}

	if len(toProcess) > 0 {
		var missing []string
		for cond := range toProcess {
			missing = append(missing, string(cond))
		}
		c := b.FailedCondition(condition, gardencorev1beta1.ManagedResourceMissingConditionError, fmt.Sprintf("ManagedResource %s is missing the following condition(s), %v", mr.Name, missing))
		return &c
	}

	return nil
}

func shootHibernatedCondition(condition gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionTrue, "ConditionNotChecked", "Shoot cluster has been hibernated.")
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
func computeRequiredControlPlaneDeployments(
	shoot *gardencorev1beta1.Shoot,
	workerLister kutil.WorkerLister,
) (sets.String, error) {
	shootWantsClusterAutoscaler, err := gardencorev1beta1helper.ShootWantsClusterAutoscaler(shoot)
	if err != nil {
		return nil, err
	}

	requiredControlPlaneDeployments := sets.NewString(common.RequiredControlPlaneDeployments.UnsortedList()...)
	if shootWantsClusterAutoscaler {
		workers, err := workerLister.List(labels.Everything())
		if err != nil {
			return nil, err
		}

		// if worker resource is processing (during maintenance), there might be a rolling update in progress
		// during rolling updates, the autoscaler deployment is scaled down & therefore not required
		rollingUpdateMightBeOngoing := false
		for _, worker := range workers {
			if worker.Status.LastOperation != nil && worker.Status.LastOperation.State == gardencorev1beta1.LastOperationStateProcessing {
				rollingUpdateMightBeOngoing = true
				break
			}
		}

		if !rollingUpdateMightBeOngoing {
			requiredControlPlaneDeployments.Insert(v1beta1constants.DeploymentNameClusterAutoscaler)
		}
	}

	if gardencorev1beta1helper.ShootWantsVerticalPodAutoscaler(shoot) {
		for _, vpaDeployment := range v1beta1constants.GetShootVPADeploymentNames() {
			requiredControlPlaneDeployments.Insert(vpaDeployment)
		}
	}

	return requiredControlPlaneDeployments, nil
}

// computeRequiredMonitoringStatefulSets determine the required monitoring statefulsets
// which should exist next to the control plane.
func computeRequiredMonitoringStatefulSets(wantsAlertmanager bool) sets.String {
	var requiredMonitoringStatefulSets = sets.NewString(v1beta1constants.StatefulSetNamePrometheus)
	if wantsAlertmanager {
		requiredMonitoringStatefulSets.Insert(v1beta1constants.StatefulSetNameAlertManager)
	}
	return requiredMonitoringStatefulSets
}

// CheckControlPlane checks whether the control plane components in the given listers are complete and healthy.
func (b *HealthChecker) CheckControlPlane(
	shoot *gardencorev1beta1.Shoot,
	namespace string,
	condition gardencorev1beta1.Condition,
	deploymentLister kutil.DeploymentLister,
	etcdLister kutil.EtcdLister,
	workerLister kutil.WorkerLister,
) (*gardencorev1beta1.Condition, error) {
	requiredControlPlaneDeployments, err := computeRequiredControlPlaneDeployments(shoot, workerLister)
	if err != nil {
		return nil, err
	}

	deployments, err := deploymentLister.Deployments(namespace).List(controlPlaneSelector)
	if err != nil {
		return nil, err
	}

	if exitCondition := b.checkRequiredDeployments(condition, requiredControlPlaneDeployments, deployments); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := b.checkDeployments(condition, deployments); exitCondition != nil {
		return exitCondition, nil
	}

	etcds, err := etcdLister.Etcds(namespace).List(controlPlaneSelector)
	if err != nil {
		return nil, err
	}
	if exitCondition := b.checkRequiredEtcds(condition, common.RequiredControlPlaneEtcds, etcds); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := b.checkEtcds(condition, etcds); exitCondition != nil {
		return exitCondition, nil
	}

	return nil, nil
}

// FailedCondition returns a progressing or false condition depending on the progressing threshold.
func (b *HealthChecker) FailedCondition(condition gardencorev1beta1.Condition, reason, message string, codes ...gardencorev1beta1.ErrorCode) gardencorev1beta1.Condition {
	switch condition.Status {
	case gardencorev1beta1.ConditionTrue:
		if _, ok := b.conditionThresholds[condition.Type]; !ok {
			return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionFalse, reason, message, codes...)
		}
		return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionProgressing, reason, message, codes...)

	case gardencorev1beta1.ConditionProgressing:
		threshold, ok := b.conditionThresholds[condition.Type]
		if !ok {
			return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionFalse, reason, message, codes...)
		}
		if b.lastOperation != nil && b.lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded && Now().UTC().Sub(b.lastOperation.LastUpdateTime.UTC()) <= threshold {
			return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionProgressing, reason, message, codes...)
		}
		if delta := Now().UTC().Sub(condition.LastTransitionTime.Time.UTC()); delta <= threshold {
			return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionProgressing, reason, message, codes...)
		}
		return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionFalse, reason, message, codes...)

	case gardencorev1beta1.ConditionFalse:
		threshold, ok := b.conditionThresholds[condition.Type]
		if ok &&
			((b.lastOperation != nil && b.lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded && Now().UTC().Sub(b.lastOperation.LastUpdateTime.UTC()) <= threshold) ||
				(reason != condition.Reason)) {
			return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionProgressing, reason, message, codes...)
		}
	}

	return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionFalse, reason, message, codes...)
}

// checkAPIServerAvailability checks if the API server of a Shoot cluster is reachable and measure the response time.
func (b *Botanist) checkAPIServerAvailability(ctx context.Context, checker *HealthChecker, condition gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	return health.CheckAPIServerAvailability(ctx, condition, b.K8sShootClient.RESTClient(), func(conditionType, message string) gardencorev1beta1.Condition {
		return checker.FailedCondition(condition, conditionType, message)
	}, b.Logger)
}

// CheckClusterNodes checks whether cluster nodes in the given listers are healthy and within the desired range.
// Additional checks are executed in the provider extension
func (b *HealthChecker) CheckClusterNodes(
	workers []gardencorev1beta1.Worker,
	condition gardencorev1beta1.Condition,
	nodeLister kutil.NodeLister,
) (*gardencorev1beta1.Condition, error) {

	for _, worker := range workers {
		requirement, err := labels.NewRequirement(v1beta1constants.LabelWorkerPool, selection.Equals, []string{worker.Name})
		if err != nil {
			return nil, err
		}
		nodeList, err := nodeLister.List(labels.NewSelector().Add(*requirement))
		if err != nil {
			return nil, err
		}

		if exitCondition := b.checkNodes(condition, nodeList, worker.Name); exitCondition != nil {
			return exitCondition, nil
		}

		if len(nodeList) < int(worker.Minimum) {
			c := b.FailedCondition(condition, "MissingNodes", fmt.Sprintf("Not enough worker nodes registered in worker pool '%s' to meet minimum desired machine count. (%d/%d).", worker.Name, len(nodeList), worker.Minimum))
			return &c, nil
		}
	}

	return nil, nil
}

// CheckMonitoringControlPlane checks whether the monitoring in the given listers are complete and healthy.
func (b *HealthChecker) CheckMonitoringControlPlane(
	namespace string,
	isTestingShoot bool,
	wantsAlertmanager bool,
	condition gardencorev1beta1.Condition,
	deploymentLister kutil.DeploymentLister,
	statefulSetLister kutil.StatefulSetLister,
) (*gardencorev1beta1.Condition, error) {
	if isTestingShoot {
		return nil, nil
	}

	deploymentList, err := deploymentLister.Deployments(namespace).List(monitoringSelector)
	if err != nil {
		return nil, err
	}
	if exitCondition := b.checkRequiredDeployments(condition, common.RequiredMonitoringSeedDeployments, deploymentList); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := b.checkDeployments(condition, deploymentList); exitCondition != nil {
		return exitCondition, nil
	}

	statefulSetList, err := statefulSetLister.StatefulSets(namespace).List(monitoringSelector)
	if err != nil {
		return nil, err
	}
	if exitCondition := b.checkRequiredStatefulSets(condition, computeRequiredMonitoringStatefulSets(wantsAlertmanager), statefulSetList); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := b.checkStatefulSets(condition, statefulSetList); exitCondition != nil {
		return exitCondition, nil
	}

	return nil, nil
}

// CheckLoggingControlPlane checks whether the logging components in the given listers are complete and healthy.
func (b *HealthChecker) CheckLoggingControlPlane(
	namespace string,
	isTestingShoot bool,
	condition gardencorev1beta1.Condition,
	statefulSetLister kutil.StatefulSetLister,
) (*gardencorev1beta1.Condition, error) {
	if isTestingShoot {
		return nil, nil
	}

	statefulSetList, err := statefulSetLister.StatefulSets(namespace).List(loggingSelector)
	if err != nil {
		return nil, err
	}
	if exitCondition := b.checkRequiredStatefulSets(condition, common.RequiredLoggingStatefulSets, statefulSetList); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := b.checkStatefulSets(condition, statefulSetList); exitCondition != nil {
		return exitCondition, nil
	}

	return nil, nil
}

// CheckExtensionCondition checks whether the conditions provided by extensions are healthy.
func (b *HealthChecker) CheckExtensionCondition(condition gardencorev1beta1.Condition, extensionsConditions []ExtensionCondition) *gardencorev1beta1.Condition {
	for _, cond := range extensionsConditions {
		// check if the health check condition.lastUpdateTime is older than the configured staleExtensionHealthCheckThreshold
		if b.staleExtensionHealthCheckThreshold != nil && Now().UTC().Sub(cond.Condition.LastUpdateTime.UTC()) > b.staleExtensionHealthCheckThreshold.Duration {
			c := gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionUnknown, fmt.Sprintf("%sOutdatedHealthCheckReport", cond.ExtensionType), fmt.Sprintf("%q CRD (%s/%s) reports an outdated health status (last updated: %s ago at %s).", cond.ExtensionType, cond.ExtensionNamespace, cond.ExtensionName, time.Now().UTC().Sub(cond.Condition.LastUpdateTime.UTC()).Round(time.Minute).String(), cond.Condition.LastUpdateTime.UTC().Round(time.Minute).String()))
			return &c
		}

		if cond.Condition.Status == gardencorev1beta1.ConditionProgressing {
			c := gardencorev1beta1helper.UpdatedCondition(condition, cond.Condition.Status, cond.ExtensionType+cond.Condition.Reason, cond.Condition.Message, cond.Condition.Codes...)
			return &c
		}

		if cond.Condition.Status == gardencorev1beta1.ConditionFalse || cond.Condition.Status == gardencorev1beta1.ConditionUnknown {
			c := b.FailedCondition(condition, fmt.Sprintf("%sUnhealthyReport", cond.ExtensionType), fmt.Sprintf("%q CRD (%s/%s) reports failing health check: %s", cond.ExtensionType, cond.ExtensionNamespace, cond.ExtensionName, cond.Condition.Message), cond.Condition.Codes...)
			return &c
		}
	}

	return nil
}

// checkControlPlane checks whether the control plane of the Shoot cluster is healthy.
func (b *Botanist) checkControlPlane(
	ctx context.Context,
	checker *HealthChecker,
	condition gardencorev1beta1.Condition,
	seedDeploymentLister kutil.DeploymentLister,
	seedStatefulSetLister kutil.StatefulSetLister,
	seedEtcdLister kutil.EtcdLister,
	seedWorkerLister kutil.WorkerLister,
	extensionConditions []ExtensionCondition,
) (*gardencorev1beta1.Condition, error) {
	cluster, err := gardenerextensions.GetCluster(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			c := checker.FailedCondition(condition, "ControlPlaneNotReady", "Control plane is not yet ready because of missing cluster resource.")
			return &c, nil
		}
		b.Logger.Errorf("Failed to execute control plane health checks: %v", err)
		return nil, err
	}
	// Use shoot from cluster resource here because it reflects the actual or "active" spec to determine which health checks are required.
	// With the "confineSpecUpdateRollout" feature enabled, shoot resources have a spec which is not yet active.
	shoot := cluster.Shoot
	if shoot == nil {
		return nil, errors.New("failed to execute control plane health checks because shoot is missing in cluster resource")
	}

	if exitCondition, err := checker.CheckControlPlane(shoot, b.Shoot.SeedNamespace, condition, seedDeploymentLister, seedEtcdLister, seedWorkerLister); err != nil || exitCondition != nil {
		return exitCondition, err
	}
	if exitCondition, err := checker.CheckMonitoringControlPlane(b.Shoot.SeedNamespace, b.Shoot.GetPurpose() == gardencorev1beta1.ShootPurposeTesting, b.Shoot.WantsAlertmanager, condition, seedDeploymentLister, seedStatefulSetLister); err != nil || exitCondition != nil {
		return exitCondition, err
	}
	if gardenletfeatures.FeatureGate.Enabled(features.Logging) {
		if exitCondition, err := checker.CheckLoggingControlPlane(b.Shoot.SeedNamespace, b.Shoot.GetPurpose() == gardencorev1beta1.ShootPurposeTesting, condition, seedStatefulSetLister); err != nil || exitCondition != nil {
			return exitCondition, err
		}
	}
	if exitCondition := checker.CheckExtensionCondition(condition, extensionConditions); exitCondition != nil {
		return exitCondition, nil
	}

	c := gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionTrue, "ControlPlaneRunning", "All control plane components are healthy.")
	return &c, nil
}

var versionConstraintGreaterEqual113 *semver.Constraints

func init() {
	var err error

	versionConstraintGreaterEqual113, err = semver.NewConstraint(">= 1.13")
	utilruntime.Must(err)
}

var managedResourcesShoot = sets.NewString(
	common.ManagedResourceCoreNamespaceName,
	common.ManagedResourceShootCoreName,
	common.ManagedResourceAddonsName,
)

// checkSystemComponents checks whether the system components of a Shoot are running.
func (b *Botanist) checkSystemComponents(
	ctx context.Context,
	checker *HealthChecker,
	condition gardencorev1beta1.Condition,
	extensionConditions []ExtensionCondition,
) (*gardencorev1beta1.Condition, error) {
	managedResources := managedResourcesShoot.List()
	if versionConstraintGreaterEqual113.Check(checker.gardenerVersion) {
		managedResources = append(managedResources, metricsserver.ManagedResourceName)
	}

	for _, name := range managedResources {
		mr := &resourcesv1alpha1.ManagedResource{}
		if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, name), mr); err != nil {
			return nil, err
		}

		if exitCondition := checker.CheckManagedResource(condition, mr); exitCondition != nil {
			return exitCondition, nil
		}
	}

	if exitCondition := checker.CheckExtensionCondition(condition, extensionConditions); exitCondition != nil {
		return exitCondition, nil
	}

	podsList := &corev1.PodList{}
	if err := b.K8sShootClient.Client().List(ctx, podsList, client.InNamespace(metav1.NamespaceSystem), client.MatchingLabels{"type": "tunnel"}); err != nil {
		return nil, err
	}
	if len(podsList.Items) == 0 {
		c := checker.FailedCondition(condition, "NoTunnelDeployed", "no tunnels are currently deployed to perform health-check on")
		return &c, nil
	}

	tunnelName := common.VPNTunnel
	if podsList.Items[0].Labels["app"] == common.KonnectivityTunnel {
		tunnelName = common.KonnectivityTunnel
	}

	if established, err := b.CheckTunnelConnection(ctx, logrus.NewEntry(logger.NewNopLogger()), tunnelName); err != nil || !established {
		msg := "Tunnel connection has not been established"
		if err != nil {
			msg += fmt.Sprintf(" (%+v)", err)
		}
		c := checker.FailedCondition(condition, "TunnelConnectionBroken", msg)
		return &c, nil
	}

	c := gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionTrue, "SystemComponentsRunning", "All system components are healthy.")
	return &c, nil
}

// checkClusterNodes checks whether every node registered at the Shoot cluster is in "Ready" state, that
// as many nodes are registered as desired, and that every machine is running.
func (b *Botanist) checkClusterNodes(
	checker *HealthChecker,
	condition gardencorev1beta1.Condition,
	shootNodeLister kutil.NodeLister,
	extensionConditions []ExtensionCondition,
) (*gardencorev1beta1.Condition, error) {
	if exitCondition, err := checker.CheckClusterNodes(b.Shoot.Info.Spec.Provider.Workers, condition, shootNodeLister); err != nil || exitCondition != nil {
		return exitCondition, err
	}
	if exitCondition := checker.CheckExtensionCondition(condition, extensionConditions); exitCondition != nil {
		return exitCondition, nil
	}

	c := gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionTrue, "EveryNodeReady", "Every node registered to the cluster is ready.")
	return &c, nil
}

func makeDeploymentLister(ctx context.Context, c client.Client, namespace string, selector labels.Selector) kutil.DeploymentLister {
	var (
		once  sync.Once
		items []*appsv1.Deployment
		err   error
	)

	return kutil.NewDeploymentLister(func() ([]*appsv1.Deployment, error) {
		once.Do(func() {
			list := &appsv1.DeploymentList{}
			err = c.List(ctx, list, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: selector})
			if err != nil {
				return
			}

			for _, item := range list.Items {
				it := item
				items = append(items, &it)
			}
		})
		return items, err
	})
}

func makeStatefulSetLister(ctx context.Context, c client.Client, namespace string, selector labels.Selector) kutil.StatefulSetLister {
	var (
		once  sync.Once
		items []*appsv1.StatefulSet
		err   error

		onceBody = func() {
			list := &appsv1.StatefulSetList{}
			err = c.List(ctx, list, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: selector})
			if err != nil {
				return
			}

			for _, item := range list.Items {
				it := item
				items = append(items, &it)
			}
		}
	)

	return kutil.NewStatefulSetLister(func() ([]*appsv1.StatefulSet, error) {
		once.Do(onceBody)
		return items, err
	})
}

func makeEtcdLister(ctx context.Context, c client.Client, namespace string) kutil.EtcdLister {
	var (
		once  sync.Once
		items []*druidv1alpha1.Etcd
		err   error

		onceBody = func() {
			list := &druidv1alpha1.EtcdList{}
			if err := c.List(ctx, list, client.InNamespace(namespace)); err != nil {
				return
			}

			for _, item := range list.Items {
				it := item
				items = append(items, &it)
			}
		}
	)

	return kutil.NewEtcdLister(func() ([]*druidv1alpha1.Etcd, error) {
		once.Do(onceBody)
		return items, err
	})
}

func makeNodeLister(ctx context.Context, c client.Client) kutil.NodeLister {
	var (
		once  sync.Once
		items []*corev1.Node
		err   error

		onceBody = func() {
			list := &corev1.NodeList{}
			err = c.List(ctx, list)
			if err != nil {
				return
			}

			for _, item := range list.Items {
				it := item
				items = append(items, &it)
			}
		}
	)

	return kutil.NewNodeLister(func() ([]*corev1.Node, error) {
		once.Do(onceBody)
		return items, err
	})
}

func makeWorkerLister(ctx context.Context, c client.Client, namespace string) kutil.WorkerLister {
	var (
		once  sync.Once
		items []*extensionsv1alpha1.Worker
		err   error

		onceBody = func() {
			list := &extensionsv1alpha1.WorkerList{}
			if err := c.List(ctx, list, client.InNamespace(namespace)); err != nil {
				return
			}

			for _, item := range list.Items {
				it := item
				items = append(items, &it)
			}
		}
	)

	return kutil.NewWorkerLister(func() ([]*extensionsv1alpha1.Worker, error) {
		once.Do(onceBody)
		return items, err
	})
}

func newConditionOrError(oldCondition gardencorev1beta1.Condition, newCondition *gardencorev1beta1.Condition, err error) gardencorev1beta1.Condition {
	if err != nil || newCondition == nil {
		return gardencorev1beta1helper.UpdatedConditionUnknownError(oldCondition, err)
	}
	return *newCondition
}

var (
	controlPlaneMonitoringLoggingSelector = mustGardenRoleLabelSelector(
		v1beta1constants.GardenRoleControlPlane,
		v1beta1constants.GardenRoleMonitoring,
		v1beta1constants.GardenRoleLogging,
	)
)

func (b *Botanist) healthChecks(
	ctx context.Context,
	initializeShootClients func() (bool, error),
	thresholdMappings map[gardencorev1beta1.ConditionType]time.Duration,
	healthCheckOutdatedThreshold *metav1.Duration,
	gardenerVersion *semver.Version,
	apiserverAvailability,
	controlPlane,
	nodes,
	systemComponents gardencorev1beta1.Condition,
) (
	gardencorev1beta1.Condition,
	gardencorev1beta1.Condition,
	gardencorev1beta1.Condition,
	gardencorev1beta1.Condition,
) {
	if b.Shoot.HibernationEnabled || b.Shoot.Info.Status.IsHibernated {
		return shootHibernatedCondition(apiserverAvailability), shootHibernatedCondition(controlPlane), shootHibernatedCondition(nodes), shootHibernatedCondition(systemComponents)
	}

	extensionConditionsControlPlaneHealthy, extensionConditionsEveryNodeReady, extensionConditionsSystemComponentsHealthy, err := b.getAllExtensionConditions(ctx)
	if err != nil {
		b.Logger.Errorf("error getting extension conditions: %+v", err)
	}

	var (
		checker               = NewHealthChecker(thresholdMappings, healthCheckOutdatedThreshold, b.Shoot.Info.Status.LastOperation, b.Shoot.KubernetesVersion, gardenerVersion)
		seedDeploymentLister  = makeDeploymentLister(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace, controlPlaneMonitoringLoggingSelector)
		seedStatefulSetLister = makeStatefulSetLister(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace, controlPlaneMonitoringLoggingSelector)
		seedEtcdLister        = makeEtcdLister(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace)
		seedWorkerLister      = makeWorkerLister(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace)
	)

	if apiServerRunning, err := initializeShootClients(); err != nil || !apiServerRunning {
		// don't execute health checks if API server has already been deleted or has not been created yet
		message := shootControlPlaneNotRunningMessage(b.Shoot.Info.Status.LastOperation)
		if err != nil {
			message = fmt.Sprintf("Could not initialize Shoot client for health check: %+v", err)
			b.Logger.Error(message)
		}

		apiserverAvailability = checker.FailedCondition(apiserverAvailability, "APIServerDown", "Could not reach API server during client initialization.")
		nodes = gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(nodes, message)
		systemComponents = gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(systemComponents, message)

		newControlPlane, err := b.checkControlPlane(ctx, checker, controlPlane, seedDeploymentLister, seedStatefulSetLister, seedEtcdLister, seedWorkerLister, extensionConditionsControlPlaneHealthy)
		controlPlane = newConditionOrError(controlPlane, newControlPlane, err)
		return apiserverAvailability, controlPlane, nodes, systemComponents
	}

	_ = flow.Parallel(func(ctx context.Context) error {
		apiserverAvailability = b.checkAPIServerAvailability(ctx, checker, apiserverAvailability)
		return nil
	}, func(ctx context.Context) error {
		newControlPlane, err := b.checkControlPlane(ctx, checker, controlPlane, seedDeploymentLister, seedStatefulSetLister, seedEtcdLister, seedWorkerLister, extensionConditionsControlPlaneHealthy)
		controlPlane = newConditionOrError(controlPlane, newControlPlane, err)
		return nil
	}, func(ctx context.Context) error {
		newNodes, err := b.checkClusterNodes(checker, nodes, makeNodeLister(ctx, b.K8sShootClient.Client()), extensionConditionsEveryNodeReady)
		nodes = newConditionOrError(nodes, newNodes, err)
		return nil
	}, func(ctx context.Context) error {
		newSystemComponents, err := b.checkSystemComponents(ctx, checker, systemComponents, extensionConditionsSystemComponentsHealthy)
		systemComponents = newConditionOrError(systemComponents, newSystemComponents, err)
		return nil
	})(ctx)

	return apiserverAvailability, controlPlane, nodes, systemComponents
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

// PardonCondition pardons the given condition if the Shoot is either in create (except successful create) or delete state.
func PardonCondition(condition gardencorev1beta1.Condition, lastOp *gardencorev1beta1.LastOperation, lastErrors []gardencorev1beta1.LastError) gardencorev1beta1.Condition {
	if (lastOp == nil || isUnstableLastOperation(lastOp, lastErrors)) && condition.Status == gardencorev1beta1.ConditionFalse {
		return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionProgressing, condition.Reason, condition.Message, condition.Codes...)
	}
	return condition
}

// HealthChecks conducts the health checks on all the given conditions.
func (b *Botanist) HealthChecks(
	ctx context.Context,
	initializeShootClients func() (bool, error),
	thresholdMappings map[gardencorev1beta1.ConditionType]time.Duration,
	healthCheckOutdatedThreshold *metav1.Duration,
	gardenerVersion *semver.Version,
	apiserverAvailability,
	controlPlane,
	nodes,
	systemComponents gardencorev1beta1.Condition,
) (
	gardencorev1beta1.Condition,
	gardencorev1beta1.Condition,
	gardencorev1beta1.Condition,
	gardencorev1beta1.Condition,
) {
	apiServerAvailable, controlPlaneHealthy, everyNodeReady, systemComponentsHealthy := b.healthChecks(ctx, initializeShootClients, thresholdMappings, healthCheckOutdatedThreshold, gardenerVersion, apiserverAvailability, controlPlane, nodes, systemComponents)
	lastOp := b.Shoot.Info.Status.LastOperation
	lastErrors := b.Shoot.Info.Status.LastErrors
	return PardonCondition(apiServerAvailable, lastOp, lastErrors),
		PardonCondition(controlPlaneHealthy, lastOp, lastErrors),
		PardonCondition(everyNodeReady, lastOp, lastErrors),
		PardonCondition(systemComponentsHealthy, lastOp, lastErrors)
}

// ExtensionCondition contains information about the extension type, name, namespace and the respective condition object.
type ExtensionCondition struct {
	Condition          gardencorev1beta1.Condition
	ExtensionType      string
	ExtensionName      string
	ExtensionNamespace string
}

func (b *Botanist) getAllExtensionConditions(ctx context.Context) ([]ExtensionCondition, []ExtensionCondition, []ExtensionCondition, error) {
	var (
		conditionsControlPlaneHealthy     []ExtensionCondition
		conditionsEveryNodeReady          []ExtensionCondition
		conditionsSystemComponentsHealthy []ExtensionCondition
	)

	for _, listObj := range []runtime.Object{
		&extensionsv1alpha1.BackupEntryList{},
		&extensionsv1alpha1.ContainerRuntimeList{},
		&extensionsv1alpha1.ControlPlaneList{},
		&extensionsv1alpha1.ExtensionList{},
		&extensionsv1alpha1.InfrastructureList{},
		&extensionsv1alpha1.NetworkList{},
		&extensionsv1alpha1.OperatingSystemConfigList{},
		&extensionsv1alpha1.WorkerList{},
	} {
		listKind := listObj.GetObjectKind().GroupVersionKind().Kind
		if err := b.K8sSeedClient.Client().List(ctx, listObj, client.InNamespace(b.Shoot.SeedNamespace)); err != nil {
			return nil, nil, nil, err
		}

		if err := meta.EachListItem(listObj, func(obj runtime.Object) error {
			acc, err := extensions.Accessor(obj)
			if err != nil {
				return err
			}

			kind := obj.GetObjectKind().GroupVersionKind().Kind
			name := acc.GetName()
			namespace := acc.GetNamespace()

			for _, condition := range acc.GetExtensionStatus().GetConditions() {
				switch condition.Type {
				case gardencorev1beta1.ShootControlPlaneHealthy:
					conditionsControlPlaneHealthy = append(conditionsControlPlaneHealthy, ExtensionCondition{condition, kind, name, namespace})
				case gardencorev1beta1.ShootEveryNodeReady:
					conditionsEveryNodeReady = append(conditionsEveryNodeReady, ExtensionCondition{condition, kind, name, namespace})
				case gardencorev1beta1.ShootSystemComponentsHealthy:
					conditionsSystemComponentsHealthy = append(conditionsSystemComponentsHealthy, ExtensionCondition{condition, kind, name, namespace})
				}
			}

			return nil
		}); err != nil {
			b.Logger.Errorf("Error during evaluation of kind %q for extensions health check: %+v", listKind, err)
			return nil, nil, nil, err
		}
	}

	return conditionsControlPlaneHealthy, conditionsEveryNodeReady, conditionsSystemComponentsHealthy, nil
}
