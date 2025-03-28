// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package checker

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/Masterminds/semver/v3"
	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
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
	loggingSelector      = mustGardenRoleLabelSelector(v1beta1constants.GardenRoleLogging)
)

// HealthChecker contains the condition thresholds.
type HealthChecker struct {
	reader              client.Reader
	clock               clock.Clock
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration
	lastOperation       *gardencorev1beta1.LastOperation
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(
	reader client.Reader,
	clock clock.Clock,
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration,
	lastOperation *gardencorev1beta1.LastOperation,
) *HealthChecker {
	return &HealthChecker{
		reader:              reader,
		clock:               clock,
		conditionThresholds: conditionThresholds,
		lastOperation:       lastOperation,
	}
}

func (h *HealthChecker) checkRequiredResourceNames(condition gardencorev1beta1.Condition, requiredNames, names sets.Set[string], reason, message string) *gardencorev1beta1.Condition {
	if missingNames := requiredNames.Difference(names); missingNames.Len() != 0 {
		c := v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, reason, fmt.Sprintf("%s: %v", message, sets.List(missingNames)))
		return &c
	}

	return nil
}

func (h *HealthChecker) checkRequiredDeployments(condition gardencorev1beta1.Condition, requiredNames sets.Set[string], objects []appsv1.Deployment) *gardencorev1beta1.Condition {
	actualNames := sets.New[string]()
	for _, object := range objects {
		actualNames.Insert(object.Name)
	}

	return h.checkRequiredResourceNames(condition, requiredNames, actualNames, "DeploymentMissing", "Missing required deployments")
}

func (h *HealthChecker) checkDeployments(condition gardencorev1beta1.Condition, objects []appsv1.Deployment) *gardencorev1beta1.Condition {
	for _, object := range objects {
		if err := health.CheckDeployment(&object); err != nil {
			c := v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, "DeploymentUnhealthy", fmt.Sprintf("Deployment %q is unhealthy: %v", object.Name, err.Error()))
			return &c
		}
	}

	return nil
}

func (h *HealthChecker) checkRequiredEtcds(condition gardencorev1beta1.Condition, requiredNames sets.Set[string], objects []druidcorev1alpha1.Etcd) *gardencorev1beta1.Condition {
	actualNames := sets.New[string]()
	for _, object := range objects {
		actualNames.Insert(object.Name)
	}

	return h.checkRequiredResourceNames(condition, requiredNames, actualNames, "EtcdMissing", "Missing required etcds")
}

func (h *HealthChecker) checkEtcds(condition gardencorev1beta1.Condition, objects []druidcorev1alpha1.Etcd) *gardencorev1beta1.Condition {
	for _, object := range objects {
		if err := health.CheckEtcd(&object); err != nil {
			var (
				message = fmt.Sprintf("Etcd extension resource %q is unhealthy: %v", object.Name, err.Error())
				codes   []gardencorev1beta1.ErrorCode
			)

			if len(object.Status.LastErrors) != 0 {
				message = fmt.Sprintf("%s (%+v)", message, object.Status.LastErrors)
			}

			c := v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, "EtcdUnhealthy", message, codes...)
			return &c
		}
	}

	return nil
}

// kubeletConfigProblemRegex is used to check if an error occurred due to a kubelet configuration problem.
var kubeletConfigProblemRegex = regexp.MustCompile(`(?i)(KubeletHasInsufficientMemory|KubeletHasDiskPressure|KubeletHasInsufficientPID)`)

// CheckNodes whether the given nodes are ready and the version in the node status is of the same major-minor as given in 'workerGroupKubernetesVersion'.
func (h *HealthChecker) CheckNodes(condition gardencorev1beta1.Condition, nodes []corev1.Node, workerGroupName string, workerGroupKubernetesVersion *semver.Version) *gardencorev1beta1.Condition {
	for _, object := range nodes {
		if err := health.CheckNode(&object); err != nil {
			var (
				errorCodes []gardencorev1beta1.ErrorCode
				message    = fmt.Sprintf("Node %q in worker group %q is unhealthy: %v", object.Name, workerGroupName, err)
			)

			if kubeletConfigProblemRegex.MatchString(err.Error()) {
				errorCodes = append(errorCodes, gardencorev1beta1.ErrorConfigurationProblem)
			}

			c := v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, "NodeUnhealthy", message, errorCodes...)
			return &c
		}

		sameMajorMinor, err := semver.NewConstraint("~ " + object.Status.NodeInfo.KubeletVersion)
		if err != nil {
			c := v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, "VersionParseError", fmt.Sprintf("Error checking for same major minor Kubernetes version for node %q: %+v", object.Name, err))
			return &c
		}
		if sameMajorMinor.Check(workerGroupKubernetesVersion) {
			equal, err := semver.NewConstraint("= " + object.Status.NodeInfo.KubeletVersion)
			if err != nil {
				c := v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, "VersionParseError", fmt.Sprintf("Error checking for equal Kubernetes versions for node %q: %+v", object.Name, err))
				return &c
			}

			if !equal.Check(workerGroupKubernetesVersion) {
				c := v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, "KubeletVersionMismatch", fmt.Sprintf("The kubelet version for node %q (%s) does not match the desired Kubernetes version (v%s)", object.Name, object.Status.NodeInfo.KubeletVersion, workerGroupKubernetesVersion.Original()))
				return &c
			}
		}
	}

	return nil
}

// defaultSuccessfulCheck returns a function that checks whether the condition status is successful.
func defaultSuccessfulCheck() func(condition gardencorev1beta1.Condition) bool {
	return func(condition gardencorev1beta1.Condition) bool {
		return condition.Status != gardencorev1beta1.ConditionFalse && condition.Status != gardencorev1beta1.ConditionUnknown
	}
}

// resourcesNotProgressingCheck returns a function that checks a condition is not progressing.
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

// CheckManagedResources checks multiple ManagedResources in case the provided filter func returns true. If their state
// indicates issues then this is reflected in the state of the provided condition. If there are no issues, nil is
// returned.
func (h *HealthChecker) CheckManagedResources(
	condition gardencorev1beta1.Condition,
	managedResources []resourcesv1alpha1.ManagedResource,
	filterFunc func(resourcesv1alpha1.ManagedResource) bool,
	progressingThreshold *metav1.Duration,
) *gardencorev1beta1.Condition {
	for _, managedResource := range managedResources {
		if !filterFunc(managedResource) {
			continue
		}

		if exitCondition := h.CheckManagedResource(condition, &managedResource, progressingThreshold); exitCondition != nil {
			return exitCondition
		}
	}

	return nil
}

// CheckManagedResource checks the conditions of the given managed resource and reflects the state in the returned condition.
func (h *HealthChecker) CheckManagedResource(condition gardencorev1beta1.Condition, mr *resourcesv1alpha1.ManagedResource, managedResourceProgressingThreshold *metav1.Duration) *gardencorev1beta1.Condition {
	conditionsToCheck := map[gardencorev1beta1.ConditionType]func(condition gardencorev1beta1.Condition) bool{
		resourcesv1alpha1.ResourcesApplied:     defaultSuccessfulCheck(),
		resourcesv1alpha1.ResourcesHealthy:     defaultSuccessfulCheck(),
		resourcesv1alpha1.ResourcesProgressing: resourcesNotProgressingCheck(h.clock, managedResourceProgressingThreshold),
	}

	return h.checkManagedResourceConditions(condition, mr, conditionsToCheck, managedResourceProgressingThreshold)
}

// checkManagedResourceConditions checks the given conditions at the ManagedResource.
func (h *HealthChecker) checkManagedResourceConditions(
	condition gardencorev1beta1.Condition,
	mr *resourcesv1alpha1.ManagedResource,
	conditionsToCheck map[gardencorev1beta1.ConditionType]func(condition gardencorev1beta1.Condition) bool,
	managedResourceProgressingThreshold *metav1.Duration,
) *gardencorev1beta1.Condition {
	if mr.Generation != mr.Status.ObservedGeneration {
		c := v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, gardencorev1beta1.OutdatedStatusError, fmt.Sprintf("observed generation of managed resource '%s/%s' outdated (%d/%d)", mr.Namespace, mr.Name, mr.Status.ObservedGeneration, mr.Generation))

		// check if MangedResource `ResourcesApplied` condition is in failed state
		conditionResourcesApplied := v1beta1helper.GetCondition(mr.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
		if conditionResourcesApplied != nil && conditionResourcesApplied.Status == gardencorev1beta1.ConditionFalse && conditionResourcesApplied.Reason == resourcesv1alpha1.ConditionApplyFailed {
			c = v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, conditionResourcesApplied.Reason, conditionResourcesApplied.Message)
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
			c := v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, cond.Reason, cond.Message)
			if cond.Type == resourcesv1alpha1.ResourcesProgressing && managedResourceProgressingThreshold != nil {
				c = v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, gardencorev1beta1.ManagedResourceProgressingRolloutStuck, fmt.Sprintf("ManagedResource %s is progressing for more than %s", mr.Name, managedResourceProgressingThreshold.Duration))
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
		c := v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, gardencorev1beta1.ManagedResourceMissingConditionError, fmt.Sprintf("ManagedResource %s is missing the following condition(s), %v", mr.Name, missing))
		return &c
	}

	return nil
}

// CheckControlPlane checks whether the given required control-plane component deployments and ETCDs are complete and healthy.
func (h *HealthChecker) CheckControlPlane(
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
	if err := h.reader.List(ctx, deploymentList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: controlPlaneSelector}); err != nil {
		return nil, err
	}

	etcdList := &druidcorev1alpha1.EtcdList{}
	if err := h.reader.List(ctx, etcdList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: controlPlaneSelector}); err != nil {
		return nil, err
	}

	if exitCondition := h.checkRequiredDeployments(condition, requiredControlPlaneDeployments, deploymentList.Items); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := h.checkDeployments(condition, deploymentList.Items); exitCondition != nil {
		return exitCondition, nil
	}

	if exitCondition := h.checkRequiredEtcds(condition, requiredControlPlaneEtcds, etcdList.Items); exitCondition != nil {
		return exitCondition, nil
	}
	if exitCondition := h.checkEtcds(condition, etcdList.Items); exitCondition != nil {
		return exitCondition, nil
	}

	return nil, nil
}

// CheckMonitoringControlPlane checks the monitoring components of the control-plane.
func (h *HealthChecker) CheckMonitoringControlPlane(
	ctx context.Context,
	namespace string,
	requiredMonitoringDeployments sets.Set[string],
	appsSelector labels.Selector,
	condition gardencorev1beta1.Condition,
) (
	*gardencorev1beta1.Condition,
	error,
) {
	deploymentList := &appsv1.DeploymentList{}
	if err := h.reader.List(ctx, deploymentList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: appsSelector}); err != nil {
		return nil, err
	}

	if exitCondition := h.checkRequiredDeployments(condition, requiredMonitoringDeployments, deploymentList.Items); exitCondition != nil {
		return exitCondition, nil
	}

	if exitCondition := h.checkDeployments(condition, deploymentList.Items); exitCondition != nil {
		return exitCondition, nil
	}

	return nil, nil
}

var requiredLoggingDeployments = sets.New(v1beta1constants.DeploymentNameEventLogger)

// CheckLoggingControlPlane checks whether the logging components are complete and healthy.
func (h *HealthChecker) CheckLoggingControlPlane(
	ctx context.Context,
	namespace string,
	eventLoggingEnabled bool,
	condition gardencorev1beta1.Condition,
) (
	*gardencorev1beta1.Condition,
	error,
) {
	if eventLoggingEnabled {
		deploymentList := &appsv1.DeploymentList{}
		if err := h.reader.List(ctx, deploymentList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: loggingSelector}); err != nil {
			return nil, err
		}

		if exitCondition := h.checkRequiredDeployments(condition, requiredLoggingDeployments, deploymentList.Items); exitCondition != nil {
			return exitCondition, nil
		}
		if exitCondition := h.checkDeployments(condition, deploymentList.Items); exitCondition != nil {
			return exitCondition, nil
		}
	}

	return nil, nil
}

// CheckExtensionCondition checks whether the conditions provided by extensions are healthy.
func (h *HealthChecker) CheckExtensionCondition(condition gardencorev1beta1.Condition, extensionsConditions []ExtensionCondition, staleExtensionHealthCheckThreshold *metav1.Duration) *gardencorev1beta1.Condition {
	for _, cond := range extensionsConditions {
		// check if the extension controller's last heartbeat time or the condition's LastUpdateTime is older than the configured staleExtensionHealthCheckThreshold
		if staleExtensionHealthCheckThreshold != nil {
			lastHeartbeatTime := cond.LastHeartbeatTime
			if lastHeartbeatTime == nil {
				lastHeartbeatTime = &metav1.MicroTime{}
			}
			if h.clock.Now().UTC().Sub(lastHeartbeatTime.UTC()) > staleExtensionHealthCheckThreshold.Duration {
				c := v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionUnknown, fmt.Sprintf("%sOutdatedHealthCheckReport", cond.ExtensionType), fmt.Sprintf("%s extension (%s/%s) reports an outdated health status (last updated: %s ago at %s).", cond.ExtensionType, cond.ExtensionNamespace, cond.ExtensionName, h.clock.Now().UTC().Sub(lastHeartbeatTime.UTC()).Round(time.Minute).String(), lastHeartbeatTime.UTC().Round(time.Minute).String()))
				return &c
			}
		}

		if cond.Condition.Status == gardencorev1beta1.ConditionProgressing {
			c := v1beta1helper.UpdatedConditionWithClock(h.clock, condition, cond.Condition.Status, cond.ExtensionType+cond.Condition.Reason, cond.Condition.Message, cond.Condition.Codes...)
			return &c
		}

		if cond.Condition.Status == gardencorev1beta1.ConditionFalse || cond.Condition.Status == gardencorev1beta1.ConditionUnknown {
			c := v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, fmt.Sprintf("%sUnhealthyReport", cond.ExtensionType), fmt.Sprintf("%s extension (%s/%s) reports failing health check: %s", cond.ExtensionType, cond.ExtensionNamespace, cond.ExtensionName, cond.Condition.Message), cond.Condition.Codes...)
			return &c
		}
	}

	return nil
}

// ExtensionCondition contains information about the extension type, name, namespace and the respective condition object.
type ExtensionCondition struct {
	Condition          gardencorev1beta1.Condition
	ExtensionType      string
	ExtensionName      string
	ExtensionNamespace string
	LastHeartbeatTime  *metav1.MicroTime
}

// CheckControllerInstallations checks multiple controller installation in case the provided filter func returns true. If their state
// indicates issues then this is reflected in the state of the provided condition. If there are no issues, nil is returned.
func (h *HealthChecker) CheckControllerInstallations(
	ctx context.Context,
	gardenReader client.Reader,
	condition gardencorev1beta1.Condition,
	controllerInstallations []gardencorev1beta1.ControllerInstallation,
	filterFunc func(gardencorev1beta1.ControllerInstallation) bool,
	progressingThreshold *metav1.Duration,
) (*gardencorev1beta1.Condition, error) {
	for _, controllerInstallation := range controllerInstallations {
		if !filterFunc(controllerInstallation) {
			continue
		}

		if exitCondition, err := h.CheckControllerInstallation(ctx, gardenReader, condition, &controllerInstallation, progressingThreshold); err != nil {
			return nil, fmt.Errorf("failed to check condition type %q for controller installation %q: %w", condition.Type, controllerInstallation.Name, err)
		} else if exitCondition != nil {
			return exitCondition, nil
		}
	}

	return nil, nil
}

// CheckControllerInstallation checks the conditions of the given controller installation and reflects the state in the returned condition.
func (h *HealthChecker) CheckControllerInstallation(ctx context.Context, gardenReader client.Reader, condition gardencorev1beta1.Condition, controllerInstallation *gardencorev1beta1.ControllerInstallation, controllerInstallationProgressingThreshold *metav1.Duration) (*gardencorev1beta1.Condition, error) {
	conditionsToCheck := map[gardencorev1beta1.ConditionType]func(condition gardencorev1beta1.Condition) bool{
		gardencorev1beta1.ControllerInstallationInstalled:   defaultSuccessfulCheck(),
		gardencorev1beta1.ControllerInstallationHealthy:     defaultSuccessfulCheck(),
		gardencorev1beta1.ControllerInstallationProgressing: resourcesNotProgressingCheck(h.clock, controllerInstallationProgressingThreshold),
	}

	return h.checkControllerInstallationConditions(ctx, gardenReader, condition, controllerInstallation, conditionsToCheck, controllerInstallationProgressingThreshold)
}

func (h *HealthChecker) checkControllerInstallationConditions(
	ctx context.Context,
	gardenReader client.Reader,
	condition gardencorev1beta1.Condition,
	controllerInstallation *gardencorev1beta1.ControllerInstallation,
	conditionsToCheck map[gardencorev1beta1.ConditionType]func(condition gardencorev1beta1.Condition) bool,
	controllerInstallationProgressingThreshold *metav1.Duration,
) (*gardencorev1beta1.Condition, error) {
	var controllerRegistration gardencorev1beta1.ControllerRegistration

	if err := gardenReader.Get(ctx, client.ObjectKey{Name: controllerInstallation.Spec.RegistrationRef.Name}, &controllerRegistration); err != nil {
		return nil, fmt.Errorf("failed to get controller registration %q: %w", controllerInstallation.Spec.RegistrationRef.Name, err)
	}

	if controllerRegistration.ResourceVersion != controllerInstallation.Spec.RegistrationRef.ResourceVersion {
		c := v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, "OutdatedControllerRegistration", fmt.Sprintf("observed resource version of controller registration '%s' in controller installation '%s' outdated (%s/%s)", controllerRegistration.Name, controllerInstallation.Name, controllerInstallation.Spec.RegistrationRef.ResourceVersion, controllerRegistration.ResourceVersion))
		return &c, nil
	}

	for _, condType := range []gardencorev1beta1.ConditionType{
		gardencorev1beta1.ControllerInstallationInstalled,
		gardencorev1beta1.ControllerInstallationHealthy,
		gardencorev1beta1.ControllerInstallationProgressing,
	} {
		cond := v1beta1helper.GetCondition(controllerInstallation.Status.Conditions, condType)
		if cond == nil {
			continue
		}

		checkConditionStatus, ok := conditionsToCheck[cond.Type]
		if !ok {
			continue
		}
		if !checkConditionStatus(*cond) {
			c := v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, cond.Reason, fmt.Sprintf("Seed %s: %s", controllerInstallation.Spec.SeedRef.Name, cond.Message))
			if cond.Type == gardencorev1beta1.ControllerInstallationProgressing && controllerInstallationProgressingThreshold != nil {
				c = v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, "ProgressingRolloutStuck", fmt.Sprintf("Seed %s: ControllerInstallation %s is progressing for more than %s", controllerInstallation.Spec.SeedRef.Name, controllerInstallation.Name, controllerInstallationProgressingThreshold.Duration))
			}
			return &c, nil
		}

		delete(conditionsToCheck, cond.Type)
	}

	if len(conditionsToCheck) > 0 {
		var missing []string
		for cond := range conditionsToCheck {
			missing = append(missing, string(cond))
		}
		c := v1beta1helper.FailedCondition(h.clock, h.lastOperation, h.conditionThresholds, condition, "MissingControllerInstallationCondition", fmt.Sprintf("Seed %s: ControllerInstallation %s is missing the following condition(s), %v", controllerInstallation.Spec.SeedRef.Name, controllerInstallation.Name, missing))
		return &c, nil
	}

	return nil, nil
}
