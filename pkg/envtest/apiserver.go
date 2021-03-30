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

package envtest

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/onsi/gomega/gexec"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/klog/v2"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiserverapp "github.com/gardener/gardener/cmd/gardener-apiserver/app"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/secrets"
)

const (
	envGardenerAPIServerBin = "TEST_ASSET_GARDENER_APISERVER"
	waitPollInterval        = 100 * time.Millisecond
)

// GardenerAPIServer knows how to start, register and stop a temporary gardener-apiserver instance.
type GardenerAPIServer struct {
	// EtcdURL is the etcd URL that the APIServer should connect to (defaults to the URL of the envtest etcd).
	EtcdURL *url.URL
	// CertDir is a path to a directory containing whatever certificates the APIServer needs.
	// If left unspecified, then Start will create a temporary directory and generate the needed
	// certs and Stop will clean it up.
	CertDir string
	// caCert is the certificate of the CA that signed the GardenerAPIServer's serving cert.
	caCert *secrets.Certificate
	// restConfig is used to setup and register the APIServer with the envtest kube-apiserver.
	restConfig *rest.Config
	// Path is the path to the gardener-apiserver binary, can be set via TEST_ASSET_GARDENER_APISERVER.
	// If Path is unset, gardener-apiserver will be started in-process.
	Path string
	// SecurePort is the secure port that the APIServer should listen on.
	// If this is not specified, we default to a random free port on localhost.
	SecurePort int
	// listenURL is the URL we end up listening on.
	listenURL *url.URL
	// Args is a list of arguments which will passed to the APIServer binary.
	// If not specified, the minimal set of arguments to run the APIServer will
	// be used.
	Args []string
	// StartTimeout, StopTimeout specify the time the APIServer is allowed to
	// take when starting and stoppping before an error is emitted.
	// If not specified, these default to 20 seconds.
	StartTimeout time.Duration
	StopTimeout  time.Duration
	// Out, Err specify where APIServer should write its StdOut, StdErr to.
	// If not specified, the output will be discarded.
	Out io.Writer
	Err io.Writer
	// HealthCheckEndpoint is the path of the healthcheck endpoint (defaults to "/healthz").
	// It will be polled until receiving http.StatusOK (or StartTimeout occurs), before
	// returning from Start.
	HealthCheckEndpoint string

	// terminateFunc holds a func that will terminate this GardenerAPIServer.
	terminateFunc func()
	// exited is a channel that will be closed, when this GardenerAPIServer exits.
	exited chan struct{}
}

// Start brings up the GardenerAPIServer, waits for it to be healthy and registers Gardener's APIs.
func (a *GardenerAPIServer) Start() error {
	if err := a.defaultSettings(); err != nil {
		return err
	}

	a.exited = make(chan struct{})
	if a.Path != "" {
		if err := a.runAPIServerBinary(); err != nil {
			return err
		}
	} else {
		if err := a.runAPIServerInProcess(); err != nil {
			return err
		}
	}

	startCtx, cancel := context.WithTimeout(context.Background(), a.StartTimeout)
	defer cancel()

	// TODO: retry starting GardenerAPIServer on failure
	if err := a.waitUntilHealthy(startCtx); err != nil {
		return fmt.Errorf("gardener-apiserver didn't get healthy: %w", err)
	}

	log.V(1).Info("registering Gardener APIs")
	if err := a.registerGardenerAPIs(startCtx); err != nil {
		return fmt.Errorf("failed registering Gardener APIs: %w", err)
	}
	return nil
}

func (a *GardenerAPIServer) runAPIServerBinary() error {
	log.V(1).Info("starting gardener-apiserver", "path", a.Path, "args", a.Args)
	command := exec.Command(a.Path, a.Args...)
	session, err := gexec.Start(command, a.Out, a.Err)
	if err != nil {
		return err
	}

	a.terminateFunc = func() {
		session.Terminate()
	}
	go func() {
		<-session.Exited
		close(a.exited)
	}()

	return nil
}

func (a *GardenerAPIServer) runAPIServerInProcess() error {
	ctx, cancel := context.WithCancel(context.Background())
	a.terminateFunc = cancel

	opts := apiserverapp.NewOptions()

	// arrange all the flags
	flagSet := flag.NewFlagSet("gardener-apiserver", flag.ExitOnError)
	klog.InitFlags(flagSet)
	pflagSet := pflag.NewFlagSet("gardener-apiserver", pflag.ExitOnError)
	opts.AddFlags(pflagSet)
	pflagSet.AddGoFlagSet(flagSet)

	// redirect all klog output to the given writer
	// this will thereby also redirect output of client-go and other libs used by the tested code,
	// meaning such logs will only be shown when tests are run with KUBEBUILDER_ATTACH_CONTROL_PLANE_OUTPUT=true or
	// Err is explicitly set.
	if a.Err == nil {
		// a nil writer causes klog to panic
		a.Err = ioutil.Discard
	}
	// --logtostderr defaults to true, which will cause klog to log to stderr even if we set a different output writer
	a.Args = append(a.Args, "--logtostderr=false")
	klog.SetOutput(a.Err)

	log.V(1).Info("starting gardener-apiserver", "args", a.Args)
	if err := pflagSet.Parse(a.Args); err != nil {
		return err
	}

	if err := opts.Validate(); err != nil {
		return err
	}

	go func() {
		if err := opts.Run(ctx); err != nil {
			log.Error(err, "gardener-apiserver exited with error")
		}
		close(a.exited)
	}()

	return nil
}

// defaultSettings applies defaults to this GardenerAPIServer's settings.
func (a *GardenerAPIServer) defaultSettings() error {
	var err error
	if a.EtcdURL == nil {
		return fmt.Errorf("expected EtcdURL to be configured")
	}

	if a.CertDir == "" {
		_, ca, dir, err := secrets.SelfGenerateTLSServerCertificate("gardener-apiserver",
			[]string{"localhost", "gardener-apiserver.kube-system.svc"}, []net.IP{net.ParseIP("127.0.0.1")})
		if err != nil {
			return err
		}
		a.CertDir = dir
		a.caCert = ca
	}

	if binPath := os.Getenv(envGardenerAPIServerBin); binPath != "" {
		a.Path = binPath
	}
	if a.Path != "" {
		_, err := os.Stat(a.Path)
		if err != nil {
			return fmt.Errorf("failed checking for gardener-apiserver binary under %q: %w", a.Path, err)
		}
		log.V(1).Info("using pre-built gardener-apiserver test binary", "path", a.Path)
	}

	if a.SecurePort == 0 {
		a.SecurePort, _, err = SuggestPort("")
		if err != nil {
			return err
		}
	}

	// resolve localhost IP (pin to IPv4)
	addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort("localhost", "0"))
	if err != nil {
		return err
	}
	a.listenURL = &url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(addr.IP.String(), strconv.Itoa(a.SecurePort)),
	}

	if a.HealthCheckEndpoint == "" {
		a.HealthCheckEndpoint = "/healthz"
	}

	kubeconfigFile, err := a.prepareKubeconfigFile()
	if err != nil {
		return err
	}

	a.Args = append([]string{
		"--bind-address=" + addr.IP.String(),
		"--etcd-servers=" + a.EtcdURL.String(),
		"--tls-cert-file=" + filepath.Join(a.CertDir, "tls.crt"),
		"--tls-private-key-file=" + filepath.Join(a.CertDir, "tls.key"),
		"--secure-port=" + fmt.Sprintf("%d", a.SecurePort),
		"--cluster-identity=envtest",
		"--authorization-always-allow-paths=" + a.HealthCheckEndpoint,
		"--authentication-kubeconfig=" + kubeconfigFile,
		"--authorization-kubeconfig=" + kubeconfigFile,
		"--kubeconfig=" + kubeconfigFile,
	}, a.Args...)

	return nil
}

// prepareKubeconfigFile marshals the test environments rest config to a kubeconfig file in the CertDir.
func (a *GardenerAPIServer) prepareKubeconfigFile() (string, error) {
	kubeconfigBytes, err := util.MarshalKubeconfigWithClientCertificate(a.restConfig, nil, nil)
	if err != nil {
		return "", err
	}
	kubeconfigFile := filepath.Join(a.CertDir, "kubeconfig.yaml")

	return kubeconfigFile, ioutil.WriteFile(kubeconfigFile, kubeconfigBytes, 0600)
}

// waitUntilHealthy waits for the HealthCheckEndpoint to return 200.
func (a *GardenerAPIServer) waitUntilHealthy(ctx context.Context) error {
	// setup secure http client
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(a.caCert.CertificatePEM)
	httpClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: certPool}}}

	healthCheckURL := a.listenURL
	healthCheckURL.Path = a.HealthCheckEndpoint

	err := retry.Until(ctx, waitPollInterval, func(context.Context) (bool, error) {
		res, err := httpClient.Get(healthCheckURL.String())
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				log.V(1).Info("gardener-apiserver got healthy")
				return retry.Ok()
			}
		}
		return retry.MinorError(err)
	})
	if err != nil {
		if stopErr := a.Stop(); stopErr != nil {
			log.Error(stopErr, "failed stopping gardener-apiserver")
		}
	}
	return err
}

var allGardenerAPIGroupVersions = []schema.GroupVersion{
	gardencorev1beta1.SchemeGroupVersion,
	gardencorev1alpha1.SchemeGroupVersion,
	settingsv1alpha1.SchemeGroupVersion,
	seedmanagementv1alpha1.SchemeGroupVersion,
}

// registerGardenerAPIs registers GardenerAPIServer's APIs in the test environment and waits for them to be discoverable.
func (a *GardenerAPIServer) registerGardenerAPIs(ctx context.Context) error {
	c, err := client.New(a.restConfig, client.Options{Scheme: kubernetes.GardenScheme})
	if err != nil {
		return err
	}

	// create ExternalName service pointing to localhost
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardener-apiserver",
			Namespace: metav1.NamespaceSystem,
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: "localhost",
		},
	}
	if err := c.Create(ctx, service); err != nil {
		return err
	}

	// create APIServices for all API GroupVersions served by GardenerAPIServer
	var allAPIServices []*apiregistrationv1.APIService
	for _, gv := range allGardenerAPIGroupVersions {
		apiService := a.apiServiceForSchemeGroupVersion(service, gv)
		allAPIServices = append(allAPIServices, apiService)
		if err := c.Create(ctx, apiService); err != nil {
			return err
		}
	}

	// wait for all the APIServices to be available
	if err := retry.Until(ctx, waitPollInterval, func(ctx context.Context) (bool, error) {
		for _, apiService := range allAPIServices {
			if err := c.Get(ctx, client.ObjectKeyFromObject(apiService), apiService); err != nil {
				return retry.MinorError(err)
			}
			if err := health.CheckAPIService(apiService); err != nil {
				return retry.MinorError(err)
			}
		}
		log.V(1).Info("all Gardener APIServices available")
		return retry.Ok()
	}); err != nil {
		return err
	}

	// wait for all APIGroupVersions to be discoverable
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(a.restConfig)
	if err != nil {
		return err
	}

	undiscoverableGardenerAPIGroups := make(sets.String, len(allGardenerAPIGroupVersions))
	for _, gv := range allGardenerAPIGroupVersions {
		undiscoverableGardenerAPIGroups.Insert(gv.String())
	}

	return retry.Until(ctx, waitPollInterval, func(ctx context.Context) (bool, error) {
		apiGroupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
		if err != nil {
			return retry.MinorError(err)
		}
		for _, apiGroup := range apiGroupResources {
			for apiVersion, resources := range apiGroup.VersionedResources {
				// wait for all APIGroupVersions discovery endpoints to be available and list at least one resource
				// otherwise the rest mapper will return no match errors shortly after registering gardener-apiserver
				if len(resources) > 0 {
					undiscoverableGardenerAPIGroups.Delete(apiGroup.Group.Name + "/" + apiVersion)
				}
			}
		}
		if undiscoverableGardenerAPIGroups.Len() > 0 {
			return retry.MinorError(fmt.Errorf("the following Gardener API GroupVersions are not discoverable: %v", undiscoverableGardenerAPIGroups.List()))
		}
		log.V(1).Info("all Gardener APIs discoverable")
		return retry.Ok()
	})
}

func (a *GardenerAPIServer) apiServiceForSchemeGroupVersion(svc *corev1.Service, gv schema.GroupVersion) *apiregistrationv1.APIService {
	port := int32(a.SecurePort)
	return &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: apiServiceNameForSchemeGroupVersion(gv),
		},
		Spec: apiregistrationv1.APIServiceSpec{
			Service: &apiregistrationv1.ServiceReference{
				Name:      svc.Name,
				Namespace: svc.Namespace,
				Port:      &port,
			},
			Group:                gv.Group,
			Version:              gv.Version,
			GroupPriorityMinimum: 100,
			VersionPriority:      100,
			CABundle:             a.caCert.CertificatePEM,
		},
	}
}

func apiServiceNameForSchemeGroupVersion(gv schema.GroupVersion) string {
	return gv.Version + "." + gv.Group
}

// Stop stops this GardenerAPIServer and cleans its temporary resources.
func (a *GardenerAPIServer) Stop() error {
	// trigger stop procedure
	a.terminateFunc()

	select {
	case <-a.exited:
		break
	case <-time.After(a.StopTimeout):
		return fmt.Errorf("timeout waiting for gardener-apiserver to stop")
	}

	// cleanup temp dirs
	if a.CertDir != "" {
		return os.RemoveAll(a.CertDir)
	}
	return nil
}
