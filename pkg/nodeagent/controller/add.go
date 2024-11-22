// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/nodeagent/apis/config"
	"github.com/gardener/gardener/pkg/nodeagent/controller/certificate"
	"github.com/gardener/gardener/pkg/nodeagent/controller/healthcheck"
	"github.com/gardener/gardener/pkg/nodeagent/controller/hostnamecheck"
	"github.com/gardener/gardener/pkg/nodeagent/controller/lease"
	"github.com/gardener/gardener/pkg/nodeagent/controller/node"
	"github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/nodeagent/controller/token"
)

// AddToManager adds all controllers to the given manager.
func AddToManager(ctx context.Context, cancel context.CancelFunc, mgr manager.Manager, cfg *config.NodeAgentConfiguration, hostName, machineName string) error {
	nodePredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelHostname: hostName}})
	if err != nil {
		return fmt.Errorf("failed computing label selector predicate for node: %w", err)
	}

	if features.DefaultFeatureGate.Enabled(features.NodeAgentAuthorizer) {
		if err := (&certificate.Reconciler{
			Cancel:      cancel,
			MachineName: machineName,
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding certificate controller: %w", err)
		}
	}

	if err := (&node.Reconciler{}).AddToManager(mgr, nodePredicate); err != nil {
		return fmt.Errorf("failed adding node controller: %w", err)
	}

	if err := (&operatingsystemconfig.Reconciler{
		Config:        cfg.Controllers.OperatingSystemConfig,
		HostName:      hostName,
		CancelContext: cancel,
	}).AddToManager(ctx, mgr); err != nil {
		return fmt.Errorf("failed adding operating system config controller: %w", err)
	}

	if err := (&token.Reconciler{
		Config: cfg.Controllers.Token,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding token controller: %w", err)
	}

	if err := (&lease.Reconciler{}).AddToManager(mgr, nodePredicate); err != nil {
		return fmt.Errorf("failed adding lease controller: %w", err)
	}

	if err := (&healthcheck.Reconciler{}).AddToManager(mgr, nodePredicate); err != nil {
		return fmt.Errorf("failed adding health-check controller: %w", err)
	}

	if err := (&hostnamecheck.Reconciler{
		HostName:      hostName,
		CancelContext: cancel,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding hostname-check controller: %w", err)
	}

	return nil
}
