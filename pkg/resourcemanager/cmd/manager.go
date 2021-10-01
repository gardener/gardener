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

package cmd

import (
	"time"

	"github.com/spf13/pflag"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ Option = &ManagerOptions{}

// ManagerOptions contains options needed to construct the controller-runtime manager.
type ManagerOptions struct {
	metricsBindAddress string
	healthBindAddress  string

	leaderElection              bool
	leaderElectionResourceLock  string
	leaderElectionNamespace     string
	leaderElectionLeaseDuration time.Duration
	leaderElectionRenewDeadline time.Duration
	leaderElectionRetryPeriod   time.Duration

	completed *ManagerConfig
}

// ManagerConfig contains the completed general manager configuration which can be applied to manager.Options via Apply.
type ManagerConfig struct {
	metricsBindAddress   string
	healthBindAddress    string
	livenessEndpointName string

	leaderElection              bool
	leaderElectionResourceLock  string
	leaderElectionID            string
	leaderElectionNamespace     string
	leaderElectionLeaseDuration time.Duration
	leaderElectionRenewDeadline time.Duration
	leaderElectionRetryPeriod   time.Duration
}

// AddFlags adds the needed command line flags to the given FlagSet.
func (o *ManagerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&o.leaderElection, "leader-election", true, "enable or disable leader election")
	fs.StringVar(&o.leaderElectionResourceLock, "leader-election-resource-lock", resourcelock.LeasesResourceLock, "Which resource type to use for leader election. "+
		"Supported options are 'endpoints', 'configmaps', 'leases', 'endpointsleases' and 'configmapsleases'.")
	fs.StringVar(&o.leaderElectionNamespace, "leader-election-namespace", "", "namespace for leader election")
	fs.DurationVar(&o.leaderElectionLeaseDuration, "leader-election-lease-duration", 15*time.Second, "lease duration for leader election")
	fs.DurationVar(&o.leaderElectionRenewDeadline, "leader-election-renew-deadline", 10*time.Second, "renew deadline for leader election")
	fs.DurationVar(&o.leaderElectionRetryPeriod, "leader-election-retry-period", 2*time.Second, "retry period for leader election")
	fs.StringVar(&o.metricsBindAddress, "metrics-bind-address", ":8080", "bind address for the metrics server")
	fs.StringVar(&o.healthBindAddress, "health-bind-address", ":8081", "bind address for the health server")
}

// Complete builds the manager.Options based on the given flag values.
func (o *ManagerOptions) Complete() error {
	o.completed = &ManagerConfig{
		leaderElection:              o.leaderElection,
		leaderElectionResourceLock:  o.leaderElectionResourceLock,
		leaderElectionID:            "gardener-resource-manager",
		leaderElectionNamespace:     o.leaderElectionNamespace,
		leaderElectionLeaseDuration: o.leaderElectionLeaseDuration,
		leaderElectionRenewDeadline: o.leaderElectionRenewDeadline,
		leaderElectionRetryPeriod:   o.leaderElectionRetryPeriod,
		metricsBindAddress:          o.metricsBindAddress,
		healthBindAddress:           o.healthBindAddress,
		livenessEndpointName:        "/healthz",
	}
	return nil
}

// Completed returns the constructed ManagerConfig.
func (o *ManagerOptions) Completed() *ManagerConfig {
	return o.completed
}

// Apply sets the values of this ManagerConfig on the given manager.Options.
func (c *ManagerConfig) Apply(opts *manager.Options) {
	opts.LeaderElection = c.leaderElection
	opts.LeaderElectionResourceLock = c.leaderElectionResourceLock
	opts.LeaderElectionID = c.leaderElectionID
	opts.LeaderElectionNamespace = c.leaderElectionNamespace
	opts.LeaseDuration = &c.leaderElectionLeaseDuration
	opts.RenewDeadline = &c.leaderElectionRenewDeadline
	opts.RetryPeriod = &c.leaderElectionRetryPeriod
	opts.MetricsBindAddress = c.metricsBindAddress
	opts.HealthProbeBindAddress = c.healthBindAddress
	opts.LivenessEndpointName = c.livenessEndpointName
}
