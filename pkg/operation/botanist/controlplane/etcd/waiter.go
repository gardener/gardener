// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package etcd

import (
	"context"
	"fmt"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/retry"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (e *etcd) Wait(_ context.Context) error        { return nil }
func (e *etcd) WaitCleanup(_ context.Context) error { return nil }

// WaitUntilEtcdsReady waits until all etcds in the given namespace are ready.
func WaitUntilEtcdsReady(
	ctx context.Context,
	c client.Client,
	logger logrus.FieldLogger,
	namespace string,
	count int,
	interval time.Duration,
	severeThreshold time.Duration,
	timeout time.Duration,
) error {
	var retryCountUntilSevere int

	return retry.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (done bool, err error) {
		retryCountUntilSevere++

		etcdList := &druidv1alpha1.EtcdList{}
		if err := c.List(
			ctx,
			etcdList,
			client.InNamespace(namespace),
			client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane},
		); err != nil {
			return retry.SevereError(err)
		}

		if n := len(etcdList.Items); n < count {
			logger.Info("Waiting until the etcd gets created...")
			return retry.MinorError(fmt.Errorf("only %d/%d etcd resources found", n, count))
		}

		var lastErrors error

		for _, etcd := range etcdList.Items {
			switch {
			case etcd.Status.LastError != nil:
				return retry.MinorOrSevereError(retryCountUntilSevere, int(severeThreshold.Nanoseconds()/interval.Nanoseconds()), fmt.Errorf("%s reconciliation errored: %s", etcd.Name, *etcd.Status.LastError))
			case etcd.DeletionTimestamp != nil:
				lastErrors = multierror.Append(lastErrors, fmt.Errorf("%s unexpectedly has a deletion timestamp", etcd.Name))
			case etcd.Status.ObservedGeneration == nil || etcd.Generation != *etcd.Status.ObservedGeneration:
				lastErrors = multierror.Append(lastErrors, fmt.Errorf("%s reconciliation pending", etcd.Name))
			case metav1.HasAnnotation(etcd.ObjectMeta, v1beta1constants.GardenerOperation):
				lastErrors = multierror.Append(lastErrors, fmt.Errorf("%s reconciliation in process", etcd.Name))
			case !utils.IsTrue(etcd.Status.Ready):
				lastErrors = multierror.Append(lastErrors, fmt.Errorf("%s is not ready yet", etcd.Name))
			}
		}

		if lastErrors == nil {
			return retry.Ok()
		}

		logger.Info("Waiting until both the etcds are ready...")
		return retry.MinorError(lastErrors)
	})
}
