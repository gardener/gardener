/*
 * Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 *
 */

package resources

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gardener/controller-manager-library/pkg/logger"
	"github.com/gardener/controller-manager-library/pkg/utils"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/restmapper"
)

type Info struct {
	groupVersion *schema.GroupVersion
	kind         string
	resourcename string
	namespaced   bool
	subresources utils.StringSet
}

func (this *Info) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: this.groupVersion.Group, Version: this.groupVersion.Version, Kind: this.kind}
}

func (this *Info) GroupKind() schema.GroupKind {
	return schema.GroupKind{Group: this.groupVersion.Group, Kind: this.kind}
}

func (this *Info) GroupVersion() schema.GroupVersion {
	return schema.GroupVersion{Group: this.groupVersion.Group, Version: this.groupVersion.Version}
}

func (this *Info) GroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: this.groupVersion.Group, Version: this.groupVersion.Version, Resource: this.resourcename}
}

func (this *Info) Group() string {
	return this.groupVersion.Group
}
func (this *Info) Version() string {
	return this.groupVersion.Version
}

func (this *Info) Kind() string {
	return this.kind
}
func (this *Info) Name() string {
	return this.resourcename
}
func (this *Info) Namespaced() bool {
	return this.namespaced
}
func (this *Info) SubResources() utils.StringSet {
	return utils.NewStringSetBySets(this.subresources)
}
func (this *Info) HasSubResource(name string) bool {
	return this.subresources.Contains(name)
}
func (this *Info) HasStatusSubResource() bool {
	return this.HasSubResource("status")
}

func (this *Info) String() string {
	return fmt.Sprintf("%s/%s %s %s %t", this.groupVersion.Group, this.groupVersion.Version, this.resourcename, this.kind, this.namespaced)
}

func (this *Info) InfoString() string {
	return fmt.Sprintf("%s %s %t", this.resourcename, this.kind, this.namespaced)
}

type ResourceInfos struct {
	lock              sync.RWMutex
	groupVersionKinds map[schema.GroupVersion]map[string]*Info
	preferredVersions map[string]string
	cluster           Cluster
	mapper            meta.RESTMapper
}

func NewResourceInfos(c Cluster) (*ResourceInfos, error) {
	res := &ResourceInfos{
		groupVersionKinds: map[schema.GroupVersion]map[string]*Info{},
		preferredVersions: map[string]string{},
		cluster:           c,
	}
	err := res.update()
	return res, err
}

func (this *ResourceInfos) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	m, err := this.mapper.RESTMapping(gk, versions...)
	if err != nil {
		err = this.updateRestMapper()
		if err != nil {
			return nil, err
		}
		m, err = this.mapper.RESTMapping(gk, versions...)
	}
	return m, err
}

func (this *ResourceInfos) updateRestMapper() error {
	cfg := this.cluster.Config()
	dc, err := discovery.NewDiscoveryClientForConfig(&cfg)
	if err != nil {
		return err
	}
	gr, err := restmapper.GetAPIGroupResources(dc)
	if err != nil {
		return err
	}
	this.mapper = restmapper.NewDiscoveryRESTMapper(gr)
	return nil
}

func (this *ResourceInfos) update() error {
	cfg := this.cluster.Config()
	dc, err := discovery.NewDiscoveryClientForConfig(&cfg)
	if err != nil {
		logger.Warnf("failed to get discovery client for cluster %s: %s", this.cluster.GetName(), err)
		return err
	}

	//list, err := discovery.ServerResources(dc)
	list, err := dc.ServerResources()
	if err != nil {
		logger.Warnf("failed to get all server resources for cluster %s: %s", this.cluster.GetName(), err)
		if len(list) == 0 {
			return err
		}
		logger.Infof("found %d resources", len(list))
	}
	this.lock.Lock()
	defer this.lock.Unlock()
	for _, rl := range list {
		gv, _ := schema.ParseGroupVersion(rl.GroupVersion)

		m := this.groupVersionKinds[gv]
		if m == nil {
			m = map[string]*Info{}
			this.groupVersionKinds[gv] = m
		}
		for _, r := range rl.APIResources {
			if strings.Index(r.Name, "/") < 0 {
				m[r.Kind] = &Info{groupVersion: &gv, resourcename: r.Name, kind: r.Kind, namespaced: r.Namespaced, subresources: utils.StringSet{}}
			}
		}
		for _, r := range rl.APIResources {
			if i := strings.Index(r.Name, "/"); i > 0 {
				info := m[r.Kind]
				if info != nil {
					info.subresources.Add(r.Name[i+1:])
				}
			}
		}
	}

	list, err = dc.ServerPreferredResources()
	if err != nil {
		logger.Warnf("*** failed to get all preferred server resources for cluster %s: %s", this.cluster.GetName(), err)
		if len(list) == 0 {
			return err
		}
		logger.Infof("found %d resources", len(list))
	}
	for _, rl := range list {
		gv, _ := schema.ParseGroupVersion(rl.GroupVersion)
		this.preferredVersions[gv.Group] = gv.Version
	}

	return nil
}

func (this *ResourceInfos) GetGroups() []schema.GroupVersion {
	this.lock.RLock()
	defer this.lock.RUnlock()
	grps := make([]schema.GroupVersion, len(this.preferredVersions))[0:0]
	for g, v := range this.preferredVersions {
		grps = append(grps, schema.GroupVersion{Group: g, Version: v})
	}
	return grps
}

func (this *ResourceInfos) GetResourceInfos(gv schema.GroupVersion) []*Info {
	this.lock.RLock()
	defer this.lock.RUnlock()
	m := this.groupVersionKinds[gv]
	if m == nil {
		return []*Info{}
	}
	r := make([]*Info, len(m))[0:0]
	for _, i := range m {
		r = append(r, i)
	}
	return r
}

func (this *ResourceInfos) GetPreferred(gk schema.GroupKind) (*Info, error) {
	i := this.getPreferred(gk)
	if i == nil {
		err := this.update()
		if err != nil {
			return nil, err
		}
		i = this.getPreferred(gk)
	}
	if i == nil {
		return nil, fmt.Errorf("%s not known", gk)
	}
	return i, nil
}

func (this *ResourceInfos) getPreferred(gk schema.GroupKind) *Info {
	this.lock.RLock()
	defer this.lock.RUnlock()
	v, ok := this.preferredVersions[gk.Group]
	if !ok {
		return nil
	}
	g := this.groupVersionKinds[schema.GroupVersion{Group: gk.Group, Version: v}]
	if g == nil {
		return nil
	}
	return g[gk.Kind]
}

func (this *ResourceInfos) Get(gvk schema.GroupVersionKind) (*Info, error) {
	i := this.get(gvk)
	if i == nil {
		err := this.update()
		if err != nil {
			return nil, err
		}
		i = this.get(gvk)
	}
	if i == nil {
		return nil, fmt.Errorf("%s not known", gvk)
	}
	return i, nil
}

func (this *ResourceInfos) get(gvk schema.GroupVersionKind) *Info {
	this.lock.RLock()
	defer this.lock.RUnlock()
	g := this.groupVersionKinds[gvk.GroupVersion()]
	if g == nil {
		return nil
	}
	return g[gvk.Kind]
}
