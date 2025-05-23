// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	"k8s.io/utils/clock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

// CheckAPIServerAvailability checks if the API server of a cluster is reachable and measure the response time.
func CheckAPIServerAvailability(
	ctx context.Context,
	clock clock.Clock,
	log logr.Logger,
	condition gardencorev1beta1.Condition,
	restClient rest.Interface,
	conditioner conditionerFunc,
) gardencorev1beta1.Condition {
	response := restClient.Get().AbsPath("/healthz").Do(ctx)
	responseDurationText := fmt.Sprintf("[response_time:%dms]", clock.Now().Sub(clock.Now()).Nanoseconds()/time.Millisecond.Nanoseconds())
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
			body = "Could not parse response body: " + err.Error()
		} else {
			body = string(bodyRaw)
		}

		log.Error(err, "API Server /healthz endpoint check returned non ok status code", "statusCode", statusCode, "body", body)
		return conditioner("HealthzRequestError", fmt.Sprintf("API server /healthz endpoint check returned a non ok status code %d. (%s)", statusCode, body))
	}

	message := "API server /healthz endpoint responded with success status code."
	return v1beta1helper.UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionTrue, "HealthzRequestSucceeded", message)
}
