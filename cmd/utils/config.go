// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"os"

	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	componentbaseconfig "k8s.io/component-base/config"
)

// MakeLeaderElectionConfig creates a *leaderelection.LeaderElectionConfig from a set of configurations
func MakeLeaderElectionConfig(cfg componentbaseconfig.LeaderElectionConfiguration, lockObjectNamespace string, lockObjectName string, client k8s.Interface, recorder record.EventRecorder) (*leaderelection.LeaderElectionConfig, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("unable to get hostname: %v", err)
	}

	lock, err := resourcelock.New(
		cfg.ResourceLock,
		lockObjectNamespace,
		lockObjectName,
		client.CoreV1(),
		client.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      hostname,
			EventRecorder: recorder,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("couldn't create resource lock: %v", err)
	}

	return &leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: cfg.LeaseDuration.Duration,
		RenewDeadline: cfg.RenewDeadline.Duration,
		RetryPeriod:   cfg.RetryPeriod.Duration,
	}, nil
}
