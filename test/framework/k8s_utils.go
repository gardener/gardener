// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/test/utils/access"
)

// WaitUntilDaemonSetIsRunning waits until the daemon set with <daemonSetName> is running
func (f *CommonFramework) WaitUntilDaemonSetIsRunning(ctx context.Context, k8sClient client.Client, name, namespace string) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		daemonSet := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
		log := f.Logger.WithValues("daemonSet", client.ObjectKeyFromObject(daemonSet))

		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(daemonSet), daemonSet); err != nil {
			return retry.MinorError(err)
		}

		if err := health.CheckDaemonSet(daemonSet); err != nil {
			log.Info("Waiting for DaemonSet to be ready")
			return retry.MinorError(fmt.Errorf("daemon set %q is not healthy: %v", name, err))
		}

		log.Info("DaemonSet is ready now")
		return retry.Ok()
	})
}

// WaitUntilStatefulSetIsRunning waits until the stateful set with <statefulSetName> is running
func (f *CommonFramework) WaitUntilStatefulSetIsRunning(ctx context.Context, name, namespace string, c kubernetes.Interface) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
		log := f.Logger.WithValues("statefulSet", client.ObjectKeyFromObject(statefulSet))

		if err := c.Client().Get(ctx, client.ObjectKeyFromObject(statefulSet), statefulSet); err != nil {
			return retry.MinorError(err)
		}

		if err := health.CheckStatefulSet(statefulSet); err != nil {
			log.Info("Waiting for StatefulSet to be ready")
			return retry.MinorError(fmt.Errorf("stateful set %q is not healthy: %v", name, err))
		}

		log.Info("StatefulSet is ready now")
		return retry.Ok()
	})
}

// WaitUntilIngressIsReady waits until the given ingress is ready
func (f *CommonFramework) WaitUntilIngressIsReady(ctx context.Context, name string, namespace string, k8sClient kubernetes.Interface) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		ingress := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
		log := f.Logger.WithValues("ingress", client.ObjectKeyFromObject(ingress))

		if err := k8sClient.Client().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, ingress); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Waiting for ingress to be ready")
				return retry.MinorError(fmt.Errorf("ingress %q in namespace %q does not exist", name, namespace))
			}
			return retry.SevereError(err)
		}

		if len(ingress.Status.LoadBalancer.Ingress) > 0 {
			log.Info("Ingress is ready now")
			return retry.Ok()
		}

		log.Info("Waiting for Ingress to be ready")
		return retry.MinorError(fmt.Errorf("ingress %q in namespace %q is not healthy", name, namespace))
	})
}

// WaitUntilDeploymentIsReady waits until the given deployment is ready
func (f *CommonFramework) WaitUntilDeploymentIsReady(ctx context.Context, name string, namespace string, k8sClient kubernetes.Interface) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
		log := f.Logger.WithValues("deployment", client.ObjectKeyFromObject(deployment))

		if err := k8sClient.Client().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Waiting for Deployment to be ready")
				return retry.MinorError(fmt.Errorf("deployment %q in namespace %q does not exist", name, namespace))
			}
			return retry.SevereError(err)
		}

		if err := health.CheckDeployment(deployment); err != nil {
			log.Info("Waiting for Deployment to be ready")
			return retry.MinorError(fmt.Errorf("deployment %q in namespace %q is not healthy", name, namespace))
		}

		log.Info("Deployment is ready now")
		return retry.Ok()
	})
}

// WaitUntilDeploymentsWithLabelsIsReady wait until pod with labels <podLabels> is running
func (f *CommonFramework) WaitUntilDeploymentsWithLabelsIsReady(ctx context.Context, deploymentLabels labels.Selector, namespace string, k8sClient kubernetes.Interface) error {
	log := f.Logger.WithValues("labelSelector", client.MatchingLabelsSelector{Selector: deploymentLabels}.String())

	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		deployments := &appsv1.DeploymentList{}
		if err := k8sClient.Client().List(ctx, deployments, client.MatchingLabelsSelector{Selector: deploymentLabels}, client.InNamespace(namespace)); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Waiting for deployments to be ready")
				return retry.MinorError(fmt.Errorf("no deployments with labels '%s' exist", deploymentLabels.String()))
			}
			return retry.SevereError(err)
		}

		for _, deployment := range deployments.Items {
			err = health.CheckDeployment(&deployment)
			if err != nil {
				log.Info("Waiting for deployments to be ready")
				return retry.MinorError(fmt.Errorf("deployment %q is not healthy: %v", deployment.Name, err))
			}
		}

		log.Info("Deployments are ready now")
		return retry.Ok()
	})
}

// WaitUntilNamespaceIsDeleted waits until a namespace has been deleted
func (f *CommonFramework) WaitUntilNamespaceIsDeleted(ctx context.Context, k8sClient kubernetes.Interface, ns string) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (bool, error) {
		if err := k8sClient.Client().Get(ctx, client.ObjectKey{Name: ns}, &corev1.Namespace{}); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.MinorError(err)
		}
		return retry.MinorError(fmt.Errorf("namespace %q is not deleted yet", ns))
	})
}

// WaitForNNodesToBeHealthy waits for exactly <n> Nodes to be healthy within a given timeout
func WaitForNNodesToBeHealthy(ctx context.Context, k8sClient kubernetes.Interface, n int, timeout time.Duration) error {
	return WaitForNNodesToBeHealthyInWorkerPool(ctx, k8sClient, n, nil, timeout)
}

// WaitForNNodesToBeHealthyInWorkerPool waits for exactly <n> Nodes in a given worker pool to be healthy within a given timeout
func WaitForNNodesToBeHealthyInWorkerPool(ctx context.Context, k8sClient kubernetes.Interface, n int, workerGroup *string, timeout time.Duration) error {
	return retry.UntilTimeout(ctx, defaultPollInterval, timeout, func(ctx context.Context) (done bool, err error) {
		nodeList, err := GetAllNodesInWorkerPool(ctx, k8sClient, workerGroup)
		if err != nil {
			return retry.SevereError(err)
		}

		nodeCount := len(nodeList.Items)
		if nodeCount != n {
			return retry.MinorError(fmt.Errorf("waiting for %d nodes to be ready: only %d nodes registered in the cluster", n, nodeCount))
		}

		for _, node := range nodeList.Items {
			if err := health.CheckNode(&node); err != nil {
				return retry.MinorError(fmt.Errorf("waiting for %d nodes to be ready: node %q is not healthy: %v", n, node.Name, err))
			}
		}

		return retry.Ok()
	})
}

// GetAllNodes fetches all nodes
func GetAllNodes(ctx context.Context, c kubernetes.Interface) (*corev1.NodeList, error) {
	return GetAllNodesInWorkerPool(ctx, c, nil)
}

// GetAllNodesInWorkerPool fetches all nodes of a specific worker group
func GetAllNodesInWorkerPool(ctx context.Context, c kubernetes.Interface, workerGroup *string) (*corev1.NodeList, error) {
	nodeList := &corev1.NodeList{}

	selectorOption := &client.MatchingLabelsSelector{}
	if workerGroup != nil && len(*workerGroup) > 0 {
		selectorOption.Selector = labels.SelectorFromSet(labels.Set{"worker.gardener.cloud/pool": *workerGroup})
	}

	err := c.Client().List(ctx, nodeList, selectorOption)
	return nodeList, err
}

// GetPodsByLabels fetches all pods with the desired set of labels <labelsMap>
func GetPodsByLabels(ctx context.Context, labelsSelector labels.Selector, c kubernetes.Interface, namespace string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	err := c.Client().List(ctx, podList,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: labelsSelector})
	if err != nil {
		return nil, err
	}
	return podList, nil
}

// GetFirstRunningPodWithLabels fetches the first running pod with the desired set of labels <labelsMap>
func GetFirstRunningPodWithLabels(ctx context.Context, labelsMap labels.Selector, namespace string, client kubernetes.Interface) (*corev1.Pod, error) {
	var (
		podList *corev1.PodList
		err     error
	)
	podList, err = GetPodsByLabels(ctx, labelsMap, client, namespace)
	if err != nil {
		return nil, err
	}
	if len(podList.Items) == 0 {
		return nil, ErrNoRunningPodsFound
	}

	for _, pod := range podList.Items {
		if health.IsPodReady(&pod) {
			return &pod, nil
		}
	}

	return nil, ErrNoRunningPodsFound
}

// PodExecByLabel executes a command inside pods filtered by label
func PodExecByLabel(ctx context.Context, client kubernetes.Interface, namespace string, podLabels labels.Selector, podContainer string, command ...string) (io.Reader, io.Reader, error) {
	pod, err := GetFirstRunningPodWithLabels(ctx, podLabels, namespace, client)
	if err != nil {
		return nil, nil, err
	}

	return client.PodExecutor().Execute(ctx, pod.Namespace, pod.Name, podContainer, command...)
}

// DeleteAndWaitForResource deletes a kubernetes resource and waits for its deletion
func DeleteAndWaitForResource(ctx context.Context, k8sClient kubernetes.Interface, resource client.Object, timeout time.Duration) error {
	if err := kubernetesutils.DeleteObject(ctx, k8sClient.Client(), resource); err != nil {
		return err
	}
	return retry.UntilTimeout(ctx, 5*time.Second, timeout, func(ctx context.Context) (done bool, err error) {
		newResource := resource.DeepCopyObject().(client.Object)
		if err := k8sClient.Client().Get(ctx, client.ObjectKeyFromObject(resource), newResource); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.MinorError(err)
		}
		return retry.MinorError(errors.New("object still exists"))
	})
}

// ScaleDeployment scales a deployment and waits until it is scaled
func ScaleDeployment(ctx context.Context, c client.Client, desiredReplicas *int32, name, namespace string) (*int32, error) {
	if desiredReplicas == nil {
		return nil, nil
	}

	replicas, err := GetDeploymentReplicas(ctx, c, namespace, name)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve the replica count of deployment %q: '%w'", name, err)
	}
	if replicas == nil || *replicas == *desiredReplicas {
		return replicas, nil
	}

	// scale the deployment
	if err := kubernetesutils.ScaleDeployment(ctx, c, client.ObjectKey{Namespace: namespace, Name: name}, *desiredReplicas); err != nil {
		return nil, fmt.Errorf("failed to scale the replica count of deployment %q: '%w'", name, err)
	}

	// wait until scaled
	if err := WaitUntilDeploymentScaled(ctx, c, namespace, name, *desiredReplicas); err != nil {
		return nil, fmt.Errorf("failed to wait until deployment %q is scaled: '%w'", name, err)
	}
	return replicas, nil
}

// WaitUntilDeploymentScaled waits until the deployment has the desired replica count in the status
func WaitUntilDeploymentScaled(ctx context.Context, c client.Client, namespace, name string, desiredReplicas int32) error {
	return retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		deployment := &appsv1.Deployment{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment); err != nil {
			return retry.SevereError(err)
		}
		if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != desiredReplicas {
			return retry.SevereError(fmt.Errorf("waiting for deployment scale failed. spec.replicas does not match the desired replicas"))
		}

		if deployment.Status.Replicas == desiredReplicas && deployment.Status.AvailableReplicas == desiredReplicas {
			return retry.Ok()
		}

		return retry.MinorError(fmt.Errorf("deployment currently has '%d' replicas. Desired: %d", deployment.Status.AvailableReplicas, desiredReplicas))
	})
}

// GetDeploymentReplicas gets the spec.Replicas count from a deployment
func GetDeploymentReplicas(ctx context.Context, c client.Client, namespace, name string) (*int32, error) {
	deployment := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment); err != nil {
		return nil, err
	}
	replicas := deployment.Spec.Replicas
	return replicas, nil
}

// ShootReconciliationSuccessful checks if a shoot is successfully reconciled. In case it is not, it also returns a descriptive message stating the reason.
func ShootReconciliationSuccessful(shoot *gardencorev1beta1.Shoot) (bool, string) {
	if shoot.Generation != shoot.Status.ObservedGeneration {
		return false, "shoot generation did not equal observed generation"
	}
	if len(shoot.Status.Conditions) == 0 && shoot.Status.LastOperation == nil {
		return false, "no conditions and last operation present yet"
	}

	workerlessShoot := v1beta1helper.IsWorkerless(shoot)
	shootConditions := sets.New(gardenerutils.GetShootConditionTypes(workerlessShoot)...)

	for _, condition := range shoot.Status.Conditions {
		if condition.Status != gardencorev1beta1.ConditionTrue {
			// Only return false if the status of a shoot condition is not True during hibernation. If the shoot also acts as a seed and
			// the `gardenlet` that operates the seed has already been shut down as part of the hibernation, the seed conditions will never
			// be updated to True if they were previously not True.
			hibernation := shoot.Spec.Hibernation
			if !shootConditions.Has(condition.Type) && hibernation != nil && ptr.Deref(hibernation.Enabled, false) {
				continue
			}
			return false, fmt.Sprintf("condition type %s is not true yet, had message %s with reason %s", condition.Type, condition.Message, condition.Reason)
		}
	}

	if shoot.Status.LastOperation != nil {
		if shoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeCreate ||
			shoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeReconcile ||
			shoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeRestore {
			if shoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded {
				return false, "last operation type was create, reconcile or restore but state was not succeeded"
			}
		} else if shoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate {
			return false, "last operation type was migrate, the migration process is not finished yet"
		}
	}

	return true, ""
}

// DownloadKubeconfig retrieves the static token kubeconfig for the given shoot and writes the kubeconfig to the
// given download path.
func DownloadKubeconfig(ctx context.Context, client kubernetes.Interface, namespace, name, downloadPath string) error {
	kubeconfig, err := GetObjectFromSecret(ctx, client, namespace, name, KubeconfigSecretKeyName)
	if err != nil {
		return err
	}
	err = os.WriteFile(downloadPath, []byte(kubeconfig), 0600)
	if err != nil {
		return err
	}

	return nil
}

// DownloadAdminKubeconfigForShoot requests an admin kubeconfig for the given shoot and writes the kubeconfig to the
// given download path. The kubeconfig expires in 6 hours.
func DownloadAdminKubeconfigForShoot(ctx context.Context, client kubernetes.Interface, shoot *gardencorev1beta1.Shoot, downloadPath string) error {
	const expirationSeconds int64 = 6 * 3600 // 6h
	kubeconfig, err := access.RequestAdminKubeconfigForShoot(ctx, client, shoot, ptr.To(expirationSeconds))
	if err != nil {
		return err
	}

	err = os.WriteFile(downloadPath, kubeconfig, 0600)
	if err != nil {
		return err
	}

	return nil
}

// PatchSecret patches the Secret.
func PatchSecret(ctx context.Context, c client.Client, secret *corev1.Secret) error {
	existingSecret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: secret.Namespace, Name: secret.Name}, existingSecret); err != nil {
		return err
	}
	patch := client.MergeFrom(existingSecret.DeepCopy())

	existingSecret.Data = secret.Data
	return c.Patch(ctx, existingSecret, patch)
}

// GetObjectFromSecret returns object from secret
func GetObjectFromSecret(ctx context.Context, k8sClient kubernetes.Interface, namespace, secretName, objectKey string) (string, error) {
	secret := &corev1.Secret{}
	err := k8sClient.Client().Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretName}, secret)
	if err != nil {
		return "", err
	}

	if _, ok := secret.Data[objectKey]; ok {
		return string(secret.Data[objectKey]), nil
	}
	return "", fmt.Errorf("secret %s/%s did not contain object key %q", namespace, secretName, objectKey)
}

// CreateTokenForServiceAccount requests a service account token.
func CreateTokenForServiceAccount(ctx context.Context, k8sClient kubernetes.Interface, serviceAccount *corev1.ServiceAccount, expirationSeconds *int64) (string, error) {
	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			ExpirationSeconds: expirationSeconds,
		},
	}

	if err := k8sClient.Client().SubResource("token").Create(ctx, serviceAccount, tokenRequest); err != nil {
		return "", err
	}

	return tokenRequest.Status.Token, nil
}

// NewClientFromServiceAccount returns a kubernetes client for a service account.
func NewClientFromServiceAccount(ctx context.Context, k8sClient kubernetes.Interface, serviceAccount *corev1.ServiceAccount) (kubernetes.Interface, error) {
	token, err := CreateTokenForServiceAccount(ctx, k8sClient, serviceAccount, ptr.To[int64](3600))
	if err != nil {
		return nil, err
	}

	restConfig := &rest.Config{
		Host: k8sClient.RESTConfig().Host,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: false,
			CAData:   k8sClient.RESTConfig().CAData,
		},
		BearerToken: token,
	}

	return kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(restConfig),
		kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.GardenScheme}),
		kubernetes.WithDisabledCachedClient(),
	)
}

// WaitUntilPodIsRunning waits until the pod with <podName> is running
func WaitUntilPodIsRunning(ctx context.Context, log logr.Logger, name, namespace string, c kubernetes.Interface) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
		podLog := log.WithValues("pod", client.ObjectKeyFromObject(pod))

		if err := c.Client().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, pod); err != nil {
			return retry.SevereError(err)
		}

		if !health.IsPodReady(pod) {
			podLog.Info("Waiting for Pod to be ready")
			return retry.MinorError(fmt.Errorf(`pod "%s/%s" is not ready: %v`, namespace, name, err))
		}

		podLog.Info("Pod is ready now")
		return retry.Ok()
	})
}

// WaitUntilPodIsRunningWithLabels waits until the pod with <podLabels> is running
func (f *CommonFramework) WaitUntilPodIsRunningWithLabels(ctx context.Context, labels labels.Selector, podNamespace string, c kubernetes.Interface) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		pod, err := GetFirstRunningPodWithLabels(ctx, labels, podNamespace, c)
		if err != nil {
			return retry.SevereError(err)
		}

		log := f.Logger.WithValues("pod", client.ObjectKeyFromObject(pod))

		if !health.IsPodReady(pod) {
			log.Info("Waiting for Pod to be ready")
			return retry.MinorError(fmt.Errorf(`pod "%s/%s" is not ready: %v`, pod.GetNamespace(), pod.GetName(), err))
		}

		log.Info("Pod is ready now")
		return retry.Ok()
	})
}

// DeployRootPod deploys a pod with root permissions for testing purposes.
func DeployRootPod(ctx context.Context, c client.Client, namespace string, nodename *string) (*corev1.Pod, error) {
	podPriority := int32(0)
	allowedCharacters := "0123456789abcdefghijklmnopqrstuvwxyz"
	id, err := utils.GenerateRandomStringFromCharset(3, allowedCharacters)
	if err != nil {
		return nil, err
	}

	rootPod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("rootpod-%s", id),
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "root-container",
					Image: "registry.k8s.io/e2e-test-images/busybox:1.36.1-1",
					Command: []string{
						"sleep",
						"10000000",
					},
					Resources:                corev1.ResourceRequirements{},
					TerminationMessagePath:   "/dev/termination-log",
					TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					ImagePullPolicy:          corev1.PullIfNotPresent,
					SecurityContext: &corev1.SecurityContext{
						Privileged: ptr.To(true),
					},
					Stdin: true,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "root-volume",
							MountPath: "/hostroot",
						},
					},
				},
			},
			HostNetwork: true,
			HostPID:     true,
			Priority:    &podPriority,
			Volumes: []corev1.Volume{
				{
					Name: "root-volume",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/",
						},
					},
				},
			},
		},
	}

	if nodename != nil {
		rootPod.Spec.NodeName = *nodename
	}

	if err := c.Create(ctx, &rootPod); err != nil {
		return nil, err
	}
	return &rootPod, nil
}
