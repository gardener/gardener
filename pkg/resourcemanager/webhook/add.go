// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/crddeletionprotection"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/endpointslicehints"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/extensionvalidation"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/highavailabilityconfig"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/kubernetesservicehost"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/nodeagentauthorizer"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/podschedulername"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/podtopologyspreadconstraints"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/projectedtokenmount"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/seccompprofile"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/systemcomponentsconfig"
)

// AddToManager adds all webhook handlers to the given manager.
func AddToManager(mgr manager.Manager, sourceCluster, targetCluster cluster.Cluster, cfg *resourcemanagerconfigv1alpha1.ResourceManagerConfiguration) error {
	if cfg.Webhooks.CRDDeletionProtection.Enabled {
		if err := (&crddeletionprotection.Handler{
			Logger:       mgr.GetLogger().WithName("webhook").WithName(crddeletionprotection.HandlerName),
			SourceReader: sourceCluster.GetAPIReader(),
			Decoder:      admission.NewDecoder(mgr.GetScheme()),
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", crddeletionprotection.HandlerName, err)
		}
	}

	if cfg.Webhooks.EndpointSliceHints.Enabled {
		if err := (&endpointslicehints.Handler{
			Logger: mgr.GetLogger().WithName("webhook").WithName(endpointslicehints.HandlerName),
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", endpointslicehints.HandlerName, err)
		}
	}

	if cfg.Webhooks.ExtensionValidation.Enabled {
		if err := extensionvalidation.AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handlers: %w", extensionvalidation.HandlerName, err)
		}
	}

	if cfg.Webhooks.HighAvailabilityConfig.Enabled {
		if err := (&highavailabilityconfig.Handler{
			Logger:       mgr.GetLogger().WithName("webhook").WithName(highavailabilityconfig.HandlerName),
			TargetClient: targetCluster.GetClient(),
			Config:       cfg.Webhooks.HighAvailabilityConfig,
			Decoder:      admission.NewDecoder(mgr.GetScheme()),
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", highavailabilityconfig.HandlerName, err)
		}
	}

	if cfg.Webhooks.KubernetesServiceHost.Enabled {
		if err := (&kubernetesservicehost.Handler{
			Logger: mgr.GetLogger().WithName("webhook").WithName(kubernetesservicehost.HandlerName),
			Host:   cfg.Webhooks.KubernetesServiceHost.Host,
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", kubernetesservicehost.HandlerName, err)
		}
	}

	if cfg.Webhooks.SystemComponentsConfig.Enabled {
		if err := (&systemcomponentsconfig.Handler{
			Logger:          mgr.GetLogger().WithName("webhook").WithName(systemcomponentsconfig.HandlerName),
			TargetClient:    targetCluster.GetClient(),
			NodeSelector:    cfg.Webhooks.SystemComponentsConfig.NodeSelector,
			PodNodeSelector: cfg.Webhooks.SystemComponentsConfig.PodNodeSelector,
			PodTolerations:  cfg.Webhooks.SystemComponentsConfig.PodTolerations,
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", systemcomponentsconfig.HandlerName, err)
		}
	}

	if cfg.Webhooks.PodSchedulerName.Enabled {
		if err := (&podschedulername.Handler{
			SchedulerName: *cfg.Webhooks.PodSchedulerName.SchedulerName,
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", podschedulername.HandlerName, err)
		}
	}

	if cfg.Webhooks.PodTopologySpreadConstraints.Enabled {
		if err := (&podtopologyspreadconstraints.Handler{
			Logger: mgr.GetLogger().WithName("webhook").WithName(podtopologyspreadconstraints.HandlerName),
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", podtopologyspreadconstraints.HandlerName, err)
		}
	}

	if cfg.Webhooks.ProjectedTokenMount.Enabled {
		if err := (&projectedtokenmount.Handler{
			Logger:            mgr.GetLogger().WithName("webhook").WithName(projectedtokenmount.HandlerName),
			TargetReader:      targetCluster.GetCache(),
			ExpirationSeconds: *cfg.Webhooks.ProjectedTokenMount.ExpirationSeconds,
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", projectedtokenmount.HandlerName, err)
		}
	}

	if cfg.Webhooks.NodeAgentAuthorizer.Enabled {
		if err := (&nodeagentauthorizer.Webhook{
			Logger: mgr.GetLogger().WithName("webhook").WithName(nodeagentauthorizer.HandlerName),
			Config: cfg.Webhooks.NodeAgentAuthorizer,
		}).AddToManager(mgr, sourceCluster.GetClient(), targetCluster.GetClient()); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", nodeagentauthorizer.HandlerName, err)
		}
	}

	if cfg.Webhooks.SeccompProfile.Enabled {
		if err := (&seccompprofile.Handler{
			Logger: mgr.GetLogger().WithName("webhook").WithName(seccompprofile.HandlerName),
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding %s webhook handler: %w", seccompprofile.HandlerName, err)
		}
	}

	return nil
}
