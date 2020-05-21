// Copyright 2019 Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
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

package logging

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/test/framework"
	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Checks whether required logging resources are present.
// If not, probably the logging feature gate is not enabled.
func hasRequiredResources(ctx context.Context, k8sSeedClient kubernetes.Interface) (bool, error) {
	fluentBit := &appsv1.DaemonSet{}
	if err := k8sSeedClient.DirectClient().Get(ctx, client.ObjectKey{Namespace: garden, Name: fluentBitName}, fluentBit); err != nil {
		return false, err
	}

	loki := &appsv1.StatefulSet{}
	if err := k8sSeedClient.DirectClient().Get(ctx, client.ObjectKey{Namespace: garden, Name: lokiName}, loki); err != nil {
		return false, err
	}

	return true, nil
}

func checkRequiredResources(ctx context.Context, k8sSeedClient kubernetes.Interface) {
	isLoggingEnabled, err := hasRequiredResources(ctx, k8sSeedClient)
	if !isLoggingEnabled {
		message := fmt.Sprintf("Error occurred checking for required logging resources in the seed %s namespace. Ensure that the logging feature gate is enabled: %s", garden, err.Error())
		ginkgo.Fail(message)
	}
}

// WaitUntilLokiReceivesLogs waits until the loki instance in <lokiNamespace> receives <expected> logs from <podName>
func WaitUntilLokiReceivesLogs(ctx context.Context, f *framework.ShootFramework, lokiNamespace, podName string, expected int, client kubernetes.Interface) error {
	return retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		search, err := f.GetLokiLogs(ctx, lokiNamespace, podName, client)
		if err != nil {
			return retry.SevereError(err)
		}

		actual := search.Data.Stats.Summary.TotalLinesProcessed
		if expected > actual {
			f.Logger.Infof("Waiting to receive %d logs, currently received %d", expected, actual)
			return retry.MinorError(fmt.Errorf("received only %d/%d logs", actual, expected))
		} else if expected < actual {
			return retry.SevereError(fmt.Errorf("expected to receive %d logs but was %d", expected, actual))
		}

		f.Logger.Infof("Received all of %d logs", actual)
		return retry.Ok()
	})
}
