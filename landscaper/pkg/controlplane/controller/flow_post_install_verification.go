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

package controller

import (
	"context"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
	appsv1 "k8s.io/api/apps/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (o *operation) VerifyControlplane(ctx context.Context) error {
	var deploymentsToVerify = []string{deploymentNameGardenerAPIServer, deploymentNameGardenerScheduler, deploymentNameGardenerControllerManager}

	if o.imports.GardenerAdmissionController.Enabled {
		deploymentsToVerify = append(deploymentsToVerify, deploymentNameGardenerAdmissionController)
	}

	if err := retry.UntilTimeout(ctx, 5*time.Second, 500*time.Second, func(ctx context.Context) (done bool, err error) {
		for _, deploymentName := range deploymentsToVerify {
			apiServerDeployment := &appsv1.Deployment{}
			err = o.runtimeClient.Client().Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: deploymentName}, apiServerDeployment)
			if err != nil {
				return retry.MinorError(err)
			}

			if err := health.CheckDeployment(apiServerDeployment); err != nil {
				msg := fmt.Sprintf("deployment %s is not rolled out successfuly yet...: %v", deploymentName, err)
				o.log.Info(msg)
				return retry.MinorError(fmt.Errorf(msg))
			}
		}
		return retry.Ok()
	}); err != nil {
		return fmt.Errorf("failed waiting for the Gardener Control Plane to be available: %v", err)
	}

	o.log.Info("The Gardener Control Plane deployments are successfully rolled out")

	// wait for the Gardener API Service to be available as well as one exemplary
	// resource to be retrievable from the Gardener Resource Groups
	if err := retry.UntilTimeout(ctx, 2*time.Second, 500*time.Second, func(ctx context.Context) (done bool, err error) {
		apiService := &apiregistrationv1.APIService{}
		err = o.getGardenClient().Client().Get(ctx, kutil.Key(fmt.Sprintf("%s.%s", gardencorev1beta1.SchemeGroupVersion.Version, gardencorev1beta1.SchemeGroupVersion.Group)), apiService)
		if err != nil {
			return retry.MinorError(err)
		}

		for _, condition := range apiService.Status.Conditions {
			if condition.Status != apiregistrationv1.ConditionTrue {
				msg := fmt.Sprintf("APIService %q is not available yet. Reason: %q. Message: %q: %v", apiService.Name, condition.Reason, condition.Message, err)
				o.log.Info(msg)
				return retry.MinorError(fmt.Errorf(msg))
			}
		}
		return retry.Ok()
	}); err != nil {
		return fmt.Errorf("failed waiting for the Gardener APIService to become available: %v", err)
	}

	o.log.Info("The Gardener APIService is available")

	if err := retry.UntilTimeout(ctx, 2*time.Second, 30*time.Second, func(ctx context.Context) (done bool, err error) {
		seedList := &gardencorev1beta1.SeedList{}
		err = o.getGardenClient().Client().List(ctx, seedList)
		if err != nil {
			return retry.MinorError(err)
		}
		return retry.Ok()
	}); err != nil {
		return fmt.Errorf("failed waiting for the Gardener resource groups to be available: %v", err)
	}

	o.log.Info("The Gardener API Server successfully serves the Gardener resource groups")

	return nil
}

func (o *operation) verifyDeployment(ctx context.Context, err error, deploymentName string) (bool, error) {
	apiServerDeployment := &appsv1.Deployment{}
	err = o.runtimeClient.Client().Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: deploymentName}, apiServerDeployment)
	if err != nil {
		return retry.SevereError(err)
	}

	if err := health.CheckDeployment(apiServerDeployment); err != nil {
		msg := fmt.Sprintf("deployment %s is not rolled out successfuly yet...: %v", deploymentName, err)
		o.log.Info(msg)
		return retry.MinorError(fmt.Errorf(msg))
	}
	return retry.Ok()
}
