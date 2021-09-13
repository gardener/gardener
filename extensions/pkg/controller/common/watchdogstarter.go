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

package common

import (
	"context"
	"net"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DefaultWatchdogInterval is the default watchdog interval used by StartOwnerCheckWatchdog.
	DefaultWatchdogInterval = 1 * time.Minute
)

var (
	// DefaultResolver is the default resolver used by StartOwnerCheckWatchdog.
	DefaultResolver Resolver = net.DefaultResolver
)

// OwnerCheckWatchdogStarter provides a method that checks the owner and starts an owner check watchdog.
type OwnerCheckWatchdogStarter interface {
	// Start checks the owner and starts an owner check watchdog.
	Start(ctx context.Context, c client.Client, namespace, name string, logger logr.Logger) (bool, context.Context, context.CancelFunc, error)
}

// OwnerCheckWatchdogStarterFunc is a function that implements OwnerCheckWatchdogStarter.
type OwnerCheckWatchdogStarterFunc func(ctx context.Context, c client.Client, namespace, name string, logger logr.Logger) (bool, context.Context, context.CancelFunc, error)

// Start checks the owner and starts an owner check watchdog.
func (f OwnerCheckWatchdogStarterFunc) Start(ctx context.Context, c client.Client, namespace, name string, logger logr.Logger) (bool, context.Context, context.CancelFunc, error) {
	return f(ctx, c, namespace, name, logger)
}

// StartOwnerCheckWatchdog checks the owner and starts an owner check watchdog.
// It reads the owner domain name and ID from the owner DNSRecord extension resource in the given namespace,
// checks if the owner domain name resolves to the owner ID, and if yes, starts a watchdog that performs the same check every minute,
// returning its modified context and cancel function.
func StartOwnerCheckWatchdog(ctx context.Context, c client.Client, namespace, name string, logger logr.Logger) (bool, context.Context, context.CancelFunc, error) {
	ownerName, ownerID, err := extensionscontroller.GetOwnerNameAndID(ctx, c, namespace, name)
	if err != nil {
		return false, ctx, nil, err
	}
	if ownerName == "" && ownerID == "" {
		return true, ctx, nil, nil
	}

	logger.Info("Checking if owner domain name resolves to owner ID", "ownerName", ownerName, "ownerID", ownerID)
	ownerChecker := NewOwnerChecker(ownerName, ownerID, DefaultResolver, logger)
	if ok, err := ownerChecker.Check(ctx); err != nil {
		return false, ctx, nil, err
	} else if !ok {
		return false, ctx, nil, nil
	}

	logger.Info("Starting owner check watchdog", "interval", DefaultWatchdogInterval)
	watchdog := NewCheckerWatchdog(ownerChecker, DefaultWatchdogInterval, logger)
	var cancel context.CancelFunc
	ctx, cancel = watchdog.Start(ctx)
	return true, ctx, cancel, nil
}
