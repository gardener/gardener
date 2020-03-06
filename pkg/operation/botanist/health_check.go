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
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	prometheusmodel "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
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
	controlPlaneSelector    = mustGardenRoleLabelSelector(v1beta1constants.GardenRoleControlPlane)
	systemComponentSelector = mustGardenRoleLabelSelector(v1beta1constants.GardenRoleSystemComponent)
	monitoringSelector      = mustGardenRoleLabelSelector(v1beta1constants.GardenRoleMonitoring)
	optionalAddonSelector   = mustGardenRoleLabelSelector(v1beta1constants.GardenRoleOptionalAddon)
	loggingSelector         = mustGardenRoleLabelSelector(v1beta1constants.GardenRoleLogging)
)

// Now determines the current time.
var Now = time.Now

// HealthChecker contains the condition thresholds.
type HealthChecker struct {
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration
}

func (b *HealthChecker) checkRequiredDeployments(condition gardencorev1beta1.Condition, requiredNames sets.String, objects []*appsv1.Deployment) *gardencorev1beta1.Condition {
	actualNames := sets.NewString()
	for _, object := range objects {
		actualNames.Insert(object.Name)
	}

	if missingNames := requiredNames.Difference(actualNames); missingNames.Len() != 0 {
		c := b.FailedCondition(condition, "DeploymentMissing", fmt.Sprintf("Missing required deployments: %v", missingNames.List()))
		return &c
	}

	return nil
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

	if missingNames := requiredNames.Difference(actualNames); missingNames.Len() != 0 {
		c := b.FailedCondition(condition, "EtcdMissing", fmt.Sprintf("Missing required etcds: %v", missingNames.List()))
		return &c
	}

	return nil
}

func (b *HealthChecker) checkEtcds(condition gardencorev1beta1.Condition, objects []*druidv1alpha1.Etcd) *gardencorev1beta1.Condition {
	for _, object := range objects {
		if err := health.CheckEtcd(object); err != nil {
			c := b.FailedCondition(condition, "EtcdUnhealthy", fmt.Sprintf("Etcd %s is unhealthy: %v", object.Name, err.Error()))
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

	if missingNames := requiredNames.Difference(actualNames); missingNames.Len() != 0 {
		c := b.FailedCondition(condition, "StatefulSetMissing", fmt.Sprintf("Missing required stateful sets: %v", missingNames.List()))
		return &c
	}

	return nil
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
			c := b.FailedCondition(condition, "NodeUnhealthy", fmt.Sprintf("Node '%s' in worker group '%s' is unhealthy: %v", object.Name, workerGroupName, err))
			return &c
		}
	}

	return nil
}

func (b *HealthChecker) checkRequiredDaemonSets(condition gardencorev1beta1.Condition, requiredNames sets.String, objects []*appsv1.DaemonSet) *gardencorev1beta1.Condition {
	actualNames := sets.NewString()
	for _, object := range objects {
		actualNames.Insert(object.Name)
	}

	if missingNames := requiredNames.Difference(actualNames); missingNames.Len() != 0 {
		c := b.FailedCondition(condition, "DaemonSetMissing", fmt.Sprintf("Missing required daemon sets: %v", missingNames.List()))
		return &c
	}
	return nil
}

func (b *HealthChecker) checkDaemonSets(condition gardencorev1beta1.Condition, objects []*appsv1.DaemonSet) *gardencorev1beta1.Condition {
	for _, object := range objects {
		if err := health.CheckDaemonSet(object); err != nil {
			c := b.FailedCondition(condition, "DaemonSetUnhealthy", fmt.Sprintf("Daemon set %s is unhealthy: %v", object.Name, err.Error()))
			return &c
		}
	}

	return nil
}

func shootHibernatedCondition(condition gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionTrue, "ConditionNotChecked", "Shoot cluster has been hibernated.")
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
	gardenerVersion string,
	shoot *gardencorev1beta1.Shoot,
	namespace string,
	condition gardencorev1beta1.Condition,
	deploymentLister kutil.DeploymentLister,
	statefulSetLister kutil.StatefulSetLister,
	etcdLister kutil.EtcdLister,
	workerLister kutil.WorkerLister,
) (*gardencorev1beta1.Condition, error) {

	gardenerVersionLessThan120, err := versionutils.CompareVersions(gardenerVersion, "<", "v1.2.0")
	if err != nil {
		return nil, err
	}

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

	// TODO (georgekuruvillak): remove this once all shoots have been reconciled to version 1.2.0
	if gardenerVersionLessThan120 {
		statefulSets, err := statefulSetLister.StatefulSets(namespace).List(controlPlaneSelector)
		if err != nil {
			return nil, err
		}
		if exitCondition := b.checkRequiredStatefulSets(condition, common.RequiredControlPlaneEtcds, statefulSets); exitCondition != nil {
			return exitCondition, nil
		}
		if exitCondition := b.checkStatefulSets(condition, statefulSets); exitCondition != nil {
			return exitCondition, nil
		}
	} else {
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
	}

	return nil, nil
}

// CheckSystemComponents checks whether the system components in the given listers are complete and healthy.
func (b *HealthChecker) CheckSystemComponents(
	namespace string,
	condition gardencorev1beta1.Condition,
	deploymentLister kutil.DeploymentLister,
	daemonSetLister kutil.DaemonSetLister,
) (*gardencorev1beta1.Condition, error) {

	deploymentList, err := deploymentLister.Deployments(namespace).List(systemComponentSelector)
	if err != nil {
		return nil, err
	}

	if exitCondition := b.checkRequiredDeployments(condition, common.RequiredSystemComponentDeployments, deploymentList); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := b.checkDeployments(condition, deploymentList); exitCondition != nil {
		return exitCondition, nil
	}

	daemonSetList, err := daemonSetLister.DaemonSets(namespace).List(systemComponentSelector)
	if err != nil {
		return nil, err
	}

	if exitCondition := b.checkRequiredDaemonSets(condition, common.RequiredSystemComponentDaemonSets, daemonSetList); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := b.checkDaemonSets(condition, daemonSetList); exitCondition != nil {
		return exitCondition, nil
	}
	return nil, nil
}

// FailedCondition returns a progressing or false condition depending on the progressing threshold.
func (b *HealthChecker) FailedCondition(condition gardencorev1beta1.Condition, reason, message string) gardencorev1beta1.Condition {
	switch condition.Status {
	case gardencorev1beta1.ConditionTrue:
		_, ok := b.conditionThresholds[condition.Type]
		if !ok {
			return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionFalse, reason, message)
		}

		return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionProgressing, reason, message)
	case gardencorev1beta1.ConditionProgressing:
		threshold, ok := b.conditionThresholds[condition.Type]
		if !ok {
			return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionFalse, reason, message)
		}

		delta := Now().Sub(condition.LastTransitionTime.Time)
		if delta > threshold {
			return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionFalse, reason, message)
		}
		return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionProgressing, reason, message)
	}
	return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionFalse, reason, message)
}

// checkAPIServerAvailability checks if the API server of a Shoot cluster is reachable and measure the response time.
func (b *Botanist) checkAPIServerAvailability(checker *HealthChecker, condition gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	return health.CheckAPIServerAvailability(condition, b.K8sShootClient.RESTClient(), func(conditionType, message string) gardencorev1beta1.Condition {
		return checker.FailedCondition(condition, conditionType, message)
	})
}

const (
	alertStatusFiring  = "firing"
	alertStatusPending = "pending"
	alertNameLabel     = "alertname"
	alertStateLabel    = "alertstate"
)

// checkAlerts checks whether firing or pending alerts exists by querying the Shoot Prometheus.
func (b *Botanist) checkAlerts(checker *HealthChecker, condition gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	// Fetch firing and pending alerts from the Shoot cluster Prometheus.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	alertResultSet, err := b.MonitoringClient.Query(ctx, "ALERTS{alertstate=~'firing|pending'}", Now())
	if err != nil {
		return gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(condition, fmt.Sprintf("Alerts can't be queried from Shoot Prometheus (%s).", err.Error()))
	}

	var (
		firingAlerts  []string
		pendingAlerts []string
	)

	switch alertResultSet.Type() {
	case prometheusmodel.ValVector:
		resultVector := alertResultSet.(prometheusmodel.Vector)
		for _, v := range resultVector {
			switch v.Metric[alertStateLabel] {
			case alertStatusFiring:
				firingAlerts = append(firingAlerts, string(v.Metric[alertNameLabel]))
			case alertStatusPending:
				pendingAlerts = append(pendingAlerts, string(v.Metric[alertNameLabel]))
			}
		}
	default:
		return gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(condition, "Unexpected metrics format. Can't determine alerts.")
	}

	// Validate the alert results and update the conditions accordingly.
	var (
		message strings.Builder
		reason  string
		failed  bool
	)

	if len(firingAlerts) > 0 {
		reason = "FiringAlertsActive"
		failed = true
		message.WriteString(fmt.Sprintf("The following alerts are active: %v", strings.Join(firingAlerts, ", ")))
		if len(pendingAlerts) > 0 {
			reason = "FiringAndPendingAlertsActive"
		}
	} else {
		reason = "NoAlertsActive"
		failed = false
		message.WriteString("No active alerts")
		if len(pendingAlerts) > 0 {
			reason = "PendingAlertsActive"
		}
	}
	if len(pendingAlerts) > 0 {
		message.WriteString(fmt.Sprintf(". The following alerts might trigger soon: %v", strings.Join(pendingAlerts, ", ")))
	}
	if failed {
		return checker.FailedCondition(condition, reason, message.String())
	}
	return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionTrue, reason, message.String())
}

// CheckClusterNodes checks whether cluster nodes in the given listers are healthy and within the desired range.
// Additional checks are executed in the provider extension
func (b *HealthChecker) CheckClusterNodes(
	workers []gardencorev1beta1.Worker,
	condition gardencorev1beta1.Condition,
	nodeLister kutil.NodeLister,
) (*gardencorev1beta1.Condition, error) {

	for _, worker := range workers {
		requirement, err := labels.NewRequirement("worker.gardener.cloud/pool", selection.Equals, []string{worker.Name})
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
		} else if len(nodeList) > int(worker.Maximum) {
			c := b.FailedCondition(condition, "TooManyNodes", fmt.Sprintf("Too many worker nodes registered in worker pool '%s' - exceeds maximum desired machine count. (%d/%d).", worker.Name, len(nodeList), worker.Maximum))
			return &c, nil
		}
	}

	return nil, nil
}

// CheckMonitoringSystemComponents checks whether the monitoring components in the given listers are complete and healthy.
func (b *HealthChecker) CheckMonitoringSystemComponents(
	namespace string,
	isTestingShoot bool,
	condition gardencorev1beta1.Condition,
	deploymentLister kutil.DeploymentLister,
	daemonSetLister kutil.DaemonSetLister,
) (*gardencorev1beta1.Condition, error) {

	if isTestingShoot {
		return nil, nil
	}

	deploymentList, err := deploymentLister.Deployments(namespace).List(monitoringSelector)
	if err != nil {
		return nil, err
	}
	if exitCondition := b.checkRequiredDeployments(condition, common.RequiredMonitoringShootDeployments, deploymentList); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := b.checkDeployments(condition, deploymentList); exitCondition != nil {
		return exitCondition, nil
	}

	daemonSetList, err := daemonSetLister.DaemonSets(namespace).List(monitoringSelector)
	if err != nil {
		return nil, err
	}

	if exitCondition := b.checkRequiredDaemonSets(condition, common.RequiredMonitoringShootDaemonSets, daemonSetList); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := b.checkDaemonSets(condition, daemonSetList); exitCondition != nil {
		return exitCondition, nil
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

// CheckOptionalAddonsSystemComponents checks whether the addons in the given listers are healthy.
func (b *HealthChecker) CheckOptionalAddonsSystemComponents(
	namespace string,
	condition gardencorev1beta1.Condition,
	deploymentLister kutil.DeploymentLister,
	daemonSetLister kutil.DaemonSetLister,
) (*gardencorev1beta1.Condition, error) {

	deploymentList, err := deploymentLister.Deployments(namespace).List(optionalAddonSelector)
	if err != nil {
		return nil, err
	}

	if exitCondition := b.checkDeployments(condition, deploymentList); exitCondition != nil {
		return exitCondition, nil
	}

	daemonSetList, err := daemonSetLister.DaemonSets(namespace).List(optionalAddonSelector)
	if err != nil {
		return nil, err
	}

	if exitCondition := b.checkDaemonSets(condition, daemonSetList); exitCondition != nil {
		return exitCondition, nil
	}
	return nil, nil
}

// CheckLoggingControlPlane checks whether the logging components in the given listers are complete and healthy.
func (b *HealthChecker) CheckLoggingControlPlane(
	namespace string,
	isTestingShoot bool,
	condition gardencorev1beta1.Condition,
	deploymentLister kutil.DeploymentLister,
	statefulSetLister kutil.StatefulSetLister,
) (*gardencorev1beta1.Condition, error) {

	if isTestingShoot {
		return nil, nil
	}

	deploymentList, err := deploymentLister.Deployments(namespace).List(loggingSelector)
	if err != nil {
		return nil, err
	}

	if exitCondition := b.checkRequiredDeployments(condition, common.RequiredLoggingDeployments, deploymentList); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := b.checkDeployments(condition, deploymentList); exitCondition != nil {
		return exitCondition, nil
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
func (b *HealthChecker) CheckExtensionCondition(condition gardencorev1beta1.Condition, extensionsCondition []extensionCondition) *gardencorev1beta1.Condition {
	for _, cond := range extensionsCondition {
		if cond.condition.Status == gardencorev1beta1.ConditionFalse || cond.condition.Status == gardencorev1beta1.ConditionUnknown {
			c := b.FailedCondition(condition, fmt.Sprintf("%sUnhealthyReport", cond.extensionType), cond.condition.Message)
			return &c
		}
	}
	return nil
}

// checkControlPlane checks whether the control plane of the Shoot cluster is healthy.
func (b *Botanist) checkControlPlane(
	checker *HealthChecker,
	condition gardencorev1beta1.Condition,
	seedDeploymentLister kutil.DeploymentLister,
	seedStatefulSetLister kutil.StatefulSetLister,
	seedEtcdLister kutil.EtcdLister,
	seedWorkerLister kutil.WorkerLister,
	extensionConditions []extensionCondition,
) (*gardencorev1beta1.Condition, error) {

	if exitCondition, err := checker.CheckControlPlane(b.Shoot.Info.Status.Gardener.Version, b.Shoot.Info, b.Shoot.SeedNamespace, condition, seedDeploymentLister, seedStatefulSetLister, seedEtcdLister, seedWorkerLister); err != nil || exitCondition != nil {
		return exitCondition, err
	}
	if exitCondition, err := checker.CheckMonitoringControlPlane(b.Shoot.SeedNamespace, b.Shoot.GetPurpose() == gardencorev1beta1.ShootPurposeTesting, b.Shoot.WantsAlertmanager, condition, seedDeploymentLister, seedStatefulSetLister); err != nil || exitCondition != nil {
		return exitCondition, err
	}
	if gardenletfeatures.FeatureGate.Enabled(features.Logging) {
		if exitCondition, err := checker.CheckLoggingControlPlane(b.Shoot.SeedNamespace, b.Shoot.GetPurpose() == gardencorev1beta1.ShootPurposeTesting, condition, seedDeploymentLister, seedStatefulSetLister); err != nil || exitCondition != nil {
			return exitCondition, err
		}
	}
	if exitCondition := checker.CheckExtensionCondition(condition, extensionConditions); exitCondition != nil {
		return exitCondition, nil
	}

	c := gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionTrue, "ControlPlaneRunning", "All control plane components are healthy.")
	return &c, nil
}

// checkSystemComponents checks whether the system components of a Shoot are running.
func (b *Botanist) checkSystemComponents(
	ctx context.Context,
	checker *HealthChecker,
	condition gardencorev1beta1.Condition,
	shootDeploymentLister kutil.DeploymentLister,
	shootDaemonSetLister kutil.DaemonSetLister,
	extensionConditions []extensionCondition,
) (*gardencorev1beta1.Condition, error) {

	if exitCondition, err := checker.CheckSystemComponents(metav1.NamespaceSystem, condition, shootDeploymentLister, shootDaemonSetLister); err != nil || exitCondition != nil {
		return exitCondition, err
	}
	if exitCondition, err := checker.CheckMonitoringSystemComponents(metav1.NamespaceSystem, b.Shoot.GetPurpose() == gardencorev1beta1.ShootPurposeTesting, condition, shootDeploymentLister, shootDaemonSetLister); err != nil || exitCondition != nil {
		return exitCondition, err
	}
	if exitCondition, err := checker.CheckOptionalAddonsSystemComponents(metav1.NamespaceSystem, condition, shootDeploymentLister, shootDaemonSetLister); err != nil || exitCondition != nil {
		return exitCondition, err
	}
	if exitCondition := checker.CheckExtensionCondition(condition, extensionConditions); exitCondition != nil {
		return exitCondition, nil
	}
	if established, err := b.CheckVPNConnection(ctx, logrus.NewEntry(logger.NewNopLogger())); err != nil || !established {
		msg := "VPN connection has not been established"
		if err != nil {
			msg += fmt.Sprintf(" (%+v)", err)
		}
		c := checker.FailedCondition(condition, "VPNConnectionBroken", msg)
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
	extensionConditions []extensionCondition,
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

func makeDeploymentLister(clientset kubernetes.Interface, namespace string, options metav1.ListOptions) kutil.DeploymentLister {
	var (
		once  sync.Once
		items []*appsv1.Deployment
		err   error
	)

	return kutil.NewDeploymentLister(func() ([]*appsv1.Deployment, error) {
		once.Do(func() {
			var list *appsv1.DeploymentList
			list, err = clientset.AppsV1().Deployments(namespace).List(options)
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

func makeStatefulSetLister(clientset kubernetes.Interface, namespace string, options metav1.ListOptions) kutil.StatefulSetLister {
	var (
		once  sync.Once
		items []*appsv1.StatefulSet
		err   error

		onceBody = func() {
			var list *appsv1.StatefulSetList
			list, err = clientset.AppsV1().StatefulSets(namespace).List(options)
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

func makeEtcdLister(c client.Client, namespace string) kutil.EtcdLister {
	var (
		once  sync.Once
		items []*druidv1alpha1.Etcd
		err   error

		onceBody = func() {
			list := &druidv1alpha1.EtcdList{}
			if err := c.List(context.TODO(), list, client.InNamespace(namespace)); err != nil {
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

func makeDaemonSetLister(clientset kubernetes.Interface, namespace string, options metav1.ListOptions) kutil.DaemonSetLister {
	var (
		once  sync.Once
		items []*appsv1.DaemonSet
		err   error

		onceBody = func() {
			var list *appsv1.DaemonSetList
			list, err = clientset.AppsV1().DaemonSets(namespace).List(options)
			if err != nil {
				return
			}

			for _, item := range list.Items {
				it := item
				items = append(items, &it)
			}
		}
	)

	return kutil.NewDaemonSetLister(func() ([]*appsv1.DaemonSet, error) {
		once.Do(onceBody)
		return items, err
	})
}

func makeNodeLister(clientset kubernetes.Interface, options metav1.ListOptions) kutil.NodeLister {
	var (
		once  sync.Once
		items []*corev1.Node
		err   error

		onceBody = func() {
			var list *corev1.NodeList
			list, err = clientset.CoreV1().Nodes().List(options)
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

func makeWorkerLister(c client.Client, namespace string) kutil.WorkerLister {
	var (
		once  sync.Once
		items []*extensionsv1alpha1.Worker
		err   error

		onceBody = func() {
			list := &extensionsv1alpha1.WorkerList{}
			if err := c.List(context.TODO(), list, client.InNamespace(namespace)); err != nil {
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
	systemComponentsOptionalAddonsMonitoringSelector = mustGardenRoleLabelSelector(
		v1beta1constants.GardenRoleSystemComponent,
		v1beta1constants.GardenRoleOptionalAddon,
		v1beta1constants.GardenRoleMonitoring,
	)

	seedDeploymentListOptions  = metav1.ListOptions{LabelSelector: controlPlaneMonitoringLoggingSelector.String()}
	seedStatefulSetListOptions = metav1.ListOptions{LabelSelector: controlPlaneMonitoringLoggingSelector.String()}

	shootDeploymentListOptions = metav1.ListOptions{LabelSelector: systemComponentsOptionalAddonsMonitoringSelector.String()}
	shootDaemonSetListOptions  = metav1.ListOptions{LabelSelector: systemComponentsOptionalAddonsMonitoringSelector.String()}
	shootNodeListOptions       = metav1.ListOptions{}
)

// NewHealthChecker creates a new health checker.
func NewHealthChecker(conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration) *HealthChecker {
	return &HealthChecker{
		conditionThresholds: conditionThresholds,
	}
}

func (b *Botanist) healthChecks(initializeShootClients func() error, thresholdMappings map[gardencorev1beta1.ConditionType]time.Duration, apiserverAvailability, controlPlane, nodes, systemComponents gardencorev1beta1.Condition) (gardencorev1beta1.Condition, gardencorev1beta1.Condition, gardencorev1beta1.Condition, gardencorev1beta1.Condition) {
	if b.Shoot.HibernationEnabled || b.Shoot.Info.Status.IsHibernated {
		return shootHibernatedCondition(apiserverAvailability), shootHibernatedCondition(controlPlane), shootHibernatedCondition(nodes), shootHibernatedCondition(systemComponents)
	}

	var (
		seedDeploymentLister  = makeDeploymentLister(b.K8sSeedClient.Kubernetes(), b.Shoot.SeedNamespace, seedDeploymentListOptions)
		seedStatefulSetLister = makeStatefulSetLister(b.K8sSeedClient.Kubernetes(), b.Shoot.SeedNamespace, seedStatefulSetListOptions)
		seedEtcdLister        = makeEtcdLister(b.K8sSeedClient.Client(), b.Shoot.SeedNamespace)
		seedWorkerLister      = makeWorkerLister(b.K8sSeedClient.Client(), b.Shoot.SeedNamespace)

		checker = NewHealthChecker(thresholdMappings)
	)

	extensionConditionsControlPlaneHealthy, extensionConditionsEveryNodeReady, extensionConditionsSystemComponentsHealthy, err := b.getAllExtensionConditions(context.TODO())
	if err != nil {
		b.Logger.Errorf("error getting extension conditions: %+v", err)
	}
	if err := initializeShootClients(); err != nil {
		message := fmt.Sprintf("Could not initialize Shoot client for health check: %+v", err)
		b.Logger.Error(message)
		apiserverAvailability = checker.FailedCondition(apiserverAvailability, "APIServerDown", "Could not reach API server during client initialization.")
		nodes = gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(nodes, message)
		systemComponents = gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(systemComponents, message)

		newControlPlane, err := b.checkControlPlane(checker, controlPlane, seedDeploymentLister, seedStatefulSetLister, seedEtcdLister, seedWorkerLister, extensionConditionsControlPlaneHealthy)
		controlPlane = newConditionOrError(controlPlane, newControlPlane, err)
		return apiserverAvailability, controlPlane, nodes, systemComponents
	}

	var (
		wg sync.WaitGroup

		shootDeploymentLister = makeDeploymentLister(b.K8sShootClient.Kubernetes(), metav1.NamespaceSystem, shootDeploymentListOptions)
		shootDaemonSetLister  = makeDaemonSetLister(b.K8sShootClient.Kubernetes(), metav1.NamespaceSystem, shootDaemonSetListOptions)
		shootNodeLister       = makeNodeLister(b.K8sShootClient.Kubernetes(), shootNodeListOptions)
	)

	wg.Add(4)
	go func() {
		defer wg.Done()
		apiserverAvailability = b.checkAPIServerAvailability(checker, apiserverAvailability)
	}()
	go func() {
		defer wg.Done()
		newControlPlane, err := b.checkControlPlane(checker, controlPlane, seedDeploymentLister, seedStatefulSetLister, seedEtcdLister, seedWorkerLister, extensionConditionsControlPlaneHealthy)
		controlPlane = newConditionOrError(controlPlane, newControlPlane, err)
	}()
	go func() {
		defer wg.Done()
		newNodes, err := b.checkClusterNodes(checker, nodes, shootNodeLister, extensionConditionsEveryNodeReady)
		nodes = newConditionOrError(nodes, newNodes, err)
	}()
	go func() {
		defer wg.Done()
		newSystemComponents, err := b.checkSystemComponents(context.TODO(), checker, systemComponents, shootDeploymentLister, shootDaemonSetLister, extensionConditionsSystemComponentsHealthy)
		systemComponents = newConditionOrError(systemComponents, newSystemComponents, err)
	}()
	wg.Wait()

	return apiserverAvailability, controlPlane, nodes, systemComponents
}

var unstableOperationTypes = map[gardencorev1beta1.LastOperationType]struct{}{
	gardencorev1beta1.LastOperationTypeCreate: {},
	gardencorev1beta1.LastOperationTypeDelete: {},
}

func isUnstableOperationType(lastOperationType gardencorev1beta1.LastOperationType) bool {
	_, ok := unstableOperationTypes[lastOperationType]
	return ok
}

// pardonCondition pardons the given condition if there was no last error and the Shoot is either
// in create or delete state.
func (b *Botanist) pardonCondition(condition gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	shoot := b.Shoot.Info
	if len(shoot.Status.LastErrors) > 0 {
		return condition
	}
	if lastOp := shoot.Status.LastOperation; (lastOp == nil || (lastOp != nil && isUnstableOperationType(lastOp.Type))) && condition.Status == gardencorev1beta1.ConditionFalse {
		return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionProgressing, condition.Reason, condition.Message)
	}
	return condition
}

// HealthChecks conducts the health checks on all the given conditions.
func (b *Botanist) HealthChecks(initializeShootClients func() error, thresholdMappings map[gardencorev1beta1.ConditionType]time.Duration, apiserverAvailability, controlPlane, nodes, systemComponents gardencorev1beta1.Condition) (gardencorev1beta1.Condition, gardencorev1beta1.Condition, gardencorev1beta1.Condition, gardencorev1beta1.Condition) {
	apiServerAvailable, controlPlaneHealthy, everyNodeReady, systemComponentsHealthy := b.healthChecks(initializeShootClients, thresholdMappings, apiserverAvailability, controlPlane, nodes, systemComponents)
	return b.pardonCondition(apiServerAvailable), b.pardonCondition(controlPlaneHealthy), b.pardonCondition(everyNodeReady), b.pardonCondition(systemComponentsHealthy)
}

// MonitoringHealthChecks performs the monitoring related health checks.
func (b *Botanist) MonitoringHealthChecks(checker *HealthChecker, inactiveAlerts gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	if b.Shoot.HibernationEnabled {
		return shootHibernatedCondition(inactiveAlerts)
	}
	if err := b.InitializeMonitoringClient(); err != nil {
		message := fmt.Sprintf("Could not initialize Shoot monitoring API client for health check: %+v", err)
		b.Logger.Error(message)
		return gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(inactiveAlerts, message)
	}
	return b.checkAlerts(checker, inactiveAlerts)
}

type extensionCondition struct {
	condition     gardencorev1beta1.Condition
	extensionType string
	extensionName string
}

func (b *Botanist) getAllExtensionConditions(ctx context.Context) ([]extensionCondition, []extensionCondition, []extensionCondition, error) {
	var (
		conditionsControlPlaneHealthy     []extensionCondition
		conditionsEveryNodeReady          []extensionCondition
		conditionsSystemComponentsHealthy []extensionCondition
	)

	for _, listObj := range []runtime.Object{
		&extensionsv1alpha1.BackupEntryList{},
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

			for _, condition := range acc.GetExtensionStatus().GetConditions() {
				switch condition.Type {
				case gardencorev1beta1.ShootControlPlaneHealthy:
					conditionsControlPlaneHealthy = append(conditionsControlPlaneHealthy, extensionCondition{condition, kind, name})
				case gardencorev1beta1.ShootEveryNodeReady:
					conditionsEveryNodeReady = append(conditionsEveryNodeReady, extensionCondition{condition, kind, name})
				case gardencorev1beta1.ShootSystemComponentsHealthy:
					conditionsSystemComponentsHealthy = append(conditionsSystemComponentsHealthy, extensionCondition{condition, kind, name})
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
