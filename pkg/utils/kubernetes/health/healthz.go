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

package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"

	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
)

// CheckAPIServerAvailability checks if the API server of a cluster is reachable and measure the response time.
func CheckAPIServerAvailability(
	ctx context.Context,
	log logr.Logger,
	condition gardencorev1beta1.Condition,
	restClient rest.Interface,
	conditioner conditionerFunc,
) gardencorev1beta1.Condition {
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

		log.Error(err, "API Server /healthz endpoint check returned non ok status code", "statusCode", statusCode, "body", body)
		return conditioner("HealthzRequestError", fmt.Sprintf("API server /healthz endpoint check returned a non ok status code %d. (%s)", statusCode, body))
	}

	message := "API server /healthz endpoint responded with success status code."
	return v1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionTrue, "HealthzRequestSucceeded", message)
}

// Now determines the current time.
var Now = time.Now
