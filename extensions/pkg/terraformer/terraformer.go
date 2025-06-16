// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terraformer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	// LabelKeyName is a key for label on a Terraformer Pod indicating the Terraformer name.
	LabelKeyName = "terraformer.gardener.cloud/name"
	// LabelKeyPurpose is a key for label on a Terraformer Pod indicating the Terraformer purpose.
	LabelKeyPurpose = "terraformer.gardener.cloud/purpose"
)

type factory struct{}

func (factory) NewForConfig(logger logr.Logger, config *rest.Config, purpose, namespace, name, image string) (Terraformer, error) {
	return NewForConfig(logger, config, purpose, namespace, name, image)
}

func (f factory) New(logger logr.Logger, client client.Client, coreV1Client corev1client.CoreV1Interface, purpose, namespace, name, image string) Terraformer {
	return New(logger, client, coreV1Client, purpose, namespace, name, image)
}

func (f factory) DefaultInitializer(c client.Client, main, variables string, tfVars []byte, stateInitializer StateConfigMapInitializer) Initializer {
	return DefaultInitializer(c, main, variables, tfVars, stateInitializer)
}

// DefaultFactory returns the default factory.
func DefaultFactory() Factory {
	return factory{}
}

// NewForConfig creates a new Terraformer and its dependencies from the given configuration.
func NewForConfig(
	logger logr.Logger,
	config *rest.Config,
	purpose,
	namespace,
	name,
	image string,
) (
	Terraformer,
	error,
) {
	c, err := client.New(config, client.Options{})
	if err != nil {
		return nil, err
	}

	coreV1Client, err := corev1client.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return New(logger, c, coreV1Client, purpose, namespace, name, image), nil
}

// New takes a <logger>, a <k8sClient>, a string <purpose>, which describes for what the
// Terraformer is used, a <name>, a <namespace> in which the Terraformer will run, and the
// <image> name for the to-be-used Docker image. It returns a Terraformer interface with initialized
// values for the namespace and the names which will be used for all the stored resources like
// ConfigMaps/Secrets.
func New(
	logger logr.Logger,
	c client.Client,
	coreV1Client corev1client.CoreV1Interface,
	purpose,
	namespace,
	name,
	image string,
) Terraformer {
	var prefix = fmt.Sprintf("%s.%s", name, purpose)

	return &terraformer{
		logger:       logger.WithName("terraformer"),
		client:       c,
		coreV1Client: coreV1Client,

		name:      name,
		namespace: namespace,
		purpose:   purpose,
		image:     image,

		configName:    prefix + ConfigSuffix,
		variablesName: prefix + VariablesSuffix,
		stateName:     prefix + StateSuffix,

		logLevel:                      "info",
		terminationGracePeriodSeconds: int64(3600),

		deadlineCleaning:    20 * time.Minute,
		deadlinePod:         20 * time.Minute,
		deadlinePodCreation: 5 * time.Minute,

		useProjectedTokenMount: true,
	}
}

const (
	// CommandApply is a constant for the "apply" command.
	CommandApply = "apply"
	// CommandDestroy is a constant for the "destroy" command.
	CommandDestroy = "destroy"
)

// Apply executes a Terraform Pod by running the 'terraform apply' command.
func (t *terraformer) Apply(ctx context.Context) error {
	if !t.configurationInitialized {
		return errors.New("terraformer configuration has not been defined, cannot execute Terraformer")
	}
	return t.execute(ctx, CommandApply)
}

// Destroy executes a Terraform Pod by running the 'terraform destroy' command.
func (t *terraformer) Destroy(ctx context.Context) error {
	if err := t.execute(ctx, CommandDestroy); err != nil {
		return err
	}
	return t.CleanupConfiguration(ctx)
}

// execute creates a Terraform Pod which runs the provided scriptName (apply or destroy), waits for the Pod to be completed
// (either successful or not), prints its logs, deletes it and returns whether it was successful or not.
func (t *terraformer) execute(ctx context.Context, command string) error {
	logger := t.logger.WithValues("command", command)

	// When not both configuration and state were freshly initialized then we should check whether all configuration
	// resources still exist. If yes then we can safely continue. If nothing exists then we exit early and don't run the
	// pod. Otherwise, we might return an error in case we don't tolerate that resources are missing. We only tolerate
	// this when the command is 'destroy'. This is because the CleanupConfiguration() function could have already
	// deleted some of the resources (but not all). Hence, without the toleration we would end up in a deadlock and
	// manual action would be required.
	if !t.configurationInitialized || !t.stateInitialized {
		numberOfExistingResources, err := t.NumberOfResources(ctx)
		if err != nil {
			return err
		}

		switch {
		case numberOfExistingResources == numberOfConfigResources:
			logger.Info("All ConfigMaps and Secrets exist, will execute Terraformer pod")
		case numberOfExistingResources == 0:
			logger.Info("All ConfigMaps and Secrets missing, can not execute Terraformer pod")
			return nil
		case command != CommandDestroy:
			errResourcesMissing := fmt.Errorf("%d/%d Terraform resources are missing", numberOfConfigResources-numberOfExistingResources, numberOfConfigResources)
			logger.Error(errResourcesMissing, "Cannot execute Terraformer pod")
			return errResourcesMissing
		}
	}

	// Check if an existing Terraformer pod is still running. If yes, then adopt it. If no, then deploy a new pod (if
	// necessary).
	var (
		pod          *corev1.Pod
		deployNewPod = true
	)

	podList, err := t.listPods(ctx)
	if err != nil {
		return err
	}

	switch {
	case len(podList.Items) == 1:
		if cmd := getTerraformerCommand(&podList.Items[0]); cmd == command {
			// adopt still existing pod
			pod = &podList.Items[0]
			deployNewPod = false
		} else {
			// delete still existing pod and wait until it's gone
			oldPod := &podList.Items[0]
			logger.Info("Found old Terraformer pod with other command, ensuring cleanup", "command", cmd, "oldPod", oldPod)
			if err := t.EnsureCleanedUp(ctx); err != nil {
				return err
			}
		}

	case len(podList.Items) > 1:
		// unreachable
		logger.Error(fmt.Errorf("too many Terraformer pods"), "Unexpected number of still existing Terraformer pods", "numberOfPods", len(podList.Items))
		if err := t.EnsureCleanedUp(ctx); err != nil {
			return err
		}
	}

	// In case of command == 'destroy', we need to first check whether the Terraform state contains
	// something at all. If it does not contain anything, then the 'apply' could never be executed, probably
	// because of syntax errors. In this case, we want to skip the Terraform destroy pod (as it wouldn't do anything
	// anyway) and just delete the related ConfigMaps/Secrets.
	if command == CommandDestroy {
		deployNewPod = !t.IsStateEmpty(ctx)
	}

	if deployNewPod {
		// Create Terraform Pod which executes the provided command
		generateName := t.computePodGenerateName(command)

		logger.Info("Deploying Terraformer pod", "generateName", generateName)
		pod, err = t.deployTerraformerPod(ctx, generateName, command)
		if err != nil {
			return fmt.Errorf("failed to deploy the Terraformer pod with .meta.generateName %q: %w", generateName, err)
		}
		logger.Info("Successfully created Terraformer pod", "pod", client.ObjectKeyFromObject(pod))
	}

	if pod != nil {
		podLogger := logger.WithValues("pod", client.ObjectKey{Namespace: t.namespace, Name: pod.Name})

		// Wait for the Terraform apply/destroy Pod to be completed
		status, terminationMessage := t.waitForPod(ctx, logger, pod)
		switch status {
		case podStatusSucceeded:
			podLogger.Info("Terraformer pod finished successfully")
		case podStatusCreationTimeout:
			podLogger.Info("Terraformer pod creation timed out")
		default:
			podLogger.Info("Terraformer pod finished with error")

			if terminationMessage != "" {
				podLogger.Info("Terraformer pod terminated with message", "terminationMessage", terminationMessage)
			} else if ctxErr := ctx.Err(); ctxErr != nil {
				podLogger.Info("Context error", "err", ctxErr.Error())
			} else {
				// fall back to pod logs as termination message
				podLogger.Info("Fetching logs of Terraformer pod as termination message is empty")
				terminationMessage, err = t.retrievePodLogs(ctx, podLogger, pod)
				if err != nil {
					podLogger.Error(err, "Could not retrieve logs of Terraformer pod")
					return err
				}
				podLogger.Info("Terraformer pod terminated with logs", "logs", terminationMessage)
			}
		}

		switch {
		case errors.Is(ctx.Err(), context.Canceled):
			// If the context error is Canceled, the parent context has been canceled. Because the Terraform is fairly unstable
			// and interruptions may cause it to not properly store the state (ref https://github.com/hashicorp/terraform/issues/33358),
			// we will allow it to continue. The next reconciliation will adopt the running pod.
			podLogger.Info("Skipping Terraformer pod deletion because context was cancelled")
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			// If the context error is deadline exceeded, create a new context for deleting the pod since attempting to use the
			// original context will fail.
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()
			fallthrough
		default:
			podLogger.Info("Cleaning up Terraformer pod")
			if err := t.client.Delete(ctx, pod); client.IgnoreNotFound(err) != nil {
				return err
			}
		}

		if status != podStatusSucceeded {
			errorMessage := fmt.Sprintf("Terraform execution for command '%s' could not be completed", command)
			if terraformErrors := findTerraformErrors(terminationMessage); terraformErrors != "" {
				errorMessage += fmt.Sprintf(":\n\n%s", terraformErrors)
			}
			return errors.New(errorMessage)
		}
	}

	return nil
}

const (
	name     = "terraformer"
	rbacName = "gardener.cloud:system:terraformer"
)

func (t *terraformer) ensureServiceAccount(ctx context.Context) error {
	serviceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: name}}
	_, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, t.client, serviceAccount, func() error {
		if t.useProjectedTokenMount {
			serviceAccount.AutomountServiceAccountToken = ptr.To(false)
		} else {
			serviceAccount.AutomountServiceAccountToken = nil
		}
		return nil
	})
	return err
}

func (t *terraformer) ensureRole(ctx context.Context) error {
	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: rbacName}}
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, t.client, role, func() error {
		role.Rules = []rbacv1.PolicyRule{{
			APIGroups: []string{""},
			Resources: []string{"configmaps", "secrets"},
			Verbs:     []string{"create", "get", "list", "watch", "patch", "update"},
		}}
		return nil
	})
	return err
}

func (t *terraformer) ensureRoleBinding(ctx context.Context) error {
	roleBinding := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: rbacName}}
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, t.client, roleBinding, func() error {
		roleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     rbacName,
		}
		roleBinding.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      name,
			Namespace: t.namespace,
		}}
		return nil
	})
	return err
}

func (t *terraformer) ensureTerraformerAuth(ctx context.Context) error {
	if err := t.ensureServiceAccount(ctx); err != nil {
		return err
	}
	if err := t.ensureRole(ctx); err != nil {
		return err
	}
	return t.ensureRoleBinding(ctx)
}

func (t *terraformer) deployTerraformerPod(ctx context.Context, generateName, command string) (*corev1.Pod, error) {
	if err := t.ensureTerraformerAuth(ctx); err != nil {
		return nil, err
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: generateName,
			Namespace:    t.namespace,
			Labels: map[string]string{
				// Terraformer labels
				LabelKeyName:    t.name,
				LabelKeyPurpose: t.purpose,
				// Network policy labels
				v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToPrivateNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToPublicNetworks:   v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            "terraform",
				Image:           t.image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         t.computeTerraformerCommand(command),
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
				},
				Env:                    t.envVars,
				TerminationMessagePath: "/terraform-termination-log",
			}},
			PriorityClassName:             v1beta1constants.PriorityClassNameShootControlPlane300,
			RestartPolicy:                 corev1.RestartPolicyNever,
			ServiceAccountName:            name,
			TerminationGracePeriodSeconds: ptr.To(t.terminationGracePeriodSeconds),
		},
	}

	err := t.client.Create(ctx, pod)
	return pod, err
}

func (t *terraformer) computeTerraformerCommand(command string) []string {
	return []string{
		"/terraformer",
		command,
		"--zap-log-level=" + t.logLevel,
		"--configuration-configmap-name=" + t.configName,
		"--state-configmap-name=" + t.stateName,
		"--variables-secret-name=" + t.variablesName,
	}
}

func getTerraformerCommand(pod *corev1.Pod) string {
	if pod == nil {
		return ""
	}
	if len(pod.Spec.Containers) != 1 {
		return ""
	}
	if len(pod.Spec.Containers[0].Command) < 2 {
		return ""
	}
	return pod.Spec.Containers[0].Command[1]
}

// listTerraformerPods lists all pods in the Terraformer namespace which have labels 'terraformer.gardener.cloud/name'
// and 'terraformer.gardener.cloud/purpose' matching the current Terraformer name and purpose.
func (t *terraformer) listPods(ctx context.Context) (*corev1.PodList, error) {
	var (
		podList = &corev1.PodList{}
		labels  = map[string]string{LabelKeyName: t.name, LabelKeyPurpose: t.purpose}
	)

	if err := t.client.List(ctx, podList, client.InNamespace(t.namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return podList, nil
}

// retrievePodLogs fetches the logs of the created Pods by the Terraformer and returns them as a map whose
// keys are pod names and whose values are the corresponding logs.
func (t *terraformer) retrievePodLogs(ctx context.Context, logger logr.Logger, pod *corev1.Pod) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	logs, err := kubernetesutils.GetPodLogs(ctx, t.coreV1Client.Pods(pod.Namespace), pod.Name, &corev1.PodLogOptions{})
	if err != nil {
		logger.Error(err, "Could not retrieve the logs of Terraformer pod", "pod", client.ObjectKeyFromObject(pod))
		return "", err
	}

	return string(logs), nil
}

func (t *terraformer) deleteTerraformerPods(ctx context.Context, podList *corev1.PodList) error {
	for _, pod := range podList.Items {
		if err := t.client.Delete(ctx, &pod); client.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}

func (t *terraformer) computePodGenerateName(command string) string {
	return fmt.Sprintf("%s.%s.tf-%s-", t.name, t.purpose, command)
}
