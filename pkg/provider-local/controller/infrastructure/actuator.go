// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/provider-local/local"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type actuator struct {
	// runtimeClient uses provider-local's in-cluster config, e.g., for the seed/bootstrap cluster it runs in.
	// It's used to interact with extension objects. By default, it's also used as the provider client to interact with
	// infrastructure resources, unless a kubeconfig is specified in the cloudprovider secret.
	runtimeClient client.Client
}

// NewActuator creates a new Actuator that updates the status of the handled Infrastructure resources.
func NewActuator(mgr manager.Manager) infrastructure.Actuator {
	return &actuator{
		runtimeClient: mgr.GetClient(),
	}
}

func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	providerClient, err := local.GetProviderClient(ctx, log, a.runtimeClient, infrastructure.Spec.SecretRef)
	if err != nil {
		return fmt.Errorf("could not create client for infrastructure resources: %w", err)
	}

	// Apply the machine namespace first, so we can use its UUID as owner reference for the IPPools.
	machineNamespace := namespace(cluster.Shoot.Status.TechnicalID)
	if err := providerClient.Patch(ctx, machineNamespace, client.Apply, local.FieldOwner, client.ForceOwnership); err != nil {
		return err
	}

	ipPools := []client.Object{}
	for _, ipFamily := range cluster.Shoot.Spec.Networking.IPFamilies {
		ipPoolObj, err := ipPool(machineNamespace, cluster.Shoot.Status.TechnicalID, string(ipFamily), *cluster.Shoot.Spec.Networking.Nodes)
		if err != nil {
			return err
		}
		ipPools = append(ipPools, ipPoolObj)
	}

	for _, obj := range ipPools {
		if err := providerClient.Patch(ctx, obj, client.Apply, local.FieldOwner, client.ForceOwnership); err != nil {
			return err
		}
	}

	patch := client.MergeFrom(infrastructure.DeepCopy())
	infrastructure.Status.Networking = &extensionsv1alpha1.InfrastructureStatusNetworking{}
	if nodes := cluster.Shoot.Spec.Networking.Nodes; nodes != nil {
		infrastructure.Status.Networking.Nodes = []string{*nodes}
		// The egress CIDRs of local nodes are hard to define and depends on the traffic destination.
		// Traffic to the seed cluster will have nodes IPs as source IPs (i.e., the machine pod IPs).
		// Traffic to other containers in the kind network or the outside world will be NATed.
		// For now, we only report the nodes CIDR here to test the feature that propagates it back to the shoot status.
		infrastructure.Status.EgressCIDRs = []string{*nodes}
	}
	if pods := cluster.Shoot.Spec.Networking.Pods; pods != nil {
		infrastructure.Status.Networking.Pods = []string{*pods}
	}
	if services := cluster.Shoot.Spec.Networking.Services; services != nil {
		infrastructure.Status.Networking.Services = []string{*services}
	}

	return a.runtimeClient.Status().Patch(ctx, infrastructure, patch)
}

func (a *actuator) Delete(ctx context.Context, log logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	providerClient, err := local.GetProviderClient(ctx, log, a.runtimeClient, infrastructure.Spec.SecretRef)
	if err != nil {
		return fmt.Errorf("could not create client for infrastructure resources: %w", err)
	}

	return kubernetesutils.DeleteObjects(ctx, providerClient,
		namespace(cluster.Shoot.Status.TechnicalID),
		&metav1.PartialObjectMetadata{TypeMeta: metav1.TypeMeta{APIVersion: "crd.projectcalico.org/v1", Kind: "IPPool"}, ObjectMeta: metav1.ObjectMeta{Name: IPPoolName(cluster.Shoot.Status.TechnicalID, string(gardencorev1beta1.IPFamilyIPv4))}},
		&metav1.PartialObjectMetadata{TypeMeta: metav1.TypeMeta{APIVersion: "crd.projectcalico.org/v1", Kind: "IPPool"}, ObjectMeta: metav1.ObjectMeta{Name: IPPoolName(cluster.Shoot.Status.TechnicalID, string(gardencorev1beta1.IPFamilyIPv6))}},
	)
}

func (a *actuator) Migrate(context.Context, logr.Logger, *extensionsv1alpha1.Infrastructure, *extensionscontroller.Cluster) error {
	return nil
}

func (a *actuator) ForceDelete(ctx context.Context, log logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	return a.Delete(ctx, log, infrastructure, cluster)
}

func (a *actuator) Restore(ctx context.Context, log logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	return a.Reconcile(ctx, log, infrastructure, cluster)
}

func namespace(technicalID string) *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: MachineNamespaceName(technicalID),
			Labels: map[string]string{
				v1beta1constants.GardenRole: "machine-pods",
			},
		},
	}
}

// MachineNamespaceName returns the name of the namespace for machine pods for the given shoot namespace.
func MachineNamespaceName(technicalID string) string {
	return "machine-pods-" + technicalID
}

// IPPoolName returns the name of the crd.projectcalico.org/v1.IPPool resource for the given shoot namespace.
func IPPoolName(technicalID, ipFamily string) string {
	return "shoot-machine-pods-" + technicalID + "-" + strings.ToLower(ipFamily)
}

func ipPool(machineNamespace *corev1.Namespace, technicalID string, ipFamily, nodeCIDR string) (client.Object, error) {
	ipipMode := "Always"
	if ipFamily == string(gardencorev1beta1.IPFamilyIPv6) {
		ipipMode = "Never"
	}
	obj, err := kubernetes.NewManifestReader([]byte(`apiVersion: crd.projectcalico.org/v1
kind: IPPool
metadata:
  name: ` + IPPoolName(technicalID, ipFamily) + `
spec:
  cidr: ` + nodeCIDR + `
  ipipMode: ` + ipipMode + `
  natOutgoing: true
  nodeSelector: "!all()" # Without this, calico defaults nodeSelector to "all()" and can randomly pick this pool for
                         # IPAM for pods even if the pod does not explicitly request an IP from this pool via the
                         # cni.projectcalico.org/IPv{4,6}Pools annotation.
                         # See https://github.com/projectcalico/calico/issues/7299#issuecomment-1446834103
`)).Read()
	if err != nil {
		return nil, err
	}
	obj.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: machineNamespace.APIVersion,
		Kind:       machineNamespace.Kind,
		Name:       machineNamespace.Name,
		UID:        machineNamespace.UID,
	}})
	return obj, nil
}
