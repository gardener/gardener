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
	"github.com/gardener/gardener/pkg/client/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"strings"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

const jobNameLabel = "job-name"

// NewForConfig creates a new Terraformer and its dependencies from the given configuration.
func NewForConfig(
	logger logrus.FieldLogger,
	config *rest.Config,
	purpose,
	namespace,
	name,
	image string,
) (*Terraformer, error) {
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
// <image> name for the to-be-used Docker image. It returns a Terraformer struct with initialized
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
) *Terraformer {
	var (
		prefix    = fmt.Sprintf("%s.%s", name, purpose)
		podSuffix = utils.ComputeSHA256Hex([]byte(time.Now().String()))[:5]
	)

	return &Terraformer{
		logger:       logger,
		client:       client,
		coreV1Client: coreV1Client,

		namespace: namespace,
		purpose:   purpose,
		image:     image,

		configName:    prefix + common.TerraformerConfigSuffix,
		variablesName: prefix + common.TerraformerVariablesSuffix,
		stateName:     prefix + common.TerraformerStateSuffix,
		podName:       fmt.Sprintf("%s-%s", prefix+common.TerraformerPodSuffix, podSuffix),
		jobName:       prefix + common.TerraformerJobSuffix,
	}
}

// Apply executes the Terraform Job by running the 'terraform apply' command.
func (t *Terraformer) Apply() error {
	if !t.configurationDefined {
		return errors.New("Terraformer configuration has not been defined, cannot execute the Terraform scripts")
	}
	return t.execute(context.TODO(), "apply")
}

// Destroy executes the Terraform Job by running the 'terraform destroy' command.
func (t *Terraformer) Destroy() error {
	if err := t.execute(context.TODO(), "destroy"); err != nil {
		return err
	}
	return t.cleanupConfiguration(context.TODO())
}

// execute creates a Terraform Job which runs the provided scriptName (apply or destroy), waits for the Job to be completed
// (either successful or not), prints its logs, deletes it and returns whether it was successful or not.
func (t *Terraformer) execute(ctx context.Context, scriptName string) error {
	var (
		exitCode  int32 = 1     // Exit code of the Terraform validation pod
		succeeded       = true  // Success status of the Terraform execution job
		execute         = false // Should we skip the rest of the function depending on whether all ConfigMaps/Secrets exist/do not exist?
		skipPod         = false // Should we skip the execution of the Terraform Pod (validation of the Terraform config)?
		skipJob         = false // Should we skip the execution of the Terraform Job (actual execution of the Terraform config)?
	)

	// We should retry the preparation check in order to allow the kube-apiserver to actually create the ConfigMaps.
	if err := wait.PollImmediate(5*time.Second, 30*time.Second, func() (bool, error) {
		numberOfExistingResources, err := t.prepare(ctx)
		if err != nil {
			return false, err
		}
		if numberOfExistingResources == 0 {
			t.logger.Debug("All ConfigMaps/Secrets do not exist, can not execute the Terraform Job.")
			return true, nil
		} else if numberOfExistingResources == numberOfConfigResources {
			t.logger.Debug("All ConfigMaps/Secrets exist, will execute the Terraform Job.")
			execute = true
			return true, nil
		} else {
			t.logger.Error("Can not execute Terraform Job as ConfigMaps/Secrets are missing!")
			return false, nil
		}
	}); err != nil {
		return err
	}
	if !execute {
		return nil
	}

	// In case of scriptName == 'destroy', we need to first check whether the Terraform state contains
	// something at all. If it does not contain anything, then the 'apply' could never be executed, probably
	// because of syntax errors. In this case, we want to skip the Terraform job (as it wouldn't do anything
	// anyway) and just delete the related ConfigMaps/Secrets.
	if scriptName == "destroy" {
		skipPod = true
		skipJob = t.isStateEmpty()
	}

	if !skipPod {

		if err := t.deployTerraformerPod(ctx, "validate"); err != nil {
			return err
		}

		// Wait for the Terraform validation Pod to be completed
		exitCode = t.waitForPod(ctx)
		skipJob = exitCode == 0 || exitCode == 1

		switch exitCode {
		case 0:
			t.logger.Debug("Terraform validation succeeded but there is no difference between state and actual resources.")
		case 1:
			t.logger.Debug("Terraform validation failed, will not start the job.")
			succeeded = false
		default:
			t.logger.Debug("Terraform validation has been successful.")
		}
	}

	if !skipJob {
		// Create Terraform Job which executes the provided scriptName
		if err := t.deployTerraformerJob(ctx, scriptName); err != nil {
			return fmt.Errorf("Failed to deploy the Terraformer: %s", err.Error())
		}

		// Wait for the Terraform Job to be completed
		succeeded = t.waitForJob(ctx)
		t.logger.Infof("Terraform '%s' finished.", t.jobName)
	}

	// Retrieve the logs of the Pods belonging to the completed Job
	jobPodList, err := t.listJobPods(ctx)
	if err != nil {
		t.logger.Errorf("Could not retrieve list of pods belonging to Terraform job '%s': %s", t.jobName, err.Error())
		jobPodList = &corev1.PodList{}
	}

	t.logger.Infof("Fetching the logs for all pods belonging to the Terraform job '%s'...", t.jobName)
	logList, err := t.retrievePodLogs(jobPodList)
	if err != nil {
		t.logger.Errorf("Could not retrieve the logs of the pods belonging to Terraform job '%s': %s", t.jobName, err.Error())
		logList = map[string]string{}
	}
	for podName, podLogs := range logList {
		t.logger.Infof("Logs of Pod '%s' belonging to Terraform job '%s':\n%s", podName, t.jobName, podLogs)
	}

	// Delete the Terraform Job and all its belonging Pods
	t.logger.Infof("Cleaning up pods created by Terraform job '%s'...", t.jobName)
	if err := t.cleanupJob(ctx, jobPodList); err != nil {
		return err
	}

	// Evaluate whether the execution was successful or not
	t.logger.Infof("Terraformer execution for job '%s' has been completed.", t.jobName)
	if !succeeded {
		errorMessage := fmt.Sprintf("Terraform execution job '%s' could not be completed.", t.jobName)
		if terraformErrors := retrieveTerraformErrors(logList); terraformErrors != nil {
			errorMessage += fmt.Sprintf(" The following issues have been found in the logs:\n\n%s", strings.Join(terraformErrors, "\n\n"))
		}
		return common.DetermineError(errorMessage)
	}
	return nil
}

const (
	serviceAccountName = "terraformer"
	roleName           = "garden.sapcloud.io:system:terraformers"
	roleBindingName    = roleName
)

func (t *Terraformer) createOrUpdateServiceAccount(ctx context.Context) error {
	serviceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: serviceAccountName}}
	return kutil.CreateOrUpdate(ctx, t.client, serviceAccount, func() error {
		return nil
	})
}

func (t *Terraformer) createOrUpdateRoleBinding(ctx context.Context) error {
	roleBinding := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: roleBindingName}}
	return kutil.CreateOrUpdate(ctx, t.client, roleBinding, func() error {
		roleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     roleBindingName,
		}
		roleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: t.namespace,
			},
		}
		return nil
	})
}

func (t *Terraformer) createOrUpdateTerraformerAuth(ctx context.Context) error {
	if err := t.createOrUpdateServiceAccount(ctx); err != nil {
		return err
	}
	return t.createOrUpdateRoleBinding(ctx)
}

func (t *Terraformer) deployTerraformerPod(ctx context.Context, scriptName string) error {
	if err := t.createOrUpdateTerraformerAuth(ctx); err != nil {
		return err
	}

	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.podName}}
	return kutil.CreateOrUpdate(ctx, t.client, pod, func() error {
		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}
		pod.Labels[jobNameLabel] = t.jobName
		pod.Spec = *t.podSpec(scriptName)
		return nil
	})
}

func (t *Terraformer) deployTerraformerJob(ctx context.Context, scriptName string) error {
	if err := t.createOrUpdateTerraformerAuth(ctx); err != nil {
		return err
	}

	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.jobName}}
	podSpec := t.podSpec(scriptName)

	return kutil.CreateOrUpdate(ctx, t.client, job, func() error {
		var (
			activeDeadlineSeconds = int64(3600)
			backoffLimit          = int32(3)
			spec                  = &job.Spec
		)

		spec.ActiveDeadlineSeconds = &activeDeadlineSeconds
		spec.BackoffLimit = &backoffLimit
		spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: t.namespace,
				Name:      t.jobName,
			},
			Spec: *podSpec,
		}

		return nil
	})
}

func (t *Terraformer) env() []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{Name: "MAX_BACKOFF_SEC", Value: "60"},
		{Name: "MAX_TIME_SEC", Value: "1800"},
		{Name: "TF_STATE_CONFIG_MAP_NAME", Value: t.stateName},
	}
	for k, v := range t.variablesEnvironment {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}
	return envVars
}

func (t *Terraformer) podSpec(scriptName string) *corev1.PodSpec {
	const (
		tfVolume      = "tf"
		tfVarsVolume  = "tfvars"
		tfStateVolume = "tfstate"

		tfVolumeMountPath      = tfVolume
		tfVarsVolumeMountPath  = tfVarsVolume
		tfStateVolumeMountPath = "tf-state-in"
	)

	activeDeadlineSeconds := int64(1800)
	shCommand := fmt.Sprintf("sh /terraform.sh %s", scriptName)
	if scriptName != "validate" {
		shCommand += " 2>&1; [[ -f /success ]] && exit 0 || exit 1"
	}

	return &corev1.PodSpec{
		RestartPolicy:         corev1.RestartPolicyNever,
		ActiveDeadlineSeconds: &activeDeadlineSeconds,
		Containers: []corev1.Container{
			{
				Name:            "terraform",
				Image:           t.image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{
					"sh",
					"-c",
					shCommand,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
				Env: t.env(),
				VolumeMounts: []corev1.VolumeMount{
					{Name: tfVolume, MountPath: fmt.Sprintf("/%s", tfVolumeMountPath)},
					{Name: tfVarsVolume, MountPath: fmt.Sprintf("/%s", tfVarsVolumeMountPath)},
					{Name: tfStateVolume, MountPath: fmt.Sprintf("/%s", tfStateVolumeMountPath)},
				},
			},
		},
		ServiceAccountName: serviceAccountName,
		Volumes: []corev1.Volume{
			{
				Name: tfVolume,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: t.configName},
					},
				},
			},
			{
				Name: tfVarsVolume,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: t.variablesName,
					},
				},
			},
			{
				Name: tfStateVolume,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: t.stateName},
					},
				},
			},
		},
	}
}

// listJobPods lists all pods which have a label 'job-name' whose value is equal to the Terraformer job name.
func (t *Terraformer) listJobPods(ctx context.Context) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	if err := t.client.List(ctx, &client.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{jobNameLabel: t.jobName})}, podList); err != nil {
		return nil, err
	}
	return podList, nil
}

// retrievePodLogs fetches the logs of the created Pods by the Terraform Job and returns them as a map whose
// keys are pod names and whose values are the corresponding logs.
func (t *Terraformer) retrievePodLogs(jobPodList *corev1.PodList) (map[string]string, error) {
	logChan := make(chan map[string]string, 1)
	go func() {
		var logList = map[string]string{}
		for _, jobPod := range jobPodList.Items {
			name := jobPod.Name
			logs, err := kubernetes.GetPodLogs(t.coreV1Client.Pods(jobPod.Namespace), name, &corev1.PodLogOptions{})
			if err != nil {
				t.logger.Warnf("Could not retrieve the logs of Terraform job pod %s: '%v'", name, err)
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
		return nil, fmt.Errorf("Timeout when reading the logs of all pds created by Terraform job '%s'", t.jobName)
	}
}

// cleanupJob deletes the Terraform Job and all belonging Pods from the Garden cluster.
func (t *Terraformer) cleanupJob(ctx context.Context, jobPodList *corev1.PodList) error {
	// Delete the Terraform Job
	if err := t.client.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: t.jobName}}); err == nil {
		t.logger.Infof("Deleted Terraform Job '%s'", t.jobName)
	} else {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	// Delete the belonging Terraform Pods
	for _, jobPod := range jobPodList.Items {
		if err := t.client.Delete(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: jobPod.Namespace, Name: jobPod.Name}}); err == nil {
			t.logger.Infof("Deleted Terraform Job Pod '%s'", jobPod.Name)
		} else {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}
