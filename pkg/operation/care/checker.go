// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"errors"
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	"github.com/Masterminds/semver"
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	requiredControlPlaneDeployments = sets.NewString(
		v1beta1constants.DeploymentNameGardenerResourceManager,
		v1beta1constants.DeploymentNameKubeAPIServer,
		v1beta1constants.DeploymentNameKubeControllerManager,
		v1beta1constants.DeploymentNameKubeScheduler,
	)

	requiredControlPlaneEtcds = sets.NewString(
		v1beta1constants.ETCDMain,
		v1beta1constants.ETCDEvents,
	)

	requiredMonitoringSeedDeployments = sets.NewString(
		v1beta1constants.DeploymentNameGrafanaOperators,
		v1beta1constants.DeploymentNameGrafanaUsers,
		v1beta1constants.DeploymentNameKubeStateMetricsShoot,
	)

	requiredLoggingStatefulSets = sets.NewString(
		v1beta1constants.StatefulSetNameLoki,
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
func NewHealthChecker(
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration,
	healthCheckOutdatedThreshold *metav1.Duration,
	lastOperation *gardencorev1beta1.LastOperation,
	kubernetesVersion *semver.Version,
	gardenerVersion *semver.Version,
) *HealthChecker {
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
			c := b.FailedCondition(condition, "DeploymentUnhealthy", fmt.Sprintf("Deployment %q is unhealthy: %v", object.Name, err.Error()))
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
				message = fmt.Sprintf("Etcd extension resource %q is unhealthy: %v", object.Name, err.Error())
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
				message = fmt.Sprintf("Node %q in worker group %q is unhealthy: %v", object.Name, workerGroupName, err)
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
	if mr.Generation != mr.Status.ObservedGeneration {
		c := b.FailedCondition(condition, gardencorev1beta1.OutdatedStatusError, fmt.Sprintf("observed generation of managed resource '%s/%s' outdated (%d/%d)", mr.Namespace, mr.Name, mr.Status.ObservedGeneration, mr.Generation))
		return &c
	}

	toProcess := map[gardencorev1beta1.ConditionType]struct{}{
		resourcesv1alpha1.ResourcesApplied: {},
		resourcesv1alpha1.ResourcesHealthy: {},
	}

	for _, cond := range mr.Status.Conditions {
		_, ok := toProcess[cond.Type]
		if !ok {
			continue
		}
		if cond.Status == gardencorev1beta1.ConditionFalse {
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

func shootHibernatedConditions(conditions []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
	hibernationConditions := make([]gardencorev1beta1.Condition, 0, len(conditions))
	for _, cond := range conditions {
		hibernationConditions = append(hibernationConditions, gardencorev1beta1helper.UpdatedCondition(cond, gardencorev1beta1.ConditionTrue, "ConditionNotChecked", "Shoot cluster has been hibernated."))
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
func computeRequiredControlPlaneDeployments(
	shoot *gardencorev1beta1.Shoot,
	workerLister kutil.WorkerLister,
) (sets.String, error) {
	shootWantsClusterAutoscaler, err := gardencorev1beta1helper.ShootWantsClusterAutoscaler(shoot)
	if err != nil {
		return nil, err
	}

	requiredControlPlaneDeployments := sets.NewString(requiredControlPlaneDeployments.UnsortedList()...)
	if shootWantsClusterAutoscaler {
		workers, err := workerLister.List(labels.Everything())
		if err != nil {
			return nil, err
		}

		// TODO: This check can be removed after few releases, as the cluster-autoscaler is now enabled even
		// during the rolling-update. Related change: https://github.com/gardener/gardener/pull/3332
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
	if exitCondition := b.checkRequiredEtcds(condition, requiredControlPlaneEtcds, etcds); exitCondition != nil {
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

	workerPoolToSecretChecksum, err := botanist.WorkerPoolToCloudConfigSecretChecksumMap(ctx, shootClient)
	if err != nil {
		return nil, err
	}

	for _, worker := range workers {
		nodes := workerPoolToNodes[worker.Name]

		kubernetesVersion, err := gardencorev1beta1helper.CalculateEffectiveKubernetesVersion(b.kubernetesVersion, worker.Kubernetes)
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

	if err := botanist.CloudConfigUpdatedForAllWorkerPools(workers, workerPoolToNodes, workerPoolToSecretChecksum); err != nil {
		c := b.FailedCondition(condition, "CloudConfigOutdated", err.Error())
		return &c, nil
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
	if exitCondition := b.checkRequiredDeployments(condition, requiredMonitoringSeedDeployments, deploymentList); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := b.checkDeployments(condition, deploymentList); exitCondition != nil {
		return exitCondition, nil
	}

	statefulSetList, err := statefulSetLister.StatefulSets(namespace).List(monitoringSelector)
	if err != nil {
		return nil, err
	}

	if wantsAlertmanager {
		if exitCondition := b.checkRequiredStatefulSets(condition, sets.NewString(v1beta1constants.StatefulSetNameAlertManager), statefulSetList); exitCondition != nil {
			return exitCondition, nil
		}
	}

	if exitCondition := b.checkStatefulSets(condition, statefulSetList); exitCondition != nil {
		return exitCondition, nil
	}

	// combined check that fails if neither a prometheus
	// statefulset nor a prometheus deployment are present
	exitConditionPrometheusDeployment := b.checkRequiredDeployments(condition, sets.NewString(v1beta1constants.DeploymentNamePrometheus), deploymentList)
	exitConditionPrometheusStatefulSet := b.checkRequiredStatefulSets(condition, sets.NewString(v1beta1constants.StatefulSetNamePrometheus), statefulSetList)
	if exitConditionPrometheusDeployment != nil && exitConditionPrometheusStatefulSet != nil {
		combinedCondition := b.FailedCondition(condition, "PrometheusResourceMissing", "Either a StatefulSet or a Deployment for Prometheus is missing")
		return &combinedCondition, nil
	}

	return nil, nil
}

// CheckLoggingControlPlane checks whether the logging components in the given listers are complete and healthy.
func (b *HealthChecker) CheckLoggingControlPlane(
	namespace string,
	isTestingShoot bool,
	lokiEnabled bool,
	condition gardencorev1beta1.Condition,
	statefulSetLister kutil.StatefulSetLister,
) (*gardencorev1beta1.Condition, error) {
	if isTestingShoot || !lokiEnabled {
		return nil, nil
	}

	statefulSetList, err := statefulSetLister.StatefulSets(namespace).List(loggingSelector)
	if err != nil {
		return nil, err
	}
	if exitCondition := b.checkRequiredStatefulSets(condition, requiredLoggingStatefulSets, statefulSetList); exitCondition != nil {
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
			c := gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionUnknown, fmt.Sprintf("%sOutdatedHealthCheckReport", cond.ExtensionType), fmt.Sprintf("%s extension (%s/%s) reports an outdated health status (last updated: %s ago at %s).", cond.ExtensionType, cond.ExtensionNamespace, cond.ExtensionName, time.Now().UTC().Sub(cond.Condition.LastUpdateTime.UTC()).Round(time.Minute).String(), cond.Condition.LastUpdateTime.UTC().Round(time.Minute).String()))
			return &c
		}

		if cond.Condition.Status == gardencorev1beta1.ConditionProgressing {
			c := gardencorev1beta1helper.UpdatedCondition(condition, cond.Condition.Status, cond.ExtensionType+cond.Condition.Reason, cond.Condition.Message, cond.Condition.Codes...)
			return &c
		}

		if cond.Condition.Status == gardencorev1beta1.ConditionFalse || cond.Condition.Status == gardencorev1beta1.ConditionUnknown {
			c := b.FailedCondition(condition, fmt.Sprintf("%sUnhealthyReport", cond.ExtensionType), fmt.Sprintf("%s extension (%s/%s) reports failing health check: %s", cond.ExtensionType, cond.ExtensionNamespace, cond.ExtensionName, cond.Condition.Message), cond.Condition.Codes...)
			return &c
		}
	}

	return nil
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

// NewConditionOrError returns the given new condition or returns an unknown error condition if an error occurred or `newCondition` is nil.
func NewConditionOrError(oldCondition gardencorev1beta1.Condition, newCondition *gardencorev1beta1.Condition, err error) gardencorev1beta1.Condition {
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
func PardonConditions(conditions []gardencorev1beta1.Condition, lastOp *gardencorev1beta1.LastOperation, lastErrors []gardencorev1beta1.LastError) []gardencorev1beta1.Condition {
	pardoningConditions := make([]gardencorev1beta1.Condition, 0, len(conditions))
	for _, cond := range conditions {
		if (lastOp == nil || isUnstableLastOperation(lastOp, lastErrors)) && cond.Status == gardencorev1beta1.ConditionFalse {
			pardoningConditions = append(pardoningConditions, gardencorev1beta1helper.UpdatedCondition(cond, gardencorev1beta1.ConditionProgressing, cond.Reason, cond.Message, cond.Codes...))
			continue
		}
		pardoningConditions = append(pardoningConditions, cond)
	}
	return pardoningConditions
}
