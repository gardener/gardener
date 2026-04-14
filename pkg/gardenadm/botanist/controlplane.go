// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Masterminds/semver/v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	bootstrapetcd "github.com/gardener/gardener/pkg/component/etcd/bootstrap"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	resourcemanagerconstants "github.com/gardener/gardener/pkg/component/gardener/resourcemanager/constants"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenadm/staticpod"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

// PathKubeconfig is the path to a file on the control plane node containing an admin kubeconfig.
var PathKubeconfig = filepath.Join(string(filepath.Separator), "etc", "kubernetes", "admin.conf")

// KubeconfigSecretName is the name of the secret in the shoot namespace of the bootstrap cluster containing the
// kubeconfig of the self-hosted shoot.
const KubeconfigSecretName = "kubeconfig"

func (b *GardenadmBotanist) deployETCD(role string) func(context.Context) error {
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

func (b *GardenadmBotanist) deployKubeAPIServer(ctx context.Context) error {
	b.Shoot.Components.ControlPlane.KubeAPIServer.EnableStaticTokenKubeconfig()
	b.Shoot.Components.ControlPlane.KubeAPIServer.SetAutoscalingReplicas(ptr.To[int32](0))

	// kube-apiserver must be able to resolve the gardener-resource-manager service IP to access the
	// node-agent-authorizer webhook. Thus, we fetch the service IP. It is used to create a host alias in the
	// kube-apiserver pod spec later. If the service does not exist yet, then kube-apiserver is bootstrapped for the
	// first time - in this case, we don't activate the authorizer webhook.
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: resourcemanagerconstants.ServiceName, Namespace: b.Shoot.ControlPlaneNamespace}}
	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(service), service); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed getting service %s: %w", client.ObjectKeyFromObject(service), err)
		}
	}

	b.gardenerResourceManagerServiceIPs = service.Spec.ClusterIPs
	enableNodeAgentAuthorizer := len(b.gardenerResourceManagerServiceIPs) > 0

	return b.DeployKubeAPIServer(ctx, enableNodeAgentAuthorizer)
}

type staticControlPlaneComponent struct {
	deploy       func(context.Context) error
	name         string
	targetObject client.Object
	mutate       func(*corev1.Pod)
}

func (b *GardenadmBotanist) allStaticControlPlaneComponents() []staticControlPlaneComponent {
	if len(b.staticControlPlaneComponents) > 0 {
		return b.staticControlPlaneComponents
	}

	var (
		mutateAPIServerPodFn = func(pod *corev1.Pod) {
			for _, ip := range b.gardenerResourceManagerServiceIPs {
				pod.Spec.HostAliases = append(pod.Spec.HostAliases, corev1.HostAlias{
					IP:        ip,
					Hostnames: []string{resourcemanagerconstants.ServiceName},
				})
			}
		}
		mutateETCDPodFn = func(pod *corev1.Pod) {
			// TODO(rfranzke): Currently, the Etcd API does not allow to specify additional volumes/mounts/env vars, so
			//  we are forced to inject the generic kubeconfig here (see https://github.com/gardener/etcd-druid/issues/1317).
			//  Once this is supported, remove this code and address the TODO in pkg/component/etcd/etcd/etcd.go.
			genericTokenKubeconfigSecret, _ := b.SecretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
			utilruntime.Must(gardenerutils.InjectGenericKubeconfig(pod, genericTokenKubeconfigSecret.Name, gardenerutils.NewShootAccessSecret(pod.Name, pod.Namespace).Secret.Name, etcd.ContainerNameBackupRestore))
			utilruntime.Must(kubernetesutils.VisitPodSpec(pod, func(podSpec *corev1.PodSpec) {
				kubernetesutils.VisitContainers(podSpec, func(container *corev1.Container) {
					if container.Name == etcd.ContainerNameBackupRestore {
						kubernetesutils.AddEnvVar(container, corev1.EnvVar{Name: "KUBECONFIG", Value: gardenerutils.PathGenericKubeconfig}, true)
					}
				})
			}))
		}
	)

	if !b.useEtcdManagedByDruid {
		b.staticControlPlaneComponents = append(b.staticControlPlaneComponents,
			staticControlPlaneComponent{b.deployETCD(v1beta1constants.ETCDRoleMain), bootstrapetcd.Name(v1beta1constants.ETCDRoleMain), &appsv1.StatefulSet{}, nil},
			staticControlPlaneComponent{b.deployETCD(v1beta1constants.ETCDRoleEvents), bootstrapetcd.Name(v1beta1constants.ETCDRoleEvents), &appsv1.StatefulSet{}, nil},
		)
	} else {
		b.staticControlPlaneComponents = append(b.staticControlPlaneComponents,
			staticControlPlaneComponent{func(_ context.Context) error { return nil }, "etcd-" + v1beta1constants.ETCDRoleMain, &appsv1.StatefulSet{}, mutateETCDPodFn},
			staticControlPlaneComponent{func(_ context.Context) error { return nil }, "etcd-" + v1beta1constants.ETCDRoleEvents, &appsv1.StatefulSet{}, mutateETCDPodFn},
		)
	}

	b.staticControlPlaneComponents = append(b.staticControlPlaneComponents,
		staticControlPlaneComponent{b.deployKubeAPIServer, v1beta1constants.DeploymentNameKubeAPIServer, &appsv1.Deployment{}, mutateAPIServerPodFn},
		staticControlPlaneComponent{b.DeployKubeControllerManager, v1beta1constants.DeploymentNameKubeControllerManager, &appsv1.Deployment{}, nil},
		staticControlPlaneComponent{b.Shoot.Components.ControlPlane.KubeScheduler.Deploy, v1beta1constants.DeploymentNameKubeScheduler, &appsv1.Deployment{}, nil},
	)

	return b.staticControlPlaneComponents
}

// DeployControlPlaneDeployments deploys the deployments for the static control plane components. It also updates the
// OperatingSystemConfig, waits for it to be reconciled by the OS extension, and deploys the ManagedResource containing
// the Secret with OperatingSystemConfig for gardener-node-agent.
func (b *GardenadmBotanist) DeployControlPlaneDeployments(ctx context.Context) error {
	if err := b.deployControlPlaneDeployments(ctx); err != nil {
		return fmt.Errorf("failed deploying control plane deployments: %w", err)
	}

	if _, _, err := b.deployOperatingSystemConfig(ctx); err != nil {
		return fmt.Errorf("failed deploying OperatingSystemConfig: %w", err)
	}

	// waiting for the OSC ensures that we also pick up the status written by the OS extension (e.g., extensionUnits)
	if err := b.Shoot.Components.Extensions.OperatingSystemConfig.Wait(ctx); err != nil {
		return fmt.Errorf("failed waiting for OperatingSystemConfig to be ready: %w", err)
	}

	if err := b.DeployManagedResourceForGardenerNodeAgent(ctx); err != nil {
		return fmt.Errorf("failed deploying ManagedResource containing Secret with OperatingSystemConfig for gardener-node-agent: %w", err)
	}

	return nil
}

func (b *GardenadmBotanist) deployControlPlaneDeployments(ctx context.Context) error {
	for _, component := range b.allStaticControlPlaneComponents() {
		if err := b.deployControlPlaneComponent(ctx, component.deploy, component.targetObject, component.name); err != nil {
			return fmt.Errorf("failed deploying %q: %w", component.name, err)
		}
	}

	return nil
}

func (b *GardenadmBotanist) deployControlPlaneComponent(ctx context.Context, deploy func(context.Context) error, targetObject client.Object, componentName string) error {
	if err := deploy(ctx); err != nil {
		return fmt.Errorf("failed deploying component %q: %w", componentName, err)
	}

	targetObject.SetName(componentName)
	targetObject.SetNamespace(b.Shoot.ControlPlaneNamespace)
	return b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(targetObject), targetObject)
}

func (b *GardenadmBotanist) populateStaticAdminTokenToAccessTokenSecret(ctx context.Context, componentName string) error {
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

type staticPod struct {
	name  string
	files []extensionsv1alpha1.File
}

type staticPods []staticPod

func (s staticPods) allFiles() []extensionsv1alpha1.File {
	var files []extensionsv1alpha1.File
	for _, pod := range s {
		files = append(files, pod.files...)
	}
	return files
}

func (b *GardenadmBotanist) staticControlPlanePods(ctx context.Context) (staticPods, error) {
	var pods staticPods

	for _, component := range b.allStaticControlPlaneComponents() {
		if err := b.populateStaticAdminTokenToAccessTokenSecret(ctx, component.name); err != nil {
			return nil, fmt.Errorf("failed populating static admin token to access token secret for %q: %w", component.name, err)
		}

		files, _, err := staticpod.Translate(ctx, b.SeedClientSet.Client(), component.targetObject, component.mutate)
		if err != nil {
			return nil, fmt.Errorf("failed translating object of type %T for %q: %w", component.targetObject, component.name, err)
		}

		pods = append(pods, staticPod{
			name:  component.name,
			files: files,
		})
	}

	return pods, nil
}

// NewClientSetFromFile creates a client set from the specified kubeconfig file.
func NewClientSetFromFile(kubeconfigPath string, scheme *runtime.Scheme) (kubernetes.Interface, error) {
	return kubernetes.NewClientFromFile("", kubeconfigPath,
		kubernetes.WithClientOptions(client.Options{Scheme: scheme}),
		kubernetes.WithClientConnectionOptions(componentbaseconfigv1alpha1.ClientConnectionConfiguration{QPS: -1}),
		kubernetes.WithDisabledCachedClient(),
	)
}

// CreateClientSet creates a client set for the control plane.
func (b *GardenadmBotanist) CreateClientSet(ctx context.Context) (kubernetes.Interface, error) {
	pathKubeconfig := PathKubeconfig
	if path := os.Getenv("KUBECONFIG"); path != "" {
		pathKubeconfig = path
	}

	clientSet, err := NewClientSetFromFile(pathKubeconfig, kubernetes.SeedScheme)
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

// DiscoverKubernetesVersion discovers the Kubernetes version of the control plane.
func (b *GardenadmBotanist) DiscoverKubernetesVersion(clientSet kubernetes.Interface) (*semver.Version, error) {
	version, err := semver.NewVersion(clientSet.Version())
	if err != nil {
		return nil, fmt.Errorf("failed parsing semver version %q: %w", clientSet.Version(), err)
	}

	return version, nil
}

// FetchKubeconfig fetches the kubeconfig of the self-hosted shoot from the control plane machine via SSH and stores it
// in a secret in the shoot namespace of the bootstrap cluster for later retrieval. It also writes the kubeconfig to the
// given output writer, if any.
func (b *GardenadmBotanist) FetchKubeconfig(ctx context.Context, output io.Writer) error {
	kubeconfigBuffer := &bytes.Buffer{}
	if err := b.sshConnection.SCP.CopyFromRemotePassThru(ctx, kubeconfigBuffer, PathKubeconfig, nil); err != nil {
		return fmt.Errorf("error fetching kubeconfig: %w", err)
	}
	kubeconfigBytes := kubeconfigBuffer.Bytes()

	kubeconfigSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: KubeconfigSecretName, Namespace: b.Shoot.ControlPlaneNamespace}}
	_, err := controllerutils.CreateOrGetAndMergePatch(ctx, b.SeedClientSet.Client(), kubeconfigSecret, func() error {
		kubeconfigSecret.Data = map[string][]byte{
			secretsutils.DataKeyKubeconfig: kubeconfigBytes,
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error writing kubeconfig secret: %w", err)
	}
	b.Logger.Info("Stored kubeconfig of the self-hosted shoot in secret", "namespace", kubeconfigSecret.Namespace, "name", kubeconfigSecret.Name)

	if output != nil {
		if outputFile, ok := output.(*os.File); ok {
			b.Logger.Info("Writing kubeconfig of the self-hosted shoot to file", "path", outputFile.Name())
		}
		if _, err := output.Write(kubeconfigBytes); err != nil {
			return fmt.Errorf("error writing kubeconfig to output: %w", err)
		}
	}

	return nil
}
