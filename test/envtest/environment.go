// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package envtest

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("gardener").WithName("test-env")

const envUseExistingGardener = "USE_EXISTING_GARDENER"

// GardenerTestEnvironment wraps envtest.Environment and additionally starts, registers and stops an instance of
// gardener-apiserver in order to work with gardener resources in the test.
type GardenerTestEnvironment struct {
	*envtest.Environment

	// UseExistingGardener specifies whether to run the test against an existing gardener control plane.
	// If it is set to true, Start will skip starting a temporary control plane and gardener-apiserver, and connect to the
	// cluster targeted by $KUBECONFIG instead.
	// If it is unset, setting the USE_EXISTING_GARDENER env var to `true` will have the same effects.
	UseExistingGardener *bool

	// certDir contains the kube-apiserver certs (generated by controller-runtime's pkg/envtest) and the front-proxy
	// certs for the API server aggregation layer
	certDir          string
	aggregatorConfig AggregatorConfig

	// GardenerAPIServer knows how to start, register and stop a temporary gardener-apiserver instance.
	GardenerAPIServer *GardenerAPIServer
}

// Start starts the underlying envtest.Environment and the GardenerAPIServer.
func (e *GardenerTestEnvironment) Start() (*rest.Config, error) {
	if e.Environment == nil {
		e.Environment = &envtest.Environment{}
	}

	if e.useExistingGardener() {
		log.V(1).Info("Using existing gardener setup")
		e.Environment.UseExistingCluster = ptr.To(true)
	} else {
		// manage k-api cert dir by ourselves, we will add aggregator certs to it
		kubeAPIServer := e.Environment.ControlPlane.GetAPIServer()
		var err error
		e.certDir, err = os.MkdirTemp("", "k8s_test_framework_")
		if err != nil {
			return nil, err
		}
		kubeAPIServer.CertDir = e.certDir

		// configure kube-aggregator
		if err := e.aggregatorConfig.ConfigureAPIServerArgs(e.certDir, kubeAPIServer.Configure()); err != nil {
			return nil, err
		}
	}

	// start kube control plane
	log.V(1).Info("Starting envtest control plane")
	adminRestConfig, err := e.Environment.Start()
	if err != nil {
		return nil, err
	}

	if !e.useExistingGardener() {
		// start gardener API server
		if e.GardenerAPIServer == nil {
			e.GardenerAPIServer = &GardenerAPIServer{}
		}

		// add gardener-apiserver user
		gardenerAPIServerUser, err := e.Environment.ControlPlane.AddUser(envtest.User{
			Name: "gardener-apiserver",
			// TODO: bootstrap gardener RBAC and bind to ClusterRole/gardener.cloud:system:apiserver
			Groups: []string{"system:masters"},
		}, &rest.Config{
			// gotta go fast during tests -- we don't really care about overwhelming our test API server
			QPS:   1000.0,
			Burst: 2000.0,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to provision gardener-apiserver user: %w", err)
		}
		e.GardenerAPIServer.user = gardenerAPIServerUser

		// default GardenerAPIServer settings to the envtest ControlPlane settings
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
	}

	return adminRestConfig, nil
}

// Stop stops the underlying envtest.Environment and the GardenerAPIServer.
func (e *GardenerTestEnvironment) Stop() error {
	var errList []error

	if e.GardenerAPIServer != nil {
		log.V(1).Info("Stopping gardener-apiserver")
		if err := e.GardenerAPIServer.Stop(); err != nil {
			errList = append(errList, err)
		}
	}

	if e.Environment != nil {
		log.V(1).Info("Stopping envtest control plane")
		if err := e.Environment.Stop(); err != nil {
			errList = append(errList, err)
		}
	}

	if e.certDir != "" {
		if err := os.RemoveAll(e.certDir); err != nil {
			errList = append(errList, err)
		}
	}

	return utilerrors.Flatten(utilerrors.NewAggregate(errList))
}

func (e *GardenerTestEnvironment) useExistingGardener() bool {
	if e.UseExistingGardener == nil {
		return strings.ToLower(os.Getenv(envUseExistingGardener)) == "true"
	}
	return *e.UseExistingGardener
}

// GetK8SVersion returns the Kubernetes version used for running envtest.
func GetK8SVersion() (*semver.Version, error) {
	k8sVersion, ok := os.LookupEnv("ENVTEST_K8S_VERSION")
	if !ok {
		return nil, errors.New("error fetching k8s version from environment")
	}
	return semver.NewVersion(k8sVersion)
}
