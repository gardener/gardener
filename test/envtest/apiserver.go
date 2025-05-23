// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package envtest

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
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
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/restmapper"
	"k8s.io/klog/v2"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	apiserverapp "github.com/gardener/gardener/cmd/gardener-apiserver/app"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	"github.com/gardener/gardener/pkg/apiserver"
	"github.com/gardener/gardener/pkg/apiserver/features"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	netutils "github.com/gardener/gardener/pkg/utils/net"
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
	// Path is the path to the gardener-apiserver binary, can be set via TEST_ASSET_GARDENER_APISERVER.
	// If Path is unset, gardener-apiserver will be started in-process.
	Path string
	// SecurePort is the secure port that the APIServer should listen on.
	// If this is not specified, we default to a random free port on localhost.
	SecurePort int
	// Args is a list of arguments which will passed to the APIServer binary.
	// If not specified, the minimal set of arguments to run the APIServer will
	// be used.
	Args []string
	// StartTimeout, StopTimeout specify the time the APIServer is allowed to
	// take when starting and stopping before an error is emitted.
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

	// caCert is the certificate of the CA that signed the GardenerAPIServer's serving cert.
	caCert *secrets.Certificate
	// user is used to setup and register the GardenerAPIServer with the envtest kube-apiserver.
	user *envtest.AuthenticatedUser
	// listenURL is the URL we end up listening on.
	listenURL *url.URL
	// terminateFunc holds a func that will terminate this GardenerAPIServer.
	terminateFunc func()
	// exited is a channel that will be closed, when this GardenerAPIServer exits.
	exited chan struct{}
}

// Start brings up the GardenerAPIServer, waits for it to be healthy and registers Gardener's APIs.
func (g *GardenerAPIServer) Start() error {
	features.RegisterFeatureGates()

	if err := g.defaultSettings(); err != nil {
		return err
	}

	g.exited = make(chan struct{})
	if g.Path != "" {
		if err := g.runAPIServerBinary(); err != nil {
			return err
		}
	} else {
		if err := g.runAPIServerInProcess(); err != nil {
			return err
		}
	}

	startCtx, cancel := context.WithTimeout(context.Background(), g.StartTimeout)
	defer cancel()

	// TODO: retry starting GardenerAPIServer on failure
	if err := g.waitUntilHealthy(startCtx); err != nil {
		return fmt.Errorf("gardener-apiserver didn't get healthy: %w", err)
	}

	log.V(1).Info("Registering Gardener APIs")
	if err := g.registerGardenerAPIs(startCtx); err != nil {
		return fmt.Errorf("failed registering Gardener APIs: %w", err)
	}
	return nil
}

func (g *GardenerAPIServer) runAPIServerBinary() error {
	log.V(1).Info("Starting gardener-apiserver", "path", g.Path, "args", g.Args)
	command := exec.Command(g.Path, g.Args...) // #nosec: G204 -- Test only.
	session, err := gexec.Start(command, g.Out, g.Err)
	if err != nil {
		return err
	}

	g.terminateFunc = func() {
		session.Terminate()
	}
	go func() {
		<-session.Exited
		close(g.exited)
	}()

	return nil
}

func (g *GardenerAPIServer) runAPIServerInProcess() error {
	ctx, cancel := context.WithCancel(context.Background())
	g.terminateFunc = cancel

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
	if g.Err == nil {
		// a nil writer causes klog to panic
		g.Err = io.Discard
	}
	// --logtostderr defaults to true, which will cause klog to log to stderr even if we set a different output writer
	g.Args = append(g.Args, "--logtostderr=false")
	klog.SetOutput(g.Err)

	log.V(1).Info("Starting gardener-apiserver", "args", g.Args)
	if err := pflagSet.Parse(g.Args); err != nil {
		return err
	}

	if err := opts.Validate(); err != nil {
		return err
	}

	go func() {
		if err := opts.Run(ctx); err != nil {
			log.Error(err, "gardener-apiserver exited with error")
		}
		close(g.exited)
	}()

	return nil
}

// defaultSettings applies defaults to this GardenerAPIServer's settings.
func (g *GardenerAPIServer) defaultSettings() error {
	var err error
	if g.EtcdURL == nil {
		return errors.New("expected EtcdURL to be configured")
	}

	// resolve localhost IP (pin to IPv4)
	addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort("localhost", "0"))
	if err != nil {
		return err
	}

	if g.CertDir == "" {
		_, ca, dir, err := secrets.SelfGenerateTLSServerCertificate("gardener-apiserver",
			[]string{"localhost", "gardener-apiserver.kube-system.svc"}, []net.IP{net.ParseIP(addr.IP.String())})
		if err != nil {
			return err
		}
		g.CertDir = dir
		g.caCert = ca
	}

	if binPath := os.Getenv(envGardenerAPIServerBin); binPath != "" {
		g.Path = binPath
	}
	if g.Path != "" {
		_, err := os.Stat(g.Path)
		if err != nil {
			return fmt.Errorf("failed checking for gardener-apiserver binary under %q: %w", g.Path, err)
		}
		log.V(1).Info("Using pre-built gardener-apiserver test binary", "path", g.Path)
	}

	if g.SecurePort == 0 {
		g.SecurePort, _, err = netutils.SuggestPort("")
		if err != nil {
			return err
		}
	}

	g.listenURL = &url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(addr.IP.String(), strconv.Itoa(g.SecurePort)),
	}

	if g.HealthCheckEndpoint == "" {
		g.HealthCheckEndpoint = "/healthz"
	}

	kubeconfigFile, err := g.prepareKubeconfigFile()
	if err != nil {
		return err
	}

	g.Args = append([]string{
		"--bind-address=" + addr.IP.String(),
		"--etcd-servers=" + g.EtcdURL.String(),
		"--tls-cert-file=" + filepath.Join(g.CertDir, "tls.crt"),
		"--tls-private-key-file=" + filepath.Join(g.CertDir, "tls.key"),
		"--secure-port=" + strconv.Itoa(g.SecurePort),
		"--cluster-identity=envtest",
		"--authorization-always-allow-paths=" + g.HealthCheckEndpoint,
		"--authentication-kubeconfig=" + kubeconfigFile,
		"--authorization-kubeconfig=" + kubeconfigFile,
		"--kubeconfig=" + kubeconfigFile,
	}, g.Args...)

	return nil
}

// prepareKubeconfigFile marshals the test environments rest config to a kubeconfig file in the CertDir.
func (g *GardenerAPIServer) prepareKubeconfigFile() (string, error) {
	kubeconfigBytes, err := g.user.KubeConfig()
	if err != nil {
		return "", err
	}
	kubeconfigFile := filepath.Join(g.CertDir, "kubeconfig.yaml")

	return kubeconfigFile, os.WriteFile(kubeconfigFile, kubeconfigBytes, 0600)
}

// waitUntilHealthy waits for the HealthCheckEndpoint to return 200.
func (g *GardenerAPIServer) waitUntilHealthy(ctx context.Context) error {
	// setup secure http client
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(g.caCert.CertificatePEM)
	httpClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: certPool, MinVersion: tls.VersionTLS12}}}

	healthCheckURL := g.listenURL
	healthCheckURL.Path = g.HealthCheckEndpoint

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
		if stopErr := g.Stop(); stopErr != nil {
			log.Error(stopErr, "Failed stopping gardener-apiserver")
		}
	}
	return err
}

// registerGardenerAPIs registers GardenerAPIServer's APIs in the test environment and waits for them to be discoverable.
func (g *GardenerAPIServer) registerGardenerAPIs(ctx context.Context) error {
	c, err := client.New(g.user.Config(), client.Options{Scheme: kubernetes.GardenScheme})
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
	for _, gv := range apiserver.AllGardenerAPIGroupVersions {
		apiService := g.apiServiceForSchemeGroupVersion(service, gv)
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
		log.V(1).Info("All Gardener APIServices available")
		return retry.Ok()
	}); err != nil {
		return err
	}

	// wait for all APIGroupVersions to be discoverable
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(g.user.Config())
	if err != nil {
		return err
	}

	undiscoverableGardenerAPIGroups := make(sets.Set[string], len(apiserver.AllGardenerAPIGroupVersions))
	for _, gv := range apiserver.AllGardenerAPIGroupVersions {
		undiscoverableGardenerAPIGroups.Insert(gv.String())
	}

	if err := retry.Until(ctx, waitPollInterval, func(_ context.Context) (bool, error) {
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
			return retry.MinorError(fmt.Errorf("the following Gardener API GroupVersions are not discoverable: %v", sets.List(undiscoverableGardenerAPIGroups)))
		}
		log.V(1).Info("All Gardener APIs discoverable")
		return retry.Ok()
	}); err != nil {
		return err
	}

	// ensure that we can really list objects in the Gardener API
	// after https://github.com/kubernetes/kubernetes/pull/119824 (first available in v0.28.3), we have seen that GAPI
	// sometimes fails to communicate with etcd even if we have passed all prior checks here, see
	// https://github.com/gardener/gardener/pull/8666
	// TODO: Revisit this once sigs.k8s.io/controller-runtime has upgraded their envtest version to v1.28.3+ (currently,
	//  only v1.28.0 is used, hence the kube-apiserver does not yet suffer from the same issue).
	return retry.Until(ctx, waitPollInterval, func(ctx context.Context) (bool, error) {
		for _, gvk := range []schema.GroupVersionKind{
			gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"),
			operationsv1alpha1.SchemeGroupVersion.WithKind("BastionList"),
			seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeedList"),
			settingsv1alpha1.SchemeGroupVersion.WithKind("OpenIDConnectPresetList"),
		} {
			objList := &metav1.PartialObjectMetadataList{}
			objList.SetGroupVersionKind(gvk)
			if err := c.List(ctx, objList, client.Limit(1)); err != nil {
				return retry.MinorError(err)
			}
			log.V(1).Info("Listing resources is possible", "gvk", gvk)
		}
		return retry.Ok()
	})
}

func (g *GardenerAPIServer) apiServiceForSchemeGroupVersion(svc *corev1.Service, gv schema.GroupVersion) *apiregistrationv1.APIService {
	port := int32(g.SecurePort) // #nosec G115 -- Test code.
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
			CABundle:             g.caCert.CertificatePEM,
		},
	}
}

func apiServiceNameForSchemeGroupVersion(gv schema.GroupVersion) string {
	return gv.Version + "." + gv.Group
}

// Stop stops this GardenerAPIServer and cleans its temporary resources.
func (g *GardenerAPIServer) Stop() error {
	var errList []error

	// trigger stop procedure
	if g.terminateFunc != nil {
		g.terminateFunc()

		select {
		case <-g.exited:
			break
		case <-time.After(g.StopTimeout):
			errList = append(errList, errors.New("timeout waiting for gardener-apiserver to stop"))
		}
	}

	// cleanup temp dirs
	if g.CertDir != "" {
		if err := os.RemoveAll(g.CertDir); err != nil {
			errList = append(errList, err)
		}
	}

	return utilerrors.NewAggregate(errList)
}
