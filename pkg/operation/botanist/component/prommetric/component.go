// Package prommetric implements the Prometheus Metrics Adapter seed component. It consumes data from the aggregate seed
// Prometheus and exposes it in metrics server format. The adapter is registered as an extension service to the seed's
// kube-apiserver, serving custom metrics.
package prommetric

import (
	"bytes"
	"context"
	"embed"
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	utilerrors "github.com/gardener/gardener/pkg/utils/errors"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// HPlusVAutoscaler implements an extension apiservice to the seed kube-apiserver. It serves custom metrics based on
// data from the aggregate seed prometheus.
type PrometheusMetricsAdapter struct {
	namespaceName      string
	containerImageName string
	isEnabled          bool

	kubeClient              client.Client
	secretsManager          secretsmanager.Interface
	managedResourceRegistry *managedresources.Registry

	testIsolation prometheusMetricsAdapterTestIsolation // Provides indirections necessary to isolate the unit during tests
}

// Creates a new PrometheusMetricsAdapter instance tied to a specific server connection
func NewPrometheusMetricsAdapter(
	namespace string,
	containerImageName string,
	enabled bool,
	kubeClient client.Client,
	secretsManager secretsmanager.Interface) *PrometheusMetricsAdapter {

	return &PrometheusMetricsAdapter{
		namespaceName:           namespace,
		containerImageName:      containerImageName,
		isEnabled:               enabled,
		kubeClient:              kubeClient,
		secretsManager:          secretsManager,
		managedResourceRegistry: managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer),

		testIsolation: prometheusMetricsAdapterTestIsolation{
			DeployResourceConfigs: component.DeployResourceConfigs,
			DestroyResourceConfigs: func(
				ctx context.Context, clt client.Client, ns string, clusterType component.ClusterType, mrName string) error {

				return component.DestroyResourceConfigs(ctx, clt, ns, clusterType, mrName)
			},
		},
	}
}

// Implements component.Deployer.Deploy()
func (pma *PrometheusMetricsAdapter) Deploy(ctx context.Context) error {
	baseErrorMessage :=
		fmt.Sprintf("An error occurred while reconciling PrometheusMetricsAdapter component in namespace '%s' of the seed server",
			pma.namespaceName)

	if !pma.isEnabled {
		if err := pma.Destroy(ctx); err != nil {
			return fmt.Errorf(baseErrorMessage+
				" - failed to bring the PrometheusMetricsAdapter on the server to a disabled state. "+
				"The error message reported by the underlying operation follows: %w",
				err)
		}
		return nil
	}

	serverCertificateSecret, err := pma.deployServerCertificate(ctx)
	if err != nil {
		return fmt.Errorf(baseErrorMessage+
			" - failed to deploy the prommetric server TLS certificate to the seed server. "+
			"The error message reported by the underlying operation follows: %w",
			err)
	}

	resourceConfigs, err := getResourceConfigs(pma.namespaceName, pma.containerImageName, serverCertificateSecret)
	if err != nil {
		return fmt.Errorf(baseErrorMessage+
			" - failed to acquire the necessary resource config objects. "+
			"The error message reported by the underlying operation follows: %w",
			err)
	}

	err = pma.testIsolation.DeployResourceConfigs(
		ctx, pma.kubeClient, pma.namespaceName, component.ClusterTypeSeed, managedResourceName, pma.managedResourceRegistry, resourceConfigs)
	if err != nil {
		return fmt.Errorf(baseErrorMessage+
			" - failed to deploy the necessary resource config objects as a ManagedResource named '%s' to the server. "+
			"The error message reported by the underlying operation follows: %w",
			managedResourceName,
			err)
	}

	return nil
}

// Implements component.Deployer.Destroy()
func (pma *PrometheusMetricsAdapter) Destroy(ctx context.Context) error {
	if err := pma.testIsolation.DestroyResourceConfigs(
		ctx, pma.kubeClient, pma.namespaceName, component.ClusterTypeSeed, managedResourceName); err != nil {

		return fmt.Errorf(
			"An error occurred while removing the PrometheusMetricsAdapter component in namespace '%s' from the seed server"+
				" - failed to remove ManagedResource '%s'. "+
				"The error message reported by the underlying operation follows: %w",
			pma.namespaceName,
			managedResourceName,
			err)
	}

	return nil
}

// Implements component.Waiter.Wait()
func (pma *PrometheusMetricsAdapter) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, managedResourceTimeout)
	defer cancel()

	if err := managedresources.WaitUntilHealthy(timeoutCtx, pma.kubeClient, pma.namespaceName, managedResourceName); err != nil {
		return fmt.Errorf(
			"An error occurred while waiting for the deployment process of PrometheusMetricsAdapter component to "+
				"'%s' namespace in the seed server to finish and for the component to report ready status"+
				" - the wait for ManagedResource '%s' to become healty failed. "+
				"The error message reported by the underlying operation follows: %w",
			pma.namespaceName,
			managedResourceName,
			err)
	}

	return nil
}

// Implements component.Waiter.WaitCleanup()
func (pma *PrometheusMetricsAdapter) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, managedResourceTimeout)
	defer cancel()

	if err := managedresources.WaitUntilDeleted(timeoutCtx, pma.kubeClient, pma.namespaceName, managedResourceName); err != nil {
		return fmt.Errorf(
			"An error occurred while waiting for the PrometheusMetricsAdapter component to be fully removed from the "+
				"'%s' namespace in the seed server"+
				" - the wait for ManagedResource '%s' to be removed failed. "+
				"The error message reported by the underlying operation follows: %w",
			pma.namespaceName,
			managedResourceName,
			err)
	}

	return nil
}

const (
	componentBaseName           = "prometheus-metrics-adapter"
	deploymentName              = componentBaseName
	managedResourceName         = componentBaseName // The implementing artifacts are deployed to the seed via this MR
	serviceName                 = componentBaseName
	serverCertificateSecretName = componentBaseName + "-server" // PMA's HTTPS certificate
	managedResourceTimeout      = 2 * time.Minute               // Timeout for ManagedResources to become healthy or deleted
)

//go:embed templates/*.yaml
var resourceTemplateFiles embed.FS         // Templates for the k8s resources realising PMA
var resourceTemplates []*template.Template // resourceTemplateFiles loaded into Template objects

func init() {
	baseErrorMessage := "An error occurred while loading resource templates for the PrometheusMetricsAdapter component"

	// Load the k8s resource templates
	fs.WalkDir(resourceTemplateFiles, ".", func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf(baseErrorMessage+". The error message reported by the underlying operation follows: %w", err)
		}
		if dirEntry.IsDir() {
			return nil
		}

		bytes, err := resourceTemplateFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf(
				baseErrorMessage+" - reading file '%s' failed. "+
					"The error message reported by the underlying operation follows: %w",
				path,
				err)
		}

		template, err := template.
			New(dirEntry.Name()).
			Funcs(sprig.TxtFuncMap()).
			Parse(string(bytes))
		if err != nil {
			return fmt.Errorf(
				baseErrorMessage+" - parsing template file '%s' failed. "+
					"The error message reported by the underlying operation follows: %w",
				path,
				err)
		}

		resourceTemplates = append(resourceTemplates, template)
		return nil
	})
}

// prometheusMetricsAdapterTestIsolation contains all points of indirection necessary to isolate static function calls
// in the PrometheusMetricsAdapter unit during tests
type prometheusMetricsAdapterTestIsolation struct {
	// Points to component.DeployResourceConfigs()
	DeployResourceConfigs func(
		context.Context, client.Client, string, component.ClusterType, string, *managedresources.Registry, component.ResourceConfigs) error

	// Points to component.DestroyResourceConfigs()
	DestroyResourceConfigs func(context.Context, client.Client, string, component.ClusterType, string) error
}

// Deploys the PMA server TLS certificate to a secret and returns the name of the created secret
func (pma *PrometheusMetricsAdapter) deployServerCertificate(ctx context.Context) (*corev1.Secret, error) {
	const baseErrorMessage = "An error occurred while deploying server TLS certificate for the prometheus metrics adapter"

	_, found := pma.secretsManager.Get(v1beta1constants.SecretNameCASeed)
	if !found {
		return nil, fmt.Errorf(
			baseErrorMessage+
				" - the CA certificate, which is required to sign said server certificate, is missing. "+
				"The CA certificate was expected in the '%s' secret, but that secret was not found",
			v1beta1constants.SecretNameCASeed)
	}

	serverCertificateSecret, err := pma.secretsManager.Generate(
		ctx,
		&secretutils.CertificateSecretConfig{
			Name:                        serverCertificateSecretName,
			CommonName:                  fmt.Sprintf("%s.%s.svc", serviceName, pma.namespaceName),
			DNSNames:                    kutil.DNSNamesForService(serviceName, pma.namespaceName),
			CertType:                    secretutils.ServerCert,
			SkipPublishingCACertificate: true,
		},
		secretsmanager.SignedByCA(v1beta1constants.SecretNameCASeed, secretsmanager.UseCurrentCA),
		secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return nil, fmt.Errorf(
			baseErrorMessage+
				" - the attept to generate the certificate and store it in a secret named '%s' failed. "+
				"The error message reported by the underlying operation follows: %w",
			serverCertificateSecretName,
			err)
	}

	return serverCertificateSecret, nil
}

//#region Manifest data retrieval

// Reads and returns all objects from the specified manifestReader
func readManifest(manifestReader kubernetes.UnstructuredReader) ([]client.Object, error) {
	var objectsRead []client.Object
	allErrors := &multierror.Error{
		ErrorFormat: utilerrors.NewErrorFormatFuncWithPrefix("failed to read manifests for the prommetric component"),
	}

	for {
		obj, err := manifestReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			allErrors = multierror.Append(allErrors, fmt.Errorf("could not read object: %+v", err))
			continue
		}
		if obj == nil {
			continue
		}

		objectsRead = append(objectsRead, obj)
	}

	return objectsRead, allErrors.ErrorOrNil()
}

// Formats all prommetric manifests based on the specified parameters and returns them in the form of reader objects
func getManifests(
	namespaceName string, containerImageName string, serverCertificateSecret *corev1.Secret) ([]kubernetes.UnstructuredReader, error) {

	templateParams := struct {
		ContainerImageName string
		DeploymentName     string
		Namespace          string
		ServerSecretName   string
	}{
		ContainerImageName: containerImageName,
		DeploymentName:     deploymentName,
		Namespace:          namespaceName,
		ServerSecretName:   serverCertificateSecret.Name,
	}

	// Execute each manifest template and get object reader for the resulting raw output
	var formattedManifests []kubernetes.UnstructuredReader
	for i, template := range resourceTemplates {
		var formattedManifest bytes.Buffer
		if err := template.Execute(&formattedManifest, templateParams); err != nil {
			return nil, fmt.Errorf(
				"An error occurred while retrieving resource manifests for the prometheus metrics adapter component - "+
					"executing the template at index %d failed. "+
					"The error message reported by the underlying operation follows: %w",
				i,
				err)
		}
		formattedManifests = append(formattedManifests, kubernetes.NewManifestReader(formattedManifest.Bytes()))
	}

	return formattedManifests, nil
}

// Returns the resource configs which represent the seed resources required to support prommetric's operation
func getResourceConfigs(
	namespaceName string, containerImageName string, serverCertificateSecret *corev1.Secret) (component.ResourceConfigs, error) {

	const baseErrorMessage = "An error occurred while retrieving the list of resource config objects describing the " +
		"elements of the prometheus metrics adapter component"

	manifestReaders, err := getManifests(namespaceName, containerImageName, serverCertificateSecret)
	if err != nil {
		return nil, fmt.Errorf(baseErrorMessage+" - failed to retrieve manifest data. "+
			"The error message reported by the underlying operation follows: %w",
			err)
	}

	var allResources component.ResourceConfigs
	for i, manifest := range manifestReaders {
		manifestObjects, err := readManifest(manifest)
		if err != nil {
			return nil, fmt.Errorf(baseErrorMessage+" - failed to parse the manifest at index %d. "+
				"The error message reported by the underlying operation follows: %w",
				i,
				err)
		}

		for _, manifestObject := range manifestObjects {
			resourceConfig := component.ResourceConfig{
				Obj:   manifestObject,
				Class: component.Runtime,
			}
			allResources = append(allResources, resourceConfig)
		}
	}

	return allResources, nil
}

//#endregion Manifest data retrieval
