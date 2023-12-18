// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package healthcheck

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

// ContainerdClient defines the containerd client Interface exported for testing.
type ContainerdClient interface {
	Version(context.Context) (containerd.Version, error)
}

type containerdHealthChecker struct {
	client client.Client

	containerdClient ContainerdClient
	firstFailure     *time.Time
	clock            clock.Clock
	dbus             dbus.DBus
	recorder         record.EventRecorder
}

// NewContainerdHealthChecker creates a new instance of a containerd health check.
func NewContainerdHealthChecker(client client.Client, containerdClient ContainerdClient, clock clock.Clock, dbus dbus.DBus, recorder record.EventRecorder) HealthChecker {
	return &containerdHealthChecker{
		client:           client,
		containerdClient: containerdClient,
		clock:            clock,
		dbus:             dbus,
		recorder:         recorder,
	}
}

// Name returns the name of this health check.
func (*containerdHealthChecker) Name() string {
	return "containerd"
}

// Check performs the actual health check for containerd.
func (c *containerdHealthChecker) Check(ctx context.Context, node *corev1.Node) error {
	log := logf.FromContext(ctx).WithName(c.Name())

	_, err := c.containerdClient.Version(ctx)
	if err != nil {
		if c.firstFailure == nil {
			now := c.clock.Now()
			c.firstFailure = &now
			log.Error(err, "Unable to get containerd version, considered unhealthy")
			c.recorder.Eventf(node, corev1.EventTypeWarning, "containerd", "Containerd is unhealthy: %s", err.Error())
		}

		if time.Since(*c.firstFailure).Abs() < maxFailureDuration {
			return nil
		}

		log.Error(err, "Unable to get containerd version, restarting it", "failureDuration", maxFailureDuration)
		c.recorder.Eventf(node, corev1.EventTypeWarning, "containerd", "Containerd is unhealthy for more than %s, restarting it: %s", maxFailureDuration, err.Error())
		if err := c.dbus.Restart(ctx, c.recorder, node, v1beta1constants.OperatingSystemConfigUnitNameContainerDService); err != nil {
			return fmt.Errorf("failed restarting containerd: %w", err)
		}

		c.firstFailure = nil
		return nil
	}

	if c.firstFailure != nil {
		log.Info("Containerd is healthy again")
		c.recorder.Event(node, corev1.EventTypeNormal, "containerd", "Containerd is healthy")
		c.firstFailure = nil
	}
	return nil
}
