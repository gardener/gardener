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

package envtest_garden

import (
	"fmt"
	"os"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("gardener.test-env")

// GardenerTestEnvironment wraps envtest.Environment and additionally starts, registers and stops an instance of
// gardener-apiserver in order to work with gardener resources in the test.
type GardenerTestEnvironment struct {
	*envtest.Environment

	// GardenerAPIServer knows how to start, register and stop a temporary gardener-apiserver instance.
	GardenerAPIServer *GardenerAPIServer
}

// Start starts the underlying envtest.Environment and the GardenerAPIServer.
func (e *GardenerTestEnvironment) Start() (*rest.Config, error) {
	if e.Environment == nil {
		e.Environment = &envtest.Environment{}
	}

	// configure APIServer aggregation layer
	if e.Environment.ControlPlane.APIServer == nil {
		e.Environment.ControlPlane.APIServer = &envtest.APIServer{}
	}
	e.Environment.ControlPlane.APIServer.Args = append(envtest.DefaultKubeAPIServerFlags,
		"--client-ca-file={{ .CertDir }}/apiserver-ca.crt",
		"--proxy-client-key-file={{ .CertDir }}/apiserver.key",
		"--proxy-client-cert-file={{ .CertDir }}/apiserver.crt",
		"--requestheader-client-ca-file={{ .CertDir }}/apiserver-ca.crt",
		"--requestheader-extra-headers-prefix=X-Remote-Extra-",
		"--requestheader-group-headers=X-Remote-Group",
		"--requestheader-username-headers=X-Remote-User",
	)

	log.V(1).Info("starting envtest control plane")
	restConfig, err := e.Environment.Start()
	if err != nil {
		return nil, err
	}

	// TODO: respect Environment.UseExistingCluster / USE_EXISTING_CLUSTER for running tests against a local setup
	// instead of running gardener-apiserver and spinning up a test environment.
	if e.GardenerAPIServer == nil {
		e.GardenerAPIServer = &GardenerAPIServer{}
	}

	// default GardenerAPIServer settings to the envtest ControlPlane settings
	e.GardenerAPIServer.restConfig = restConfig
	if e.GardenerAPIServer.StartTimeout.Milliseconds() == 0 {
		e.GardenerAPIServer.StartTimeout = e.ControlPlaneStartTimeout
	}
	if e.GardenerAPIServer.StopTimeout.Milliseconds() == 0 {
		e.GardenerAPIServer.StopTimeout = e.ControlPlaneStopTimeout
	}
	if e.GardenerAPIServer.Out == nil && e.AttachControlPlaneOutput {
		e.GardenerAPIServer.Out = os.Stdout
	}
	if e.GardenerAPIServer.Err == nil && e.AttachControlPlaneOutput {
		e.GardenerAPIServer.Err = os.Stderr
	}
	// reuse etcd from envtest ControlPlane if not overwritten
	if e.GardenerAPIServer.EtcdURL == nil {
		e.GardenerAPIServer.EtcdURL = e.Environment.ControlPlane.Etcd.URL
	}

	if err := e.GardenerAPIServer.Start(); err != nil {
		return nil, fmt.Errorf("failed to start gardener-apiserver: %w", err)
	}

	return restConfig, nil
}

// Stop stops the underlying envtest.Environment and the GardenerAPIServer.
func (e *GardenerTestEnvironment) Stop() error {
	err := e.GardenerAPIServer.Stop()
	if err != nil {
		return err
	}
	return e.Environment.Stop()
}
