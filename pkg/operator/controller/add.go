// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/charts"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/controller/service"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	"github.com/gardener/gardener/pkg/operator/controller/garden/care"
	"github.com/gardener/gardener/pkg/operator/controller/garden/garden"
	"github.com/gardener/gardener/pkg/operator/controller/networkpolicyregistrar"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// AddToManager adds all controllers to the given manager.
func AddToManager(ctx context.Context, mgr manager.Manager, cfg *config.OperatorConfiguration) error {
	imageVector, err := imagevector.ReadGlobalImageVectorWithEnvOverride(filepath.Join(charts.Path, "images.yaml"))
	if err != nil {
		return fmt.Errorf("failed reading image vector override: %w", err)
	}

	var componentImageVectors imagevector.ComponentImageVectors
	if path := os.Getenv(imagevector.ComponentOverrideEnv); path != "" {
		componentImageVectors, err = imagevector.ReadComponentOverwriteFile(path)
		if err != nil {
			return fmt.Errorf("failed reading component-specific image vector override: %w", err)
		}
	}

	identity, err := gardenerutils.DetermineIdentity()
	if err != nil {
		return err
	}

	gardenClientMap, err := clientmapbuilder.
		NewGardenClientMapBuilder().
		WithRuntimeClient(mgr.GetClient()).
		WithClientConnectionConfig(&cfg.VirtualClientConnection).
		Build(mgr.GetLogger())
	if err != nil {
		return fmt.Errorf("failed to build garden ClientMap: %w", err)
	}
	if err := mgr.Add(gardenClientMap); err != nil {
		return err
	}

	if err := (&garden.Reconciler{
		Config:                *cfg,
		Identity:              identity,
		ImageVector:           imageVector,
		ComponentImageVectors: componentImageVectors,
		GardenClientMap:       gardenClientMap,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding Garden controller: %w", err)
	}

	if err := (&care.Reconciler{
		Config:          *cfg,
		GardenClientMap: gardenClientMap,
	}).AddToManager(ctx, mgr); err != nil {
		return fmt.Errorf("failed adding Garden-Care controller: %w", err)
	}

	if err := (&networkpolicyregistrar.Reconciler{
		Config: cfg.Controllers.NetworkPolicy,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding NetworkPolicy Registrar controller: %w", err)
	}

	if os.Getenv("GARDENER_OPERATOR_LOCAL") == "true" {
		virtualGardenKubeAPIServerPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: map[string]string{
			v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
			v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer,
		}})
		if err != nil {
			return err
		}

		virtualGardenIstioIngressPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: sharedcomponent.GetIstioZoneLabels(nil, nil)})
		if err != nil {
			return err
		}

		nginxIngressPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: map[string]string{
			"app":       "nginx-ingress",
			"component": "controller",
		}})
		if err != nil {
			return err
		}

		if err := (&service.Reconciler{}).AddToManager(mgr, predicate.Or(virtualGardenKubeAPIServerPredicate, virtualGardenIstioIngressPredicate, nginxIngressPredicate)); err != nil {
			return fmt.Errorf("failed adding Service controller: %w", err)
		}
	}

	return nil
}
