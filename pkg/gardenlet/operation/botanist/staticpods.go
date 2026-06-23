// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	bootstrapetcd "github.com/gardener/gardener/pkg/component/etcd/bootstrap"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	"github.com/gardener/gardener/pkg/gardenadm/staticpod"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

// PathKubeconfig is the path to a file on the control plane node containing an admin kubeconfig.
var PathKubeconfig = filepath.Join(string(filepath.Separator), "etc", "kubernetes", "admin.conf")

type staticControlPlaneComponent struct {
	deploy       func(context.Context) error
	name         string
	targetObject client.Object
	mutate       func(*corev1.Pod)
}

func (b *Botanist) deployETCD(role string) func(context.Context) error {
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

func (b *Botanist) deployKubeAPIServer(ctx context.Context) error {
	b.Shoot.Components.ControlPlane.KubeAPIServer.EnableStaticTokenKubeconfig()
	b.Shoot.Components.ControlPlane.KubeAPIServer.SetAutoscalingReplicas(new(int32(0)))
	return b.DeployKubeAPIServer(ctx)
}

func (b *Botanist) staticControlPlaneComponents(useBootstrapEtcd bool) []staticControlPlaneComponent {
	var (
		components []staticControlPlaneComponent

		mutateETCDPodFn = func(pod *corev1.Pod) {
			// The pod name of the etcd pod must match `etcd-<role>-0`. However, when running as static pod, its pod name is
			// `etcd-<role>-<node-name>`. Hence, let's mutate it here for now (this might not be the final solution
			// depending on how support for highly-available etcd clusters will be implemented in the future).
			pod.Spec.Hostname = pod.Name + "-0"
			for i := range pod.Spec.Containers {
				kubernetesutils.AddEnvVar(&pod.Spec.Containers[i], corev1.EnvVar{Name: "POD_NAME", Value: pod.Spec.Hostname}, true)
			}
		}
	)

	if useBootstrapEtcd {
		components = append(components,
			staticControlPlaneComponent{b.deployETCD(v1beta1constants.ETCDRoleMain), bootstrapetcd.Name(v1beta1constants.ETCDRoleMain), &appsv1.StatefulSet{}, nil},
			staticControlPlaneComponent{b.deployETCD(v1beta1constants.ETCDRoleEvents), bootstrapetcd.Name(v1beta1constants.ETCDRoleEvents), &appsv1.StatefulSet{}, nil},
		)
	} else {
		components = append(components,
			staticControlPlaneComponent{func(_ context.Context) error { return nil }, "etcd-" + v1beta1constants.ETCDRoleMain, &appsv1.StatefulSet{}, mutateETCDPodFn},
			staticControlPlaneComponent{func(_ context.Context) error { return nil }, "etcd-" + v1beta1constants.ETCDRoleEvents, &appsv1.StatefulSet{}, mutateETCDPodFn},
		)
	}

	return append(components,
		staticControlPlaneComponent{b.deployKubeAPIServer, v1beta1constants.DeploymentNameKubeAPIServer, &appsv1.Deployment{}, nil},
		staticControlPlaneComponent{b.DeployKubeControllerManager, v1beta1constants.DeploymentNameKubeControllerManager, &appsv1.Deployment{}, nil},
		staticControlPlaneComponent{b.Shoot.Components.ControlPlane.KubeScheduler.Deploy, v1beta1constants.DeploymentNameKubeScheduler, &appsv1.Deployment{}, nil},
	)
}

// DeployStaticControlPlaneDeployments deploys the deployments for the static control plane components. It also updates
// the OperatingSystemConfig, waits for it to be reconciled by the OS extension, and deploys the ManagedResource
// containing the Secret with OperatingSystemConfig for gardener-node-agent.
func (b *Botanist) DeployStaticControlPlaneDeployments(ctx context.Context, useBootstrapEtcd bool) error {
	if err := b.DeployControlPlaneDeployments(ctx, useBootstrapEtcd); err != nil {
		return fmt.Errorf("failed deploying control plane deployments: %w", err)
	}

	if _, _, err := b.DeployOperatingSystemConfigWithStaticPods(ctx, useBootstrapEtcd); err != nil {
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

// DeployControlPlaneDeployments runs the Deploy function of the control plane components.
func (b *Botanist) DeployControlPlaneDeployments(ctx context.Context, useBootstrapEtcd bool) error {
	for _, component := range b.staticControlPlaneComponents(useBootstrapEtcd) {
		if err := b.deployControlPlaneComponent(ctx, component.deploy, component.targetObject, component.name); err != nil {
			return fmt.Errorf("failed deploying %q: %w", component.name, err)
		}
	}

	return nil
}

func (b *Botanist) deployControlPlaneComponent(ctx context.Context, deploy func(context.Context) error, targetObject client.Object, componentName string) error {
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

func (b *Botanist) populateStaticAdminTokenToAccessTokenSecret(ctx context.Context, componentName string) error {
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

// DeployOperatingSystemConfigWithStaticPods deploys the OperatingSystemConfig containing the files for the control
// plane components running as static pods.
func (b *Botanist) DeployOperatingSystemConfigWithStaticPods(ctx context.Context, useBootstrapEtcd bool) (*operatingsystemconfig.Data, string, error) {
	pods, err := b.staticControlPlanePods(ctx, useBootstrapEtcd)
	if err != nil {
		return nil, "", fmt.Errorf("failed computing files for static control plane pods: %w", err)
	}

	files, err := b.appendAdminKubeconfigToFiles(pods.allFiles())
	if err != nil {
		return nil, "", fmt.Errorf("failed appending admin kubeconfig to list of files: %w", err)
	}

	if err := b.DeployOperatingSystemConfig(ctx); err != nil {
		return nil, "", fmt.Errorf("failed deploying OperatingSystemConfig resource: %w", err)
	}

	controlPlaneWorkerPool := v1beta1helper.ControlPlaneWorkerPoolForShoot(b.Shoot.GetInfo().Spec.Provider.Workers)
	if controlPlaneWorkerPool == nil {
		return nil, "", fmt.Errorf("failed fetching the control plane worker pool for the shoot")
	}

	oscData, ok := b.Shoot.Components.Extensions.OperatingSystemConfig.WorkerPoolNameToOperatingSystemConfigsMap()[controlPlaneWorkerPool.Name]
	if !ok {
		return nil, "", fmt.Errorf("failed fetching the generated OperatingSystemConfig data for the control plane worker pool %q", controlPlaneWorkerPool.Name)
	}
	osc := oscData.Original.Object

	patch := client.MergeFrom(osc.DeepCopy())
	osc.Spec.Files = append(osc.Spec.Files, files...)
	if err := b.SeedClientSet.Client().Patch(ctx, osc, patch); err != nil {
		return nil, "", fmt.Errorf("failed patching OperatingSystemConfig with additional files for static control plane pods: %w", err)
	}

	return &oscData.Original, controlPlaneWorkerPool.Name, nil
}

func (b *Botanist) appendAdminKubeconfigToFiles(files []extensionsv1alpha1.File) ([]extensionsv1alpha1.File, error) {
	userKubeconfigSecret, ok := b.SecretsManager.Get(kubeapiserver.SecretNameUserKubeconfig)
	if !ok {
		return nil, fmt.Errorf("failed fetching secret %q", kubeapiserver.SecretNameUserKubeconfig)
	}

	return append(files, extensionsv1alpha1.File{
		Path:        PathKubeconfig,
		Permissions: new(uint32(0600)),
		Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64(userKubeconfigSecret.Data[secretsutils.DataKeyKubeconfig])}},
	}), nil
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

func (b *Botanist) staticControlPlanePods(ctx context.Context, useBootstrapEtcd bool) (staticPods, error) {
	var pods staticPods

	for _, component := range b.staticControlPlaneComponents(useBootstrapEtcd) {
		if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKey{Name: component.name, Namespace: b.Shoot.ControlPlaneNamespace}, component.targetObject); err != nil {
			return nil, fmt.Errorf("failed reading object for %q: %w", component.name, err)
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
