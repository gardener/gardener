// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package terraformer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// TerraformerLabelKeyName is a key for label on a Terraformer Pod indicating the Terraformer name.
	TerraformerLabelKeyName = "terraformer.gardener.cloud/name"
	// TerraformerLabelKeyPurpose is a key for label on a Terraformer Pod indicating the Terraformer purpose.
	TerraformerLabelKeyPurpose = "terraformer.gardener.cloud/purpose"
)

type factory struct{}

func (factory) NewForConfig(logger logrus.FieldLogger, config *rest.Config, purpose, namespace, name, image string) (Terraformer, error) {
	return NewForConfig(logger, config, purpose, namespace, name, image)
}

func (f factory) New(logger logrus.FieldLogger, client client.Client, coreV1Client corev1client.CoreV1Interface, purpose, namespace, name, image string) Terraformer {
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
	logger logrus.FieldLogger,
	config *rest.Config,
	purpose,
	namespace,
	name,
	image string,
) (Terraformer, error) {
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
	logger logrus.FieldLogger,
	client client.Client,
	coreV1Client corev1client.CoreV1Interface,
	purpose,
	namespace,
	name,
	image string,
) Terraformer {
	var prefix = fmt.Sprintf("%s.%s", name, purpose)

	return &terraformer{
		logger:       logger,
		client:       client,
		coreV1Client: coreV1Client,

		name:      name,
		namespace: namespace,
		purpose:   purpose,
		image:     image,

		configName:    prefix + TerraformerConfigSuffix,
		variablesName: prefix + TerraformerVariablesSuffix,
		stateName:     prefix + TerraformerStateSuffix,

		terminationGracePeriodSeconds: int64(3600),

		deadlineCleaning: 10 * time.Minute,
		deadlinePod:      20 * time.Minute,
	}
}

// Apply executes a Terraform Pod by running the 'terraform apply' command.
func (t *terraformer) Apply() error {
	if !t.configurationDefined {
		return errors.New("terraformer configuration has not been defined, cannot execute the Terraform scripts")
	}
	return t.execute(context.TODO(), "apply")
}

// Destroy executes a Terraform Pod by running the 'terraform destroy' command.
func (t *terraformer) Destroy() error {
	if err := t.execute(context.TODO(), "destroy"); err != nil {
		return err
	}
	return t.CleanupConfiguration(context.TODO())
}

// execute creates a Terraform Pod which runs the provided scriptName (apply or destroy), waits for the Pod to be completed
// (either successful or not), prints its logs, deletes it and returns whether it was successful or not.
func (t *terraformer) execute(ctx context.Context, scriptName string) error {
	var (
		succeeded             = true  // Success status of the Terraform apply/destroy pod
		execute               = false // Should we skip the rest of the function depending on whether all ConfigMaps/Secrets exist/do not exist?
		skipApplyOrDestroyPod = false // Should we skip the execution of the Terraform apply/destroy command (actual execution of the Terraform config)?
	)

	// We should retry the preparation check in order to allow the kube-apiserver to actually create the ConfigMaps.
	if err := retry.UntilTimeout(ctx, 5*time.Second, 30*time.Second, func(ctx context.Context) (done bool, err error) {
		numberOfExistingResources, err := t.prepare(ctx)
		if err != nil {
			return retry.SevereError(err)
		}
		if numberOfExistingResources == 0 {
			t.logger.Debugf("All ConfigMaps/Secrets do not exist, can not execute the Terraform %s Pod.", scriptName)
			return retry.Ok()
		} else if numberOfExistingResources == numberOfConfigResources {
			t.logger.Debugf("All ConfigMaps/Secrets exist, will execute the Terraform %s Pod.", scriptName)
			execute = true
			return retry.Ok()
		} else {
			t.logger.Errorf("Can not execute Terraform %s Pod as ConfigMaps/Secrets are missing!", scriptName)
			return retry.MinorError(fmt.Errorf("%d/%d terraform resources are missing", numberOfConfigResources-numberOfExistingResources, numberOfConfigResources))
		}
	}); err != nil {
		return err
	}
	if !execute {
		return nil
	}

	// In case of scriptName == 'destroy', we need to first check whether the Terraform state contains
	// something at all. If it does not contain anything, then the 'apply' could never be executed, probably
	// because of syntax errors. In this case, we want to skip the Terraform destroy pod (as it wouldn't do anything
	// anyway) and just delete the related ConfigMaps/Secrets.
	if scriptName == "destroy" {
		skipApplyOrDestroyPod = t.IsStateEmpty()
	}

	if !skipApplyOrDestroyPod {
		// Create Terraform Pod which executes the provided scriptName
		generateName := t.computePodGenerateName(scriptName)
		pod, err := t.deployTerraformerPod(ctx, generateName, scriptName)
		if err != nil {
			return fmt.Errorf("failed to deploy the Terraformer Pod with .meta.generateName '%s': %s", generateName, err.Error())
		}

		t.logger.Infof("Successfully created Terraformer Pod '%s'.", pod.Name)

		// Wait for the Terraform apply/destroy Pod to be completed
		exitCode := t.waitForPod(ctx, pod.Name, t.deadlinePod)
		succeeded = exitCode == 0
		t.logger.Infof("Terraform Pod '%s' finished with exit code %d.", pod.Name, exitCode)
	}

	// Retrieve the logs of the apply/destroy Pods
	podList, err := t.listTerraformerPods(ctx)
	if err != nil {
		t.logger.Errorf("Could not retrieve list of pods belonging to Terraformer '%s': %s", t.name, err.Error())
		podList = &corev1.PodList{}
	}

	t.logger.Infof("Fetching the logs for all pods belonging to Terraformer '%s'...", t.name)
	logList, err := t.retrievePodLogs(podList)
	if err != nil {
		t.logger.Errorf("Could not retrieve the logs of the pods belonging to Terraformer '%s': %s", t.name, err.Error())
		logList = map[string]string{}
	}
	for podName, podLogs := range logList {
		t.logger.Infof("Logs of Pod '%s' belonging to Terraformer '%s':\n%s", podName, t.name, podLogs)
	}

	// Delete the Terraformer Pods
	t.logger.Infof("Cleaning up pods created by Terraformer '%s'...", t.name)
	if err := t.deleteTerraformerPods(ctx, podList); err != nil {
		return err
	}

	// Evaluate whether the execution was successful or not
	t.logger.Infof("Terraformer '%s' execution for command '%s' has been completed.", t.name, scriptName)
	if !succeeded {
		errorMessage := fmt.Sprintf("Terraform execution for command '%s' could not be completed.", scriptName)
		if terraformErrors := retrieveTerraformErrors(logList); terraformErrors != nil {
			errorMessage += fmt.Sprintf(" The following issues have been found in the logs:\n\n%s", strings.Join(terraformErrors, "\n\n"))
		}
		return gardencorev1beta1helper.DetermineError(errors.New(errorMessage), errorMessage)
	}
	return nil
}

const (
	terraformerName = "terraformer"
	rbacName        = "gardener.cloud:system:terraformer"
)

func (t *terraformer) createOrUpdateServiceAccount(ctx context.Context) error {
	serviceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: terraformerName}}
	_, err := controllerutil.CreateOrUpdate(ctx, t.client, serviceAccount, func() error {
		return nil
	})
	return err
}

func (t *terraformer) createOrUpdateRole(ctx context.Context) error {
	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: rbacName}}
	_, err := controllerutil.CreateOrUpdate(ctx, t.client, role, func() error {
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"*"},
			},
		}
		return nil
	})
	return err
}

func (t *terraformer) createOrUpdateRoleBinding(ctx context.Context) error {
	roleBinding := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: rbacName}}
	_, err := controllerutil.CreateOrUpdate(ctx, t.client, roleBinding, func() error {
		roleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     rbacName,
		}
		roleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      terraformerName,
				Namespace: t.namespace,
			},
		}
		return nil
	})
	return err
}

func (t *terraformer) createOrUpdateTerraformerAuth(ctx context.Context) error {
	if err := t.createOrUpdateServiceAccount(ctx); err != nil {
		return err
	}
	if err := t.createOrUpdateRole(ctx); err != nil {
		return err
	}
	return t.createOrUpdateRoleBinding(ctx)
}

func (t *terraformer) deployTerraformerPod(ctx context.Context, generateName, command string) (*corev1.Pod, error) {
	if err := t.createOrUpdateTerraformerAuth(ctx); err != nil {
		return nil, err
	}

	t.logger.Infof("Deploying Terraformer Pod with .meta.generateName '%s'.", generateName)
	podSpec := *t.podSpec(command)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: generateName,
			Namespace:    t.namespace,
			Labels: map[string]string{
				// Terraformer labels
				TerraformerLabelKeyName:    t.name,
				TerraformerLabelKeyPurpose: t.purpose,
				// Network policy labels
				v1beta1constants.LabelNetworkPolicyToDNS:             v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToPublicNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToSeedAPIServer:   v1beta1constants.LabelNetworkPolicyAllowed,
			},
		},
		Spec: podSpec,
	}

	err := t.client.Create(ctx, pod)
	return pod, err
}

func (t *terraformer) env() []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{Name: "MAX_BACKOFF_SEC", Value: "60"},
		{Name: "MAX_TIME_SEC", Value: "1800"},
		{Name: "TF_CONFIGURATION_CONFIG_MAP_NAME", Value: t.configName},
		{Name: "TF_STATE_CONFIG_MAP_NAME", Value: t.stateName},
		{Name: "TF_VARIABLES_SECRET_NAME", Value: t.variablesName},
	}
	for k, v := range t.variablesEnvironment {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}
	return envVars
}

func (t *terraformer) podSpec(command string) *corev1.PodSpec {
	terminationGracePeriodSeconds := t.terminationGracePeriodSeconds

	return &corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		Containers: []corev1.Container{
			{
				Name:            "terraform",
				Image:           t.image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{
					"/terraformer.sh",
					command,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("1.5Gi"),
					},
				},
				Env: t.env(),
			},
		},
		ServiceAccountName:            terraformerName,
		TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
	}
}

// listTerraformerPods lists all pods in the Terraformer namespace which have labels 'terraformer.gardener.cloud/name'
// and 'terraformer.gardener.cloud/purpose' matching the current Terraformer name and purpose.
func (t *terraformer) listTerraformerPods(ctx context.Context) (*corev1.PodList, error) {
	var (
		labels = map[string]string{
			TerraformerLabelKeyName:    t.name,
			TerraformerLabelKeyPurpose: t.purpose,
		}
		podList = &corev1.PodList{}
	)

	if err := t.client.List(ctx, podList,
		client.InNamespace(t.namespace),
		client.MatchingLabels(labels)); err != nil {
		return nil, err
	}
	return podList, nil
}

// retrievePodLogs fetches the logs of the created Pods by the Terraformer and returns them as a map whose
// keys are pod names and whose values are the corresponding logs.
func (t *terraformer) retrievePodLogs(podList *corev1.PodList) (map[string]string, error) {
	logChan := make(chan map[string]string, 1)
	go func() {
		var logList = map[string]string{}
		for _, pod := range podList.Items {
			name := pod.Name
			logs, err := kubernetes.GetPodLogs(t.coreV1Client.Pods(pod.Namespace), name, &corev1.PodLogOptions{})
			if err != nil {
				t.logger.Warnf("Could not retrieve the logs of Terraform pod %s: '%v'", name, err)
				continue
			}
			logList[name] = string(logs)
		}
		logChan <- logList
	}()

	select {
	case result := <-logChan:
		return result, nil
	case <-time.After(2 * time.Minute):
		return nil, fmt.Errorf("timeout when reading the logs of all pods created by Terraformer '%s'", t.name)
	}
}

func (t *terraformer) deleteTerraformerPods(ctx context.Context, podList *corev1.PodList) error {
	for _, pod := range podList.Items {
		if err := t.client.Delete(ctx, &pod); client.IgnoreNotFound(err) != nil {
			return err
		}

		t.logger.Infof("Deleted Terraform Pod '%s'", pod.Name)
	}

	return nil
}

func (t *terraformer) computePodGenerateName(command string) string {
	return fmt.Sprintf("%s.%s.tf-%s-", t.name, t.purpose, command)
}
