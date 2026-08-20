// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	bootstrapetcd "github.com/gardener/gardener/pkg/component/etcd/bootstrap"
	"github.com/gardener/gardener/pkg/component/etcd/bootstrap/backuprestore"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/component/kubernetes/adminaccess"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	"github.com/gardener/gardener/pkg/gardenadm/staticpod"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// PathKubeconfig is the path to a file on the control plane node containing an admin kubeconfig.
var PathKubeconfig = filepath.Join(string(filepath.Separator), "etc", "kubernetes", "admin.conf")

type staticControlPlaneComponent struct {
	deploy       func(context.Context) error
	name         string
	targetObject client.Object
	mutate       func(*corev1.Pod)
}

func (b *Botanist) deployETCD(role string, bootstrapEtcdBackupPath string) func(context.Context) error {
	var portClient, portPeer, portMetrics int32 = 2379, 2380, 2381
	if role == v1beta1constants.ETCDRoleEvents {
		portClient, portPeer, portMetrics = etcdconstants.StaticPodPortEtcdEventsClient, 2383, 2384
	}

	return func(ctx context.Context) error {
		image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameEtcd)
		if err != nil {
			return fmt.Errorf("failed fetching image %s: %w", imagevector.ContainerImageNameEtcd, err)
		}

		var etcdBackupRestore *backuprestore.Config

		if role == v1beta1constants.ETCDRoleMain && bootstrapEtcdBackupPath != "" {
			etcdbrctlImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameEtcdBackupRestore)
			if err != nil {
				return fmt.Errorf("failed fetching image %s: %w", imagevector.ContainerImageNameEtcdBackupRestore, err)
			}

			etcdBackupRestore, err = backuprestore.ConfigFromBackupDataPath(bootstrapEtcdBackupPath, etcdbrctlImage.String())
			if err != nil {
				return fmt.Errorf("failed building backup-restore config from path %q: %w", bootstrapEtcdBackupPath, err)
			}
		}

		return bootstrapetcd.New(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, b.SecretsManager, bootstrapetcd.Values{
			Image:         image.String(),
			Role:          role,
			BackupRestore: etcdBackupRestore,
			PortClient:    portClient,
			PortPeer:      portPeer,
			PortMetrics:   portMetrics,
		}).Deploy(ctx)
	}
}

func (b *Botanist) deployKubeAPIServer(ctx context.Context, useShootAccessTokens bool) error {
	if !useShootAccessTokens {
		b.Shoot.Components.ControlPlane.KubeAPIServer.EnableStaticTokenKubeconfig()
	} else {
		// Usually, these secrets would be deleted by a `b.SecretsManager.Cleanup()` call. However, we don't run this
		// call in gardenlet yet. Hence, for now, let's explicitly delete them.
		// TODO(rfranzke): Remove this in favor of the `b.SecretsManager.Cleanup()` call at the end of the shoot
		//  reconciliation (see TODO statement there).
		if err := b.SeedClientSet.Client().DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(b.Shoot.ControlPlaneNamespace), client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(
			utils.MustNewRequirement(secretsmanager.LabelKeyManagedBy, selection.Equals, secretsmanager.LabelValueSecretsManager),
			utils.MustNewRequirement(secretsmanager.LabelKeyManagerIdentity, selection.Equals, b.SecretsManager.Identity()),
			utils.MustNewRequirement(secretsmanager.LabelKeyName, selection.In, kubeapiserver.SecretNameUserKubeconfig),
		)}); err != nil {
			return fmt.Errorf("failed to cleanup static token kubeconfig and user kubeconfig secrets: %w", err)
		}
	}

	b.Shoot.Components.ControlPlane.KubeAPIServer.SetAutoscalingReplicas(new(int32(0)))
	return b.DeployKubeAPIServer(ctx)
}

func (b *Botanist) staticControlPlaneComponents(useBootstrapEtcd, useShootAccessTokens bool, bootstrapEtcdBackupPath string) []staticControlPlaneComponent {
	var (
		components []staticControlPlaneComponent

		mutateETCDPodFn = func(pod *corev1.Pod) {
			// TODO(CaptainIRS): Remove this mutation once https://github.com/gardener/etcd-druid/issues/1317 is resolved,
			// as the kubeconfig volume and env var can then be specified directly via the Etcd CR API.
			genericTokenKubeconfigSecret, _ := b.SecretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
			utilruntime.Must(gardenerutils.InjectGenericKubeconfig(pod, genericTokenKubeconfigSecret.Name, gardenerutils.NewShootAccessSecret(pod.Name, pod.Namespace).Secret.Name, etcdconstants.ContainerNameBackupRestore))
			utilruntime.Must(kubernetesutils.VisitPodSpec(pod, func(podSpec *corev1.PodSpec) {
				kubernetesutils.VisitContainers(podSpec, func(container *corev1.Container) {
					if container.Name == etcdconstants.ContainerNameBackupRestore {
						kubernetesutils.AddEnvVar(container, corev1.EnvVar{Name: "KUBECONFIG", Value: gardenerutils.PathGenericKubeconfig}, true)
					}
				})
			}))
		}
	)

	if useBootstrapEtcd {
		components = append(components,
			staticControlPlaneComponent{b.deployETCD(v1beta1constants.ETCDRoleMain, bootstrapEtcdBackupPath), bootstrapetcd.Name(v1beta1constants.ETCDRoleMain), &appsv1.StatefulSet{}, nil},
			staticControlPlaneComponent{b.deployETCD(v1beta1constants.ETCDRoleEvents, bootstrapEtcdBackupPath), bootstrapetcd.Name(v1beta1constants.ETCDRoleEvents), &appsv1.StatefulSet{}, nil},
		)
	} else {
		components = append(components,
			staticControlPlaneComponent{func(_ context.Context) error { return nil }, "etcd-" + v1beta1constants.ETCDRoleMain, &appsv1.StatefulSet{}, mutateETCDPodFn},
			staticControlPlaneComponent{func(_ context.Context) error { return nil }, "etcd-" + v1beta1constants.ETCDRoleEvents, &appsv1.StatefulSet{}, mutateETCDPodFn},
		)
	}

	return append(components,
		staticControlPlaneComponent{func(ctx context.Context) error { return b.deployKubeAPIServer(ctx, useShootAccessTokens) }, v1beta1constants.DeploymentNameKubeAPIServer, &appsv1.Deployment{}, nil},
		staticControlPlaneComponent{b.DeployKubeControllerManager, v1beta1constants.DeploymentNameKubeControllerManager, &appsv1.Deployment{}, nil},
		staticControlPlaneComponent{b.Shoot.Components.ControlPlane.KubeScheduler.Deploy, v1beta1constants.DeploymentNameKubeScheduler, &appsv1.Deployment{}, nil},
	)
}

// DeployStaticControlPlaneDeployments deploys the deployments for the static control plane components. It also updates
// the OperatingSystemConfig, waits for it to be reconciled by the OS extension, and deploys the ManagedResource
// containing the Secret with OperatingSystemConfig for gardener-node-agent.
func (b *Botanist) DeployStaticControlPlaneDeployments(ctx context.Context, useBootstrapEtcd bool) error {
	useShootAccessTokens, err := b.useShootAccessTokensForSelfHostedShootControlPlane(ctx)
	if err != nil {
		return fmt.Errorf("failed to check whether static auth tokens for self-hosted shoot control plane components can be synced: %w", err)
	}

	if err := b.DeployControlPlaneDeployments(ctx, useBootstrapEtcd, useShootAccessTokens, ""); err != nil {
		return fmt.Errorf("failed deploying control plane deployments: %w", err)
	}

	if _, _, err := b.DeployOperatingSystemConfigWithStaticPods(ctx, useBootstrapEtcd, useShootAccessTokens, ""); err != nil {
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
func (b *Botanist) DeployControlPlaneDeployments(ctx context.Context, useBootstrapEtcd, useShootAccessTokens bool, bootstrapEtcdBackupPath string) error {
	for _, component := range b.staticControlPlaneComponents(useBootstrapEtcd, useShootAccessTokens, bootstrapEtcdBackupPath) {
		if err := b.deployControlPlaneComponent(ctx, component.deploy, component.targetObject, component.name, useShootAccessTokens); err != nil {
			return fmt.Errorf("failed deploying %q: %w", component.name, err)
		}
	}

	return nil
}

func (b *Botanist) deployControlPlaneComponent(ctx context.Context, deploy func(context.Context) error, targetObject client.Object, componentName string, useShootAccessTokens bool) error {
	if err := deploy(ctx); err != nil {
		return fmt.Errorf("failed deploying component %q: %w", componentName, err)
	}

	if !useShootAccessTokens {
		if err := b.populateStaticAdminTokenToAccessTokenSecret(ctx, componentName); err != nil {
			return fmt.Errorf("failed populating static admin token to access token secret for %q: %w", componentName, err)
		}
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

	if secret.Annotations[resourcesv1alpha1.ServiceAccountTokenRenewTimestamp] != "" {
		return nil // Do not overwrite dynamic access token with static token.
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

// DeployOperatingSystemConfigWithStaticPods deploys and waits for the OperatingSystemConfig containing the files for the control
// plane components running as static pods.
func (b *Botanist) DeployOperatingSystemConfigWithStaticPods(ctx context.Context, useBootstrapEtcd, useShootAccessTokens bool, bootstrapEtcdBackupPath string) (*operatingsystemconfig.Data, string, error) {
	pods, err := b.staticControlPlanePods(ctx, useBootstrapEtcd, useShootAccessTokens, bootstrapEtcdBackupPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed computing files for static control plane pods: %w", err)
	}

	appendAdminKubeconfigFunc := b.appendStaticAdminKubeconfigToFiles
	if useShootAccessTokens {
		appendAdminKubeconfigFunc = b.appendDynamicAdminKubeconfigToFiles
	}

	files, err := appendAdminKubeconfigFunc(pods.allFiles())
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
	metav1.SetMetaDataAnnotation(&osc.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
	metav1.SetMetaDataAnnotation(&osc.ObjectMeta, v1beta1constants.GardenerTimestamp, time.Now().UTC().Format(time.RFC3339Nano))
	if err := b.SeedClientSet.Client().Patch(ctx, osc, patch); err != nil {
		return nil, "", fmt.Errorf("failed patching OperatingSystemConfig with additional files for static control plane pods: %w", err)
	}

	if !useBootstrapEtcd {
		if err := b.Shoot.Components.Extensions.OperatingSystemConfig.Wait(ctx); err != nil {
			return nil, "", fmt.Errorf("failed waiting for OperatingSystemConfig to be ready: %w", err)
		}
	}

	return &oscData.Original, controlPlaneWorkerPool.Name, nil
}

func (b *Botanist) appendStaticAdminKubeconfigToFiles(files []extensionsv1alpha1.File) ([]extensionsv1alpha1.File, error) {
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

func (b *Botanist) appendDynamicAdminKubeconfigToFiles(files []extensionsv1alpha1.File) ([]extensionsv1alpha1.File, error) {
	clusterCABundleSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return nil, fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	kubeconfig := kubernetesutils.NewKubeconfig(
		b.Shoot.GetInfo().Status.TechnicalID,
		clientcmdv1.Cluster{Server: "localhost", CertificateAuthorityData: clusterCABundleSecret.Data[secretsutils.DataKeyCertificateBundle]},
		clientcmdv1.AuthInfo{TokenFile: adminaccess.PathOnControlPlaneNodes},
	)
	kubeconfig.Contexts[0].Context.Namespace = b.Shoot.ControlPlaneNamespace

	rawKubeconfig, err := runtime.Encode(clientcmdlatest.Codec, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed generating authorization webhook kubeconfig: %w", err)
	}

	return append(files, extensionsv1alpha1.File{
		Path:        PathKubeconfig,
		Permissions: new(uint32(0600)),
		Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64(rawKubeconfig)}},
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

func (b *Botanist) staticControlPlanePods(ctx context.Context, useBootstrapEtcd, useShootAccessTokens bool, bootstrapEtcdBackupPath string) (staticPods, error) {
	var pods staticPods

	for _, component := range b.staticControlPlaneComponents(useBootstrapEtcd, useShootAccessTokens, bootstrapEtcdBackupPath) {
		if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKey{Name: component.name, Namespace: b.Shoot.ControlPlaneNamespace}, component.targetObject); err != nil {
			return nil, fmt.Errorf("failed reading object for %q: %w", component.name, err)
		}

		files, _, err := staticpod.Translate(ctx, b.SeedClientSet.Client(), component.targetObject, component.mutate)
		if err != nil {
			return nil, fmt.Errorf("failed translating object of type %T for %q: %w", component.targetObject, component.name, err)
		}

		if useShootAccessTokens {
			// During bootstrapping we populate a static admin token to the shoot access token secrets used as volumes
			// in the control plane component deployments. Later on, after bootstrapping is done, we don't need this
			// anymore and can let gardener-resource-manager auto-rotate the shoot access tokens.
			// We use the token-sync mechanism in gardener-node-agent which reads these shoot access token secrets from
			// the cluster and writes them directly to the disk. Hence, we don't need another file in the
			// OperatingSystemConfig containing the shoot access token (which just gets stale after GRM auto-rotates
			// it).
			// "Unfortunately", the static pod translator naively returns a file for the shoot access token secret
			// volume in the pod spec. We drop this file here before returning it and persisting it in the OSC.
			files = slices.DeleteFunc(files, func(file extensionsv1alpha1.File) bool {
				return file.Path == staticpod.FilePathForProjectedVolumeItem(component.name, "kubeconfig", resourcesv1alpha1.DataKeyToken)
			})
		}

		pods = append(pods, staticPod{
			name:  component.name,
			files: files,
		})
	}

	return pods, nil
}

// UpdateNodeAgentSecretNameLabelsOnNodes updates the worker.gardener.cloud/gardener-node-agent-secret-name labels on
// the nodes to the current computed names. This might be needed because 'gardenadm init' does not use the true/fully
// defaulted Shoot spec (this is only decided after 'gardenadm connect' registers the Shoot with gardener-apiserver).
func (b *Botanist) UpdateNodeAgentSecretNameLabelsOnNodes(ctx context.Context) error {
	workerNameToOperatingSystemConfigMaps := b.Shoot.Components.Extensions.OperatingSystemConfig.WorkerPoolNameToOperatingSystemConfigsMap()

	nodeList := &metav1.PartialObjectMetadataList{}
	nodeList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("NodeList"))
	if err := b.SeedClientSet.Client().List(ctx, nodeList, client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(utils.MustNewRequirement(v1beta1constants.LabelWorkerPool, selection.Exists))}); err != nil {
		return fmt.Errorf("failed listing all nodes: %w", err)
	}

	tasks := make([]flow.TaskFn, 0, len(nodeList.Items))
	for _, node := range nodeList.Items {
		if desiredGardenerNodeAgentSecretName := workerNameToOperatingSystemConfigMaps[node.Labels[v1beta1constants.LabelWorkerPool]].Original.GardenerNodeAgentSecretName; node.Labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName] != desiredGardenerNodeAgentSecretName {
			tasks = append(tasks, func(ctx context.Context) error {
				patch := client.MergeFrom(node.DeepCopy())
				node.Labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName] = desiredGardenerNodeAgentSecretName
				return b.SeedClientSet.Client().Patch(ctx, &node, patch)
			})
		}
	}

	return flow.Parallel(tasks...)(ctx)
}

// useShootAccessTokensForSelfHostedShootControlPlane returns true for self-hosted shoots whose
// gardener-resource-manager deployment runs in the pod network. If the shoot status indicates that gardenadm
// instantiated the Botanist instance, it returns false, since only the shoot gardenlet is the only instance who should
// take care to disable the static token after initialization of the cluster (gardenadm init).
func (b *Botanist) useShootAccessTokensForSelfHostedShootControlPlane(ctx context.Context) (bool, error) {
	if !b.Shoot.IsSelfHosted() || b.Shoot.GetInfo().Status.Gardener.Name == "gardenadm" {
		return false, nil
	}

	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameGardenerResourceManager, Namespace: b.Shoot.ControlPlaneNamespace}}
	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("failed fetching deployment %s: %w", client.ObjectKeyFromObject(deployment), err)
		}
		return false, nil
	}

	return !deployment.Spec.Template.Spec.HostNetwork, nil
}
