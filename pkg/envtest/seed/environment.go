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

package envtestseed

import (
	"path/filepath"

	"github.com/gardener/gardener/charts"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("seed.test-env")

// SeedTestEnvironment wraps envtest.Environment and registers CRDs typically deployed during
// Seed bootstrap
type SeedTestEnvironment struct {
	*envtest.Environment
	chartPath string
}

// Start starts the underlying envtest.Environment
func (e *SeedTestEnvironment) Start() (*rest.Config, error) {
	if e.Environment == nil {
		e.Environment = &envtest.Environment{}
	}

	if e.chartPath == "" {
		e.chartPath = filepath.Join("..", "..", "..", charts.Path)
	}

	pathExtensionCRDs := filepath.Join(e.chartPath, "seed-bootstrap", "charts", "extensions", "templates")
	pathVPA := filepath.Join("resources", "crd-verticalpodautoscalers.yaml")
	pathVPACheckpoints := filepath.Join("resources", "crd-verticalpodautoscalercheckpoints.yaml")
	pathHVPA := filepath.Join("resources", "hvpa-crd.yaml")
	pathIstio := filepath.Join(e.chartPath, "istio", "istio-crds", "templates", "crd-all.gen.yaml")

	e.Environment.CRDDirectoryPaths = []string{
		pathExtensionCRDs,
		pathVPA,
		pathVPACheckpoints,
		pathHVPA,
		pathIstio,
	}
	e.Environment.ErrorIfCRDPathMissing = true

	log.V(1).Info("starting Seed's envtest control plane")
	restConfig, err := e.Environment.Start()
	if err != nil {
		return nil, err
	}

	return restConfig, nil
}

// Stop stops the underlying envtest.Environment.
func (e *SeedTestEnvironment) Stop() error {
	return e.Environment.Stop()
}
