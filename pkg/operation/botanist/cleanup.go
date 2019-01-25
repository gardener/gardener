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

package botanist

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/hashicorp/go-multierror"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	exceptions = map[string]map[string]bool{
		kubernetes.CustomResourceDefinitions: map[string]bool{
			"felixconfigurations.crd.projectcalico.org":   true,
			"bgppeers.crd.projectcalico.org":              true,
			"bgpconfigurations.crd.projectcalico.org":     true,
			"ippools.crd.projectcalico.org":               true,
			"clusterinformations.crd.projectcalico.org":   true,
			"globalnetworkpolicies.crd.projectcalico.org": true,
			"globalnetworksets.crd.projectcalico.org":     true,
			"networkpolicies.crd.projectcalico.org":       true,
			"hostendpoints.crd.projectcalico.org":         true,
		},
		kubernetes.DaemonSets: {
			fmt.Sprintf("%s/calico-node", metav1.NamespaceSystem):              true,
			fmt.Sprintf("%s/kube-proxy", metav1.NamespaceSystem):               true,
			fmt.Sprintf("%s/csi-disk-plugin-alicloud", metav1.NamespaceSystem): true,
		},
		kubernetes.Deployments: {
			fmt.Sprintf("%s/coredns", metav1.NamespaceSystem):        true,
			fmt.Sprintf("%s/metrics-server", metav1.NamespaceSystem): true,
			fmt.Sprintf("%s/csi-attacher", metav1.NamespaceSystem):   true,
		},
		kubernetes.StatefulSets: {
			fmt.Sprintf("%s/csi-provisioner", metav1.NamespaceSystem): true,
		},
		kubernetes.Namespaces: {
			metav1.NamespacePublic:  true,
			metav1.NamespaceSystem:  true,
			metav1.NamespaceDefault: true,
		},
		kubernetes.Services: {
			fmt.Sprintf("%s/kubernetes", metav1.NamespaceDefault): true,
		},
	}
)

func excludeAddonManagerManagedListOptions() metav1.ListOptions {
	selector := labels.NewSelector()
	req, err := labels.NewRequirement("addonmanager.kubernetes.io/mode", selection.DoesNotExist, nil)
	runtime.Must(err)
	selector.Add(*req)
	return metav1.ListOptions{
		LabelSelector: selector.String(),
	}
}

// CleanWebhooks deletes all Webhooks in the Shoot cluster that are not being managed by the addon manager.
func (b *Botanist) CleanWebhooks(ctx context.Context) error {
	var result error
	admissionRegistration := b.K8sShootClient.Kubernetes().AdmissionregistrationV1beta1()

	if err := admissionRegistration.ValidatingWebhookConfigurations().DeleteCollection(nil, excludeAddonManagerManagedListOptions()); err != nil {
		result = multierror.Append(result, err)
	}

	if err := admissionRegistration.MutatingWebhookConfigurations().DeleteCollection(nil, excludeAddonManagerManagedListOptions()); err != nil {
		result = multierror.Append(result, err)
	}

	return result
}

// CleanKubernetesResources deletes all the Kubernetes resources in the Shoot cluster
// other than those stored in the exceptions map. It will check whether all the Kubernetes resources
// in the Shoot cluster other than those stored in the exceptions map have been deleted.
// It will return an error in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CleanKubernetesResources() error {
	var (
		wg     sync.WaitGroup
		errors []error
	)

	// TODO: The following code need to be be refactored if the following bug for CSI is fixed:
	// Because of the issue of https://github.com/kubernetes-csi/external-provisioner/issues/195 we can't allow
	// direct deletion of PVs because otherwise the CSI plugin does not get notified and won't delete the PV.
	// However, we want to wait until all PVs have been deleted. Attention: This won't delete PVs with a
	// reclaimPolicy != Delete. Those PVs must be deleted manually!
	var (
		originalResourceAPIGroups = b.K8sShootClient.GetResourceAPIGroups()
		modifiedResourceAPIGroups = make(map[string][]string, len(originalResourceAPIGroups))
	)
	for resource, apiGroupPath := range originalResourceAPIGroups {
		modifiedResourceAPIGroups[resource] = apiGroupPath
	}
	if b.Shoot.UsesCSI() {
		delete(modifiedResourceAPIGroups, kubernetes.PersistentVolumes)
	}

	if err := b.K8sShootClient.CleanupResources(exceptions, modifiedResourceAPIGroups); err != nil {
		return err
	}

	for resource, apiGroupPath := range originalResourceAPIGroups {
		wg.Add(1)
		go func(apiGroupPath []string, resource string) {
			defer wg.Done()
			if err := b.waitForAPIGroupCleanedUp(apiGroupPath, resource); err != nil {
				errors = append(errors, err)
			}
		}(apiGroupPath, resource)
	}
	wg.Wait()

	if len(errors) == 0 {
		return nil
	}
	return fmt.Errorf("Error(s) while waiting for Kubernetes resource cleanup: %+v", errors)
}

// CleanCustomResourceDefinitions deletes all the CRDs in the Kubernetes cluster (which
// will delete the existing custom resources, recursively). It will wait until all resources
// have been cleaned up.
func (b *Botanist) CleanCustomResourceDefinitions() error {
	var (
		apiGroups       = b.K8sShootClient.GetResourceAPIGroups()
		resource        = kubernetes.CustomResourceDefinitions
		crdAPIGroupPath = apiGroups[resource]
	)

	if err := b.K8sShootClient.CleanupAPIGroupResources(exceptions, resource, crdAPIGroupPath); err != nil {
		return err
	}
	return b.waitForAPIGroupCleanedUp(crdAPIGroupPath, resource)
}

// ForceDeleteCustomResourceDefinitions forcefully deletes all custom CRDs, accumulating
// all errors in the process.
func (b *Botanist) ForceDeleteCustomResourceDefinitions() error {
	crdList, err := b.K8sShootClient.ListCRDs(metav1.ListOptions{})
	if err != nil {
		return err
	}

	var result error
	for _, crd := range crdList.Items {
		if omit, ok := exceptions[kubernetes.CustomResourceDefinitions][crd.Name]; !ok || !omit {
			if err := b.K8sShootClient.DeleteCRDForcefully(crd.Name); err != nil && !apierrors.IsNotFound(err) {
				result = multierror.Append(result, err)
			}
		}
	}
	return result
}

// CleanupCustomAPIServices deletes all the custom API services in the Kubernetes cluster.
// It will wait until all resources have been cleaned up.
func (b *Botanist) CleanupCustomAPIServices() error {
	apiServiceList, err := b.K8sShootClient.ListAPIServices(metav1.ListOptions{})
	if err != nil {
		return err
	}

	var result error
	for _, apiService := range apiServiceList.Items {
		if apiService.Spec.Service != nil {
			if err := b.K8sShootClient.DeleteAPIService(apiService.Name); err != nil && !apierrors.IsNotFound(err) {
				result = multierror.Append(result, err)
			}
		}
	}
	return result
}

// ForceDeleteCustomAPIServices forcefully deletes all custom API services,
// accumulating all errors in the process.
func (b *Botanist) ForceDeleteCustomAPIServices() error {
	apiServiceList, err := b.K8sShootClient.ListAPIServices(metav1.ListOptions{})
	if err != nil {
		return err
	}

	var result error
	for _, apiService := range apiServiceList.Items {
		if apiService.Spec.Service != nil {
			if err := b.K8sShootClient.DeleteAPIServiceForcefully(apiService.Name); err != nil && !apierrors.IsNotFound(err) {
				result = multierror.Append(result, err)
			}
		}
	}
	return result
}

func (b *Botanist) waitForAPIGroupCleanedUp(apiGroupPath []string, resource string) error {
	if err := wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		return b.K8sShootClient.CheckResourceCleanup(b.Logger, exceptions, resource, apiGroupPath)
	}); err != nil {
		return fmt.Errorf("Error while waiting for cleanup of '%s' resources: '%s'", resource, err.Error())
	}
	return nil
}
