// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionsconfigv1alpha1 "github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// Actuator contains all the health checks and the means to execute them
type Actuator struct {
	restConfig   *rest.Config
	sourceClient client.Client

	provider            string
	extensionKind       string
	extensionClasses    []extensionsv1alpha1.ExtensionClass
	getExtensionObjFunc GetExtensionObjectFunc
	healthChecks        []ConditionTypeToHealthCheck
	targetRESTOptions   extensionsconfigv1alpha1.RESTOptions
}

// NewActuator creates a new Actuator.
func NewActuator(mgr manager.Manager, provider, extensionKind string, getExtensionObjFunc GetExtensionObjectFunc, healthChecks []ConditionTypeToHealthCheck, targetRESTOptions extensionsconfigv1alpha1.RESTOptions, extensionClasses []extensionsv1alpha1.ExtensionClass) HealthCheckActuator {
	return &Actuator{
		restConfig:   mgr.GetConfig(),
		sourceClient: mgr.GetClient(),

		healthChecks:        healthChecks,
		getExtensionObjFunc: getExtensionObjFunc,
		provider:            provider,
		extensionKind:       extensionKind,
		extensionClasses:    extensionClasses,
		targetRESTOptions:   targetRESTOptions,
	}
}

type healthCheckUnsuccessful struct {
	detail string
}

type healthCheckProgressing struct {
	detail    string
	threshold *time.Duration
}

type channelResult struct {
	healthConditionType string
	healthCheckResult   *SingleCheckResult
	error               error
}

type checkResultForConditionType struct {
	failedChecks       []error
	unsuccessfulChecks []healthCheckUnsuccessful
	progressingChecks  []healthCheckProgressing
	successfulChecks   int
	codes              []gardencorev1beta1.ErrorCode
}

// ExecuteHealthCheckFunctions executes all the health check functions, injects clients and logger & aggregates the results.
// returns an Result for each HealthConditionType (e.g  ControlPlaneHealthy)
func (a *Actuator) ExecuteHealthCheckFunctions(ctx context.Context, log logr.Logger, request types.NamespacedName) (*[]Result, error) {
	var (
		targetClient client.Client
		channel      = make(chan channelResult, len(a.healthChecks))
		wg           sync.WaitGroup
	)

	for _, hc := range a.healthChecks {
		check := hc.HealthCheck
		SourceClientInfo(a.sourceClient, check)
		if _, ok := check.(TargetClient); ok {
			if targetClient == nil {
				var err error
				_, targetClient, err = util.NewClientForShoot(ctx, a.sourceClient, request.Namespace, client.Options{}, a.targetRESTOptions)
				if err != nil {
					// don't return here, as we might have started some goroutines already to prevent leakage
					channel <- channelResult{
						healthCheckResult: &SingleCheckResult{
							Status: gardencorev1beta1.ConditionFalse,
							Detail: fmt.Sprintf("failed to create shoot client: %v", err),
						},
						error:               err,
						healthConditionType: hc.ConditionType,
					}
					continue
				}
			}
			TargetClientInfo(targetClient, check)
		}

		check.SetLoggerSuffix(a.provider, a.extensionKind)

		wg.Add(1)
		go func(ctx context.Context, request types.NamespacedName, check HealthCheck, preCheckFunc PreCheckFunc, errorCodeCheckFunc ErrorCodeCheckFunc, healthConditionType string) {
			defer wg.Done()

			if preCheckFunc != nil {
				obj := a.getExtensionObjFunc()
				if err := a.sourceClient.Get(ctx, request, obj); err != nil {
					channel <- channelResult{
						healthCheckResult: &SingleCheckResult{
							Status: gardencorev1beta1.ConditionFalse,
							Detail: fmt.Sprintf("failed to read the extension resource: %v", err),
						},
						error:               err,
						healthConditionType: healthConditionType,
					}
					return
				}

				preCheckObj, err := a.getPreCheckFuncObject(ctx, request)
				if err != nil {
					channel <- channelResult{
						healthCheckResult: &SingleCheckResult{
							Status: gardencorev1beta1.ConditionFalse,
							Detail: fmt.Sprintf("failed to read the resource: %v", err),
						},
						error:               err,
						healthConditionType: healthConditionType,
					}
					return
				}

				if !preCheckFunc(preCheckObj) {
					log.V(1).Info("Skipping health check as pre check function returned false", "conditionType", healthConditionType)
					channel <- channelResult{
						healthCheckResult: &SingleCheckResult{
							Status: gardencorev1beta1.ConditionTrue,
						},
						error:               nil,
						healthConditionType: healthConditionType,
					}
					return
				}
			}

			healthCheckResult, err := check.Check(ctx, request)

			if healthCheckResult != nil && errorCodeCheckFunc != nil {
				healthCheckResult.Codes = append(healthCheckResult.Codes, errorCodeCheckFunc(fmt.Errorf("%s", healthCheckResult.Detail))...)
			}

			channel <- channelResult{
				healthCheckResult:   healthCheckResult,
				error:               err,
				healthConditionType: healthConditionType,
			}
		}(ctx, request, check, hc.PreCheckFunc, hc.ErrorCodeCheckFunc, hc.ConditionType)
	}

	// close channel when wait group has 0 counter
	go func() {
		wg.Wait()
		close(channel)
	}()

	groupedHealthCheckResults := make(map[string]*checkResultForConditionType)
	// loop runs until channel is closed
	for channelResult := range channel {
		if groupedHealthCheckResults[channelResult.healthConditionType] == nil {
			groupedHealthCheckResults[channelResult.healthConditionType] = &checkResultForConditionType{}
		}
		if channelResult.error != nil {
			groupedHealthCheckResults[channelResult.healthConditionType].failedChecks = append(groupedHealthCheckResults[channelResult.healthConditionType].failedChecks, channelResult.error)
			continue
		}
		if channelResult.healthCheckResult.Status == gardencorev1beta1.ConditionFalse {
			groupedHealthCheckResults[channelResult.healthConditionType].unsuccessfulChecks = append(groupedHealthCheckResults[channelResult.healthConditionType].unsuccessfulChecks, healthCheckUnsuccessful{detail: channelResult.healthCheckResult.Detail})
			groupedHealthCheckResults[channelResult.healthConditionType].codes = append(groupedHealthCheckResults[channelResult.healthConditionType].codes, channelResult.healthCheckResult.Codes...)
			continue
		}
		if channelResult.healthCheckResult.Status == gardencorev1beta1.ConditionProgressing {
			groupedHealthCheckResults[channelResult.healthConditionType].progressingChecks = append(groupedHealthCheckResults[channelResult.healthConditionType].progressingChecks, healthCheckProgressing{detail: channelResult.healthCheckResult.Detail, threshold: channelResult.healthCheckResult.ProgressingThreshold})
			groupedHealthCheckResults[channelResult.healthConditionType].codes = append(groupedHealthCheckResults[channelResult.healthConditionType].codes, channelResult.healthCheckResult.Codes...)
			continue
		}
		groupedHealthCheckResults[channelResult.healthConditionType].successfulChecks++
	}

	var checkResults []Result
	for conditionType, result := range groupedHealthCheckResults {
		if len(result.unsuccessfulChecks) > 0 || len(result.failedChecks) > 0 {
			var details strings.Builder
			if err := result.appendFailedChecksDetails(&details); err != nil {
				return nil, err
			}
			if err := result.appendUnsuccessfulChecksDetails(&details); err != nil {
				return nil, err
			}
			if err := result.appendProgressingChecksDetails(&details); err != nil {
				return nil, err
			}

			checkResults = append(checkResults, Result{
				HealthConditionType: conditionType,
				Status:              gardencorev1beta1.ConditionFalse,
				Detail:              ptr.To(trimTrailingWhitespace(details.String())),
				SuccessfulChecks:    result.successfulChecks,
				UnsuccessfulChecks:  len(result.unsuccessfulChecks),
				FailedChecks:        len(result.failedChecks),
				Codes:               result.codes,
			})

			continue
		}

		if len(result.progressingChecks) > 0 {
			var (
				details   strings.Builder
				threshold *time.Duration
			)

			for index, check := range result.progressingChecks {
				if len(result.progressingChecks) == 1 {
					details.WriteString(ensureTrailingDot(check.detail))
				} else {
					details.WriteString(fmt.Sprintf("%d) %s ", index+1, ensureTrailingDot(check.detail)))
				}

				if check.threshold != nil && (threshold == nil || *threshold > *check.threshold) {
					threshold = check.threshold
				}
			}

			checkResults = append(checkResults, Result{
				HealthConditionType:  conditionType,
				Status:               gardencorev1beta1.ConditionProgressing,
				ProgressingThreshold: threshold,
				Detail:               ptr.To(trimTrailingWhitespace(details.String())),
				SuccessfulChecks:     result.successfulChecks,
				ProgressingChecks:    len(result.progressingChecks),
				Codes:                result.codes,
			})

			continue
		}

		checkResults = append(checkResults, Result{
			HealthConditionType: conditionType,
			Status:              gardencorev1beta1.ConditionTrue,
			SuccessfulChecks:    result.successfulChecks,
		})
	}

	return &checkResults, nil
}

func (a *Actuator) getPreCheckFuncObject(ctx context.Context, request types.NamespacedName) (any, error) {
	if isGardenExtensionClass(a.extensionClasses) {
		garden, err := getGarden(ctx, a.sourceClient)
		if err != nil {
			return nil, err
		}
		return garden, nil
	}
	cluster, err := extensionscontroller.GetCluster(ctx, a.sourceClient, request.Namespace)
	if err != nil {
		return nil, err
	}
	return cluster, nil
}

// getGarden retrieves the Garden resource from the garden cluster.
func getGarden(ctx context.Context, c client.Client) (*operatorv1alpha1.Garden, error) {
	gardenList := &operatorv1alpha1.GardenList{}
	if err := c.List(ctx, gardenList); err != nil {
		return nil, fmt.Errorf("failed to list Garden resources: %w", err)
	}

	if len(gardenList.Items) == 0 {
		return nil, fmt.Errorf("no Garden resource found")
	}

	if len(gardenList.Items) > 1 {
		return nil, fmt.Errorf("multiple Garden resources found, expected exactly one")
	}

	return &gardenList.Items[0], nil
}
