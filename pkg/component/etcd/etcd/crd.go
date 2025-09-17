// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"fmt"
	"maps"
	"slices"

	"github.com/Masterminds/semver/v3"
	druidcorecrds "github.com/gardener/etcd-druid/api/core/v1alpha1/crds"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/crddeployer"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// NewCRD can be used to deploy the CRD definitions for all CRDs defined by etcd-druid.
func NewCRD(client client.Client, k8sVersion *semver.Version) (component.DeployWaiter, error) {
	crdGetter, err := NewCRDGetter(k8sVersion)
	if err != nil {
		return nil, err
	}

	return crddeployer.New(client, crdGetter.GetAllCRDsAsStringSlice(), true)
}

// CRDGetter provides methods to get CRDs defined in etcd-druid.
type CRDGetter interface {
	// GetAllCRDs returns a map of CRD names to CRD objects.
	GetAllCRDs() (map[string]*apiextensionsv1.CustomResourceDefinition, error)
	// GetCRD returns the CRD with the given name.
	// An error is returned if no CRD is found with the given name.
	GetCRD(name string) (*apiextensionsv1.CustomResourceDefinition, error)
	// GetAllCRDsAsStringSlice returns all CRDs as Strings.
	GetAllCRDsAsStringSlice() []string
}

type crdGetterImpl struct {
	crdResources map[string]string
	k8sVersion   *semver.Version
}

func (c *crdGetterImpl) GetAllCRDsAsStringSlice() []string {
	return slices.Collect(maps.Values(c.crdResources))
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

func (c *crdGetterImpl) GetAllCRDs() (map[string]*apiextensionsv1.CustomResourceDefinition, error) {
	var crdResources = make(map[string]*apiextensionsv1.CustomResourceDefinition)
	for crdName, crdYAML := range c.crdResources {
		crdObj, err := kubernetesutils.DecodeCRD(crdYAML)
		if err != nil {
			return nil, fmt.Errorf("failed decode etcd-druid CRD: %s: %w", crdName, err)
		}
		crdResources[crdName] = crdObj
	}
	return crdResources, nil
}

func (c *crdGetterImpl) GetCRD(name string) (*apiextensionsv1.CustomResourceDefinition, error) {
	crdResources, err := c.GetAllCRDs()
	if err != nil {
		return nil, err
	}

	crdObj, ok := crdResources[name]
	if !ok {
		return nil, fmt.Errorf("required CRD not found in map: %s", name)
	}

	return crdObj, nil
}

func getEtcdCRDs(k8sVersion *semver.Version) (map[string]string, error) {
	crdYAMLs, err := druidcorecrds.GetAll(k8sVersion.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get etcd-druid CRDs for Kubernetes version %s: %w", k8sVersion, err)
	}

	return crdYAMLs, nil
}
