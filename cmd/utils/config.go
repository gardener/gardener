// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils

import (
	"fmt"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	componentbaseconfig "k8s.io/component-base/config"
	"os"
)

// RESTConfigFromClientConnectionConfiguration creates a *rest.Config from a componentbaseconfig.ClientConnectionConfiguration & the configured kubeconfig
func RESTConfigFromClientConnectionConfiguration(cfg componentbaseconfig.ClientConnectionConfiguration) (*rest.Config, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	if err != nil {
		return nil, err
	}

	restConfig.Burst = int(cfg.Burst)
	restConfig.QPS = cfg.QPS
	restConfig.AcceptContentTypes = cfg.AcceptContentTypes
	restConfig.ContentType = cfg.ContentType
	return restConfig, nil
}

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
