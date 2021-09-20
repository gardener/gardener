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

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CheckerFactory creates Checker instances.
type CheckerFactory interface {
	// NewChecker creates a new Checker using the given context, client, namespace, and shoot name.
	NewChecker(ctx context.Context, c client.Client, namespace, shootName string) (Checker, error)
}

// NewOwnerCheckerFactory creates a new CheckerFactory that uses NewOwnerChecker to create Checker instances.
func NewOwnerCheckerFactory(resolver Resolver, logger logr.Logger) CheckerFactory {
	return &ownerCheckerFactory{
		resolver: resolver,
		logger:   logger,
	}
}

type ownerCheckerFactory struct {
	resolver Resolver
	logger   logr.Logger
}

// NewChecker creates a new Checker using the given context, client, namespace, and shoot name.
// It reads the owner domain name and ID from the owner DNSRecord extension resource in the given namespace
// so that it can pass them to NewOwnerChecker.
// If the owner domain name and ID are empty strings, this method returns a nil Checker.
func (f *ownerCheckerFactory) NewChecker(ctx context.Context, c client.Client, namespace, shootName string) (Checker, error) {
	ownerName, ownerID, err := extensionscontroller.GetOwnerNameAndID(ctx, c, namespace, shootName)
	if err != nil {
		return nil, err
	}
	if ownerName == "" && ownerID == "" {
		return nil, nil
	}

	return NewOwnerChecker(ownerName, ownerID, f.resolver, f.logger.WithName(namespace)), nil
}
