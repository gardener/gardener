// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionsbastion "github.com/gardener/gardener/extensions/pkg/bastion"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/bastion"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/helper"
	"github.com/gardener/gardener/pkg/provider-local/local"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// SSHPort is the default SSH port.
const SSHPort = 22

type actuator struct {
	client client.Client
}

func newActuator(mgr manager.Manager) bastion.Actuator {
	return &actuator{
		client: mgr.GetClient(),
	}
}

func (a *actuator) Reconcile(ctx context.Context, _ logr.Logger, bastion *extensionsv1alpha1.Bastion, cluster *extensionscontroller.Cluster) error {
	image, err := bastionImage(cluster)
	if err != nil {
		return err
	}

	var (
		userDataSecret = userDataSecretForBastion(bastion)
		pod            = podForBastion(bastion, image, userDataSecret.Name)
		service        = serviceForBastion(bastion)
	)

	for _, obj := range []client.Object{userDataSecret, pod, service} {
		if err := controllerutil.SetControllerReference(bastion, obj, a.client.Scheme()); err != nil {
			return fmt.Errorf("failed to set controller reference on %T %q: %w", obj, client.ObjectKeyFromObject(obj), err)
		}
		if err := a.client.Patch(ctx, obj, client.Apply, local.FieldOwner, client.ForceOwnership); err != nil {
			return fmt.Errorf("failed to apply %T %q: %w", obj, client.ObjectKeyFromObject(obj), err)
		}
	}

	// wait for LoadBalancer to be ready
	if err := health.CheckService(service); err != nil {
		return &reconcilerutils.RequeueAfterError{
			RequeueAfter: 5 * time.Second,
			Cause:        fmt.Errorf("waiting for Bastion LoadBalancer to get ready: %w", err),
		}
	}

	// wait for Pod to be ready
	if !health.IsPodReady(pod) {
		return &reconcilerutils.RequeueAfterError{
			RequeueAfter: 5 * time.Second,
			Cause:        fmt.Errorf("waiting for Bastion Pod to get ready"),
		}
	}

	patch := client.MergeFrom(bastion.DeepCopy())
	bastion.Status.Ingress = service.Status.LoadBalancer.Ingress[0].DeepCopy()
	return a.client.Status().Patch(ctx, bastion, patch)
}

func (a *actuator) Delete(ctx context.Context, log logr.Logger, bastion *extensionsv1alpha1.Bastion, _ *extensionscontroller.Cluster) error {
	// Explicitly delete the Bastion Pod so that we can wait for it to terminate. The other objects will get cleaned up by
	// the garbage collector (due to ownerReferences), but only after removing the Bastion's finalizer.
	if err := a.client.Delete(ctx, podForBastion(bastion, "", "")); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Bastion Pod gone, releasing Bastion")
			return nil
		}
		return err
	}

	return &reconcilerutils.RequeueAfterError{
		RequeueAfter: 10 * time.Second,
		Cause:        fmt.Errorf("waiting for Bastion Pod to terminate"),
	}
}

func (a *actuator) ForceDelete(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.Bastion, _ *extensionscontroller.Cluster) error {
	return nil
}

func bastionImage(cluster *extensionscontroller.Cluster) (string, error) {
	machineSpec, err := extensionsbastion.GetMachineSpecFromCloudProfile(cluster.CloudProfile)
	if err != nil {
		return "", fmt.Errorf("failed to determine machine spec for bastion from CloudProfile: %w", err)
	}

	cloudProfileConfig, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return "", fmt.Errorf("failed to extract CloudProfileConfig from cluster: %w", err)
	}

	machineTypeFromCloudProfile := v1beta1helper.FindMachineTypeByName(cluster.CloudProfile.Spec.MachineTypes, machineSpec.MachineTypeName)

	image, err := helper.FindImageFromCloudProfile(cloudProfileConfig, machineSpec.ImageBaseName, machineSpec.ImageVersion, machineTypeFromCloudProfile.Capabilities, cluster.CloudProfile.Spec.Capabilities)
	if err != nil {
		return "", fmt.Errorf("failed to find machine image in CloudProfileConfig: %w", err)
	}

	return image.Image, nil
}

func objectMetaForBastion(bastion *extensionsv1alpha1.Bastion) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      "bastion-" + bastion.Name,
		Namespace: bastion.Namespace,
		Labels: map[string]string{
			"app":     "bastion",
			"bastion": bastion.Name,
		},
	}
}

func userDataSecretForBastion(bastion *extensionsv1alpha1.Bastion) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: objectMetaForBastion(bastion),
		Data: map[string][]byte{
			"userdata": bastion.Spec.UserData,
		},
	}
}

func podForBastion(bastion *extensionsv1alpha1.Bastion, image, userDataSecretName string) *corev1.Pod {
	objectMeta := objectMetaForBastion(bastion)
	metav1.SetMetaDataLabel(&objectMeta, gardenerutils.NetworkPolicyLabel("machines", SSHPort), v1beta1constants.LabelNetworkPolicyAllowed)
	metav1.SetMetaDataLabel(&objectMeta, v1beta1constants.LabelNetworkPolicyToDNS, v1beta1constants.LabelNetworkPolicyAllowed)

	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Pod",
		},
		ObjectMeta: objectMeta,
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "machine",
					Image:           image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					SecurityContext: &corev1.SecurityContext{
						Privileged: pointer.Bool(true),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "userdata",
							MountPath: "/etc/machine",
						},
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromString("ssh"),
							},
						},
					},
					Ports: []corev1.ContainerPort{{
						ContainerPort: SSHPort,
						Name:          "ssh",
						Protocol:      corev1.ProtocolTCP,
					}},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "userdata",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  userDataSecretName,
							DefaultMode: pointer.Int32(0777),
						},
					},
				},
			},
		},
	}
}

func serviceForBastion(bastion *extensionsv1alpha1.Bastion) *corev1.Service {
	objectMeta := objectMetaForBastion(bastion)
	metav1.SetMetaDataAnnotation(&objectMeta, resourcesv1alpha1.NetworkingFromWorldToPorts, fmt.Sprintf(`[{"protocol":"TCP","port":%d}]`, SSHPort))

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: objectMeta,
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeLoadBalancer,
			Selector: objectMeta.DeepCopy().Labels,
			Ports: []corev1.ServicePort{{
				Name:        "ssh",
				Port:        SSHPort,
				Protocol:    corev1.ProtocolTCP,
				AppProtocol: ptr.To("ssh"),
			}},
		},
	}
}
