// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/Masterminds/semver/v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	bootstrapetcd "github.com/gardener/gardener/pkg/component/etcd/bootstrap"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	"github.com/gardener/gardener/pkg/gardenadm/staticpod"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

// PathKubeconfig is the path to a file on the control plane node containing an admin kubeconfig.
var PathKubeconfig = filepath.Join(string(filepath.Separator), "etc", "kubernetes", "admin.conf")

func (b *AutonomousBotanist) deployETCD(role string) func(context.Context) error {
	var portClient, portPeer, portMetrics int32 = 2379, 2380, 2381
	if role == v1beta1constants.ETCDRoleEvents {
		portClient, portPeer, portMetrics = etcdconstants.StaticPodPortEtcdEventsClient, 2383, 2384
	}

	return func(ctx context.Context) error {
		image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameEtcd)
		if err != nil {
			return fmt.Errorf("failed fetching image %s: %w", imagevector.ContainerImageNameEtcd, err)
		}

		return bootstrapetcd.New(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, b.SecretsManager, bootstrapetcd.Values{
			Image:       image.String(),
			Role:        role,
			PortClient:  portClient,
			PortPeer:    portPeer,
			PortMetrics: portMetrics,
		}).Deploy(ctx)
	}
}

func (b *AutonomousBotanist) deployKubeAPIServer(ctx context.Context) error {
	b.Shoot.Components.ControlPlane.KubeAPIServer.EnableStaticTokenKubeconfig()
	b.Shoot.Components.ControlPlane.KubeAPIServer.SetAutoscalingReplicas(ptr.To[int32](0))
	return b.DeployKubeAPIServer(ctx, false)
}

type staticControlPlaneComponent struct {
	deploy       func(context.Context) error
	name         string
	targetObject client.Object
}

func (b *AutonomousBotanist) staticControlPlaneComponents() []staticControlPlaneComponent {
	return []staticControlPlaneComponent{
		{b.deployETCD(v1beta1constants.ETCDRoleMain), "etcd-" + v1beta1constants.ETCDRoleMain + "-0", &appsv1.StatefulSet{}},
		{b.deployETCD(v1beta1constants.ETCDRoleEvents), "etcd-" + v1beta1constants.ETCDRoleEvents + "-0", &appsv1.StatefulSet{}},
		{b.deployKubeAPIServer, v1beta1constants.DeploymentNameKubeAPIServer, &appsv1.Deployment{}},
		{b.DeployKubeControllerManager, v1beta1constants.DeploymentNameKubeControllerManager, &appsv1.Deployment{}},
		{b.Shoot.Components.ControlPlane.KubeScheduler.Deploy, v1beta1constants.DeploymentNameKubeScheduler, &appsv1.Deployment{}},
	}
}

// DeployControlPlaneDeployments deploys the deployments for the static control plane components.
func (b *AutonomousBotanist) DeployControlPlaneDeployments(ctx context.Context) error {
	for _, component := range b.staticControlPlaneComponents() {
		if err := b.deployControlPlaneComponent(ctx, component.deploy, component.targetObject, component.name); err != nil {
			return fmt.Errorf("failed deploying %q: %w", component.name, err)
		}
	}

	return nil
}

func (b *AutonomousBotanist) deployControlPlaneComponent(ctx context.Context, deploy func(context.Context) error, targetObject client.Object, componentName string) error {
	if err := deploy(ctx); err != nil {
		return fmt.Errorf("failed deploying component %q: %w", componentName, err)
	}

	if err := b.populateStaticAdminTokenToAccessTokenSecret(ctx, componentName); err != nil {
		return fmt.Errorf("failed populating static admin token to access token secret for %q: %w", componentName, err)
	}

	targetObject.SetName(componentName)
	targetObject.SetNamespace(b.Shoot.ControlPlaneNamespace)
	return b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(targetObject), targetObject)
}

func (b *AutonomousBotanist) populateStaticAdminTokenToAccessTokenSecret(ctx context.Context, componentName string) error {
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: gardenerutils.SecretNamePrefixShootAccess + componentName, Namespace: b.Shoot.ControlPlaneNamespace}}
	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed reading secret %s for %q: %w", client.ObjectKeyFromObject(secret), componentName, err)
	}

	secretStaticToken, found := b.SecretsManager.Get(kubeapiserver.SecretStaticTokenName)
	if !found {
		return fmt.Errorf("secret %q not found", kubeapiserver.SecretStaticTokenName)
	}

	staticToken, err := secretsutils.LoadStaticTokenFromCSV(kubeapiserver.SecretStaticTokenName, secretStaticToken.Data[secretsutils.DataKeyStaticTokenCSV])
	if err != nil {
		return fmt.Errorf("failed loading static token from secret %q: %w", kubeapiserver.SecretStaticTokenName, err)
	}

	adminToken, err := staticToken.GetTokenForUsername(kubeapiserver.UserNameClusterAdmin)
	if err != nil {
		return fmt.Errorf("failed getting admin token from static token csv: %w", err)
	}

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data["token"] = []byte(adminToken.Token)

	return b.SeedClientSet.Client().Update(ctx, secret)
}

func (b *AutonomousBotanist) filesForStaticControlPlanePods(ctx context.Context) ([]extensionsv1alpha1.File, error) {
	var files []extensionsv1alpha1.File

	for _, component := range b.staticControlPlaneComponents() {
		if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKey{Name: component.name, Namespace: b.Shoot.ControlPlaneNamespace}, component.targetObject); err != nil {
			return nil, fmt.Errorf("failed reading object for %q: %w", component.name, err)
		}

		f, err := staticpod.Translate(ctx, b.SeedClientSet.Client(), component.targetObject)
		if err != nil {
			return nil, fmt.Errorf("failed translating object of type %T for %q: %w", component.targetObject, component.name, err)
		}

		files = append(files, f...)
	}

	return files, nil
}

// CreateClientSet creates a client set for the control plane.
func (b *AutonomousBotanist) CreateClientSet(ctx context.Context) (kubernetes.Interface, error) {
	clientSet, err := kubernetes.NewClientFromFile("", PathKubeconfig,
		kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.SeedScheme}),
		kubernetes.WithClientConnectionOptions(componentbaseconfigv1alpha1.ClientConnectionConfiguration{QPS: 100, Burst: 130}),
		kubernetes.WithDisabledCachedClient(),
	)
	if err != nil {
		b.Logger.Info("Waiting for kube-apiserver to start", "error", err.Error())
		return nil, fmt.Errorf("failed creating client set: %w", err)
	}

	clientSet.Start(ctx)

	waitContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if !clientSet.WaitForCacheSync(waitContext) {
		return nil, fmt.Errorf("timed out waiting for caches")
	}

	result := clientSet.RESTClient().Get().AbsPath("/readyz").Do(ctx)
	if result.Error() != nil {
		return nil, fmt.Errorf("failed to GET /readyz endpoint of kube-apiserver: %w", result.Error())
	}

	var statusCode int
	result.StatusCode(&statusCode)
	if statusCode != http.StatusOK {
		b.Logger.Info("The kube-apiserver does not report readiness yet", "statusCode", statusCode)
		return nil, fmt.Errorf("kube-apiserver does not report readiness yet")
	}

	return clientSet, nil
}

// NewClientFromBytes in alias for kubernetes.NewClientFromBytes.
// Exposed for unit testing.
var NewClientFromBytes = kubernetes.NewClientFromBytes

// DiscoverKubernetesVersion discovers the Kubernetes version of the control plane.
func (b *AutonomousBotanist) DiscoverKubernetesVersion(controlPlaneAddress string, caBundle []byte, token string) (*semver.Version, error) {
	kubeconfig, err := runtime.Encode(clientcmdlatest.Codec, kubernetesutils.NewKubeconfig(
		"local",
		clientcmdv1.Cluster{Server: controlPlaneAddress, CertificateAuthorityData: caBundle},
		clientcmdv1.AuthInfo{Token: token},
	))
	if err != nil {
		return nil, fmt.Errorf("unable to encode kubeconfig: %w", err)
	}

	clientSet, err := NewClientFromBytes(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed creating a new client: %w", err)
	}

	version, err := semver.NewVersion(clientSet.Version())
	if err != nil {
		return nil, fmt.Errorf("failed parsing semver version %q: %w", clientSet.Version(), err)
	}

	return version, nil
}
