// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	_ "embed"
	"fmt"

	"github.com/Masterminds/semver/v3"
	druidcorecrds "github.com/gardener/etcd-druid/api/core/v1alpha1/crds"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/crddeployer"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// NewCRD can be used to deploy the CRD definitions for all CRDs defined by etcd-druid.
func NewCRD(client client.Client, applier kubernetes.Applier, k8sVersion *semver.Version) (component.DeployWaiter, error) {
	crdGetter, err := NewCRDGetter(k8sVersion)
	if err != nil {
		return nil, err
	}
	crds, err := crdGetter.GetAllCRDsAsStringSlice()
	if err != nil {
		return nil, err
	}
	return crddeployer.New(client, applier, crds, true)
}

// CRDGetter provides methods to get CRDs defined in etcd-druid.
type CRDGetter interface {
	// GetAllCRDs returns a map of CRD names to CRD objects.
	GetAllCRDs() map[string]*apiextensionsv1.CustomResourceDefinition
	// GetCRD returns the CRD with the given name.
	// An error is returned if no CRD is found with the given name.
	GetCRD(name string) (*apiextensionsv1.CustomResourceDefinition, error)
	// GetAllCRDsAsStringSlice returns all CRDs as Strings.
	GetAllCRDsAsStringSlice() ([]string, error)
}

type crdGetterImpl struct {
	crdResources map[string]*apiextensionsv1.CustomResourceDefinition
	k8sVersion   *semver.Version
}

func (c *crdGetterImpl) GetAllCRDsAsStringSlice() ([]string, error) {
	crds := c.GetAllCRDs()
	crdStrings := make([]string, 0, len(crds))
	for _, crd := range crds {
		crdString, err := kubernetesutils.Serialize(crd, kubernetesscheme.Scheme)
		if err != nil {
			return nil, err
		}
		crdStrings = append(crdStrings, crdString)
	}
	return crdStrings, nil
}

var _ CRDGetter = (*crdGetterImpl)(nil)

// NewCRDGetter creates a new CRDGetter.
func NewCRDGetter(k8sVersion *semver.Version) (CRDGetter, error) {
	crdResources, err := getEtcdCRDs(k8sVersion)
	if err != nil {
		return nil, err
	}
	return &crdGetterImpl{
		crdResources: crdResources,
		k8sVersion:   k8sVersion,
	}, nil
}

func (c *crdGetterImpl) GetAllCRDs() map[string]*apiextensionsv1.CustomResourceDefinition {
	return c.crdResources
}

func (c *crdGetterImpl) GetCRD(name string) (*apiextensionsv1.CustomResourceDefinition, error) {
	crdObj, ok := c.crdResources[name]
	if !ok {
		return nil, fmt.Errorf("CRD %s not found", name)
	}
	return crdObj, nil
}

func getEtcdCRDs(k8sVersion *semver.Version) (map[string]*apiextensionsv1.CustomResourceDefinition, error) {
	crdYAMLs, err := druidcorecrds.GetAll(k8sVersion.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get etcd-druid CRDs for Kubernetes version %s: %w", k8sVersion, err)
	}
	var crdResources = make(map[string]*apiextensionsv1.CustomResourceDefinition, len(crdYAMLs))
	for crdName, crdYAML := range crdYAMLs {
		crdObj, err := kubernetesutils.DecodeCRD(crdYAML)
		if err != nil {
			return nil, fmt.Errorf("failed decode etcd-druid CRD: %s: %w", crdName, err)
		}
		metav1.SetMetaDataLabel(&crdObj.ObjectMeta, gardenerutils.DeletionProtected, "true")
		crdResources[crdName] = crdObj
	}
	return crdResources, nil
}
