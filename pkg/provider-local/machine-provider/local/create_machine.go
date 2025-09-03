// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apiv1alpha1 "github.com/gardener/gardener/pkg/provider-local/machine-provider/api/v1alpha1"
	"github.com/gardener/gardener/pkg/provider-local/machine-provider/api/validation"
)

// MachinePodContainerName is a constant for the name of the container in the machine pod.
const MachinePodContainerName = "node"

func (d *localDriver) CreateMachine(ctx context.Context, req *driver.CreateMachineRequest) (*driver.CreateMachineResponse, error) {
	if req.MachineClass.Provider != apiv1alpha1.Provider {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("requested for provider '%s', we only support '%s'", req.MachineClass.Provider, apiv1alpha1.Provider))
	}

	klog.V(3).Infof("Machine creation request has been received for %q", req.Machine.Name)
	defer klog.V(3).Infof("Machine creation request has been processed for %q", req.Machine.Name)

	providerSpec, err := validateProviderSpecAndSecret(req.MachineClass, req.Secret)
	if err != nil {
		return nil, err
	}

	userDataSecret := userDataSecretForMachine(req.Machine, req.MachineClass)
	userDataSecret.Data = map[string][]byte{"userdata": req.Secret.Data["userData"]}

	if err := controllerutil.SetControllerReference(req.Machine, userDataSecret, d.client.Scheme()); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("could not set userData secret ownership: %s", err.Error()))
	}

	if err := d.client.Patch(ctx, userDataSecret, client.Apply, fieldOwner, client.ForceOwnership); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("error applying user data secret: %s", err.Error()))
	}

	if err := d.applyService(ctx, req); err != nil {
		return nil, err
	}

	pod, err := d.applyPod(ctx, req, providerSpec, userDataSecret)
	if err != nil {
		return nil, err
	}

	if err := d.waitForPodToBeRunning(ctx, pod); err != nil {
		return nil, err
	}

	return &driver.CreateMachineResponse{
		ProviderID: pod.Name,
		NodeName:   pod.Name,
		Addresses:  addressesFromStatus(pod.Status),
	}, nil
}

func (d *localDriver) applyService(ctx context.Context, req *driver.CreateMachineRequest) error {
	service := serviceForMachine(req.Machine, req.MachineClass)

	service.Labels = map[string]string{
		labelKeyProvider: apiv1alpha1.Provider,
		labelKeyApp:      labelValueMachine,
		labelKeyMachine:  req.Machine.Name,
	}
	service.Spec = corev1.ServiceSpec{
		Type:      corev1.ServiceTypeClusterIP,
		ClusterIP: corev1.ClusterIPNone,
		Selector:  maps.Clone(service.Labels),
		// Publish the machine pod IP, even if the pod is not ready, because this happens only eventually when the Node
		// joins the cluster (or never in case of `gardenadm bootstrap`).
		PublishNotReadyAddresses: true,
		Ports: []corev1.ServicePort{
			{
				Name:        "ssh",
				Port:        22,
				Protocol:    corev1.ProtocolTCP,
				AppProtocol: ptr.To("ssh"),
			},
		},
	}

	if err := controllerutil.SetControllerReference(req.Machine, service, d.client.Scheme()); err != nil {
		return status.Error(codes.Internal, fmt.Sprintf("could not set service ownership: %s", err.Error()))
	}

	if err := d.client.Patch(ctx, service, client.Apply, fieldOwner, client.ForceOwnership); err != nil {
		return status.Error(codes.Internal, fmt.Sprintf("error applying service: %s", err.Error()))
	}

	return nil
}

func (d *localDriver) applyPod(
	ctx context.Context,
	req *driver.CreateMachineRequest,
	providerSpec *apiv1alpha1.ProviderSpec,
	userDataSecret *corev1.Secret,
) (
	*corev1.Pod,
	error,
) {
	pod := podForMachine(req.Machine, req.MachineClass)
	pod.Annotations = map[string]string{}

	if providerSpec.IPPoolNameV4 != "" {
		pod.Annotations["cni.projectcalico.org/ipv4pools"] = `["` + providerSpec.IPPoolNameV4 + `"]`
	}
	if providerSpec.IPPoolNameV6 != "" {
		pod.Annotations["cni.projectcalico.org/ipv6pools"] = `["` + providerSpec.IPPoolNameV6 + `"]`
	}

	pod.Labels = map[string]string{
		labelKeyProvider:                   apiv1alpha1.Provider,
		labelKeyApp:                        labelValueMachine,
		labelKeyMachine:                    req.Machine.Name,
		"networking.gardener.cloud/to-dns": "allowed",
		"networking.gardener.cloud/to-private-networks":                 "allowed",
		"networking.gardener.cloud/to-public-networks":                  "allowed",
		"networking.gardener.cloud/to-runtime-apiserver":                "allowed", // needed for ManagedSeeds such that gardenlets deployed to these Machines can talk to the seed's kube-apiserver (which is the same like the garden cluster kube-apiserver)
		"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
	}
	pod.Spec = corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            MachinePodContainerName,
				Image:           providerSpec.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				SecurityContext: &corev1.SecurityContext{
					Privileged: pointer.Bool(true),
				},
				Env: []corev1.EnvVar{{
					Name: "NODE_NAME",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				}},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "userdata",
						MountPath: "/etc/machine",
					},
					{
						Name:      "containerd",
						MountPath: "/var/lib/containerd",
					},
					{
						Name:      "modules",
						MountPath: "/lib/modules",
						ReadOnly:  true,
					},
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						Exec: &corev1.ExecAction{
							Command: []string{"sh", "-c", "/usr/bin/kubectl --kubeconfig /var/lib/kubelet/kubeconfig-real get no $NODE_NAME"},
						},
					},
				},
				Ports: []corev1.ContainerPort{{
					ContainerPort: 30123,
					Name:          "vpn-shoot",
					Protocol:      corev1.ProtocolTCP,
				}},
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "userdata",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  userDataSecret.Name,
						DefaultMode: pointer.Int32(0777),
					},
				},
			},
			{
				Name: "containerd",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			{
				Name: "modules",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/lib/modules",
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(req.Machine, pod, d.client.Scheme()); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("could not set pod ownership: %s", err.Error()))
	}

	if err := d.client.Patch(ctx, pod, client.Apply, fieldOwner, client.ForceOwnership); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("error applying pod: %s", err.Error()))
	}

	return pod, nil
}

func (d *localDriver) waitForPodToBeRunning(ctx context.Context, pod *corev1.Pod) error {
	// Actively wait until pod is running. Without doing so, we might not be able to catch misconfigurations or image
	// problems before the Machine transitions to Available/Pending.
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := wait.PollUntilContextCancel(timeoutCtx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		if err := d.client.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
			return true, err
		}

		if pod.Status.Phase == corev1.PodRunning {
			return true, nil
		}

		return false, nil
	}); err != nil {
		// will be retried with short retry by machine controller
		return status.Error(codes.DeadlineExceeded, fmt.Sprintf("pod %q is in phase %q, failed waiting for phase %q: %v", pod.Name, pod.Status.Phase, corev1.PodRunning, err))
	}

	return nil
}

func validateProviderSpecAndSecret(machineClass *machinev1alpha1.MachineClass, secret *corev1.Secret) (*apiv1alpha1.ProviderSpec, error) {
	if machineClass == nil {
		return nil, status.Error(codes.Internal, "MachineClass ProviderSpec is nil")
	}

	var providerSpec *apiv1alpha1.ProviderSpec
	if err := json.Unmarshal(machineClass.ProviderSpec.Raw, &providerSpec); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	validationErr := validation.ValidateProviderSpec(providerSpec, secret, field.NewPath("providerSpec"))
	if validationErr.ToAggregate() != nil && len(validationErr.ToAggregate().Errors()) > 0 {
		err := fmt.Errorf("error while validating ProviderSpec: %v", validationErr.ToAggregate().Error())
		klog.V(2).Infof("Validation of AWSMachineClass failed %s", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return providerSpec, nil
}
