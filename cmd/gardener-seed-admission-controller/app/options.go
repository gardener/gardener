// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package app

import (
	"fmt"

	"github.com/spf13/pflag"
)

type options struct {
	// bindAddress is the address the HTTP server should bind to.
	bindAddress string
	// port is the port that should be opened by the HTTP server.
	port int
	// serverCertDir is the path to server TLS cert and key.
	serverCertDir string
	// metricsBindAddress is the TCP address that the controller should bind to for serving prometheus metrics.
	// It can be set to "0" to disable the metrics serving.
	metricsBindAddress string
	// healthBindAddress is the TCP address that the controller should bind to for serving health probes.
	healthBindAddress string
	// enableProfiling enables profiling via web interface host:port/debug/pprof/.
	enableProfiling bool
	// enableContentionProfiling enables lock contention profiling, if enableProfiling is true.
	enableContentionProfiling bool
}

func (o *options) addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.bindAddress, "bind-address", "0.0.0.0", "Address to bind to")
	fs.IntVar(&o.port, "port", 9443, "Webhook server port")
	fs.StringVar(&o.serverCertDir, "tls-cert-dir", "", "Directory with server TLS certificate and key (must contain a tls.crt and tls.key file)")
	fs.StringVar(&o.metricsBindAddress, "metrics-bind-address", ":8080", "Bind address for the metrics server")
	fs.StringVar(&o.healthBindAddress, "health-bind-address", ":8081", "Bind address for the health server")
	fs.BoolVar(&o.enableProfiling, "profiling", false, "Enable profiling via web interface host:port/debug/pprof/")
	fs.BoolVar(&o.enableContentionProfiling, "contention-profiling", false, "Enable lock contention profiling, if profiling is enabled")
}

func (o *options) complete() error {
	return nil
}

func (o *options) validate() error {
	if len(o.bindAddress) == 0 {
		return fmt.Errorf("missing bind address")
	}

	if o.port == 0 {
		return fmt.Errorf("missing port")
	}

	if len(o.serverCertDir) == 0 {
		return fmt.Errorf("missing server tls cert path")
	}

	return nil
}
