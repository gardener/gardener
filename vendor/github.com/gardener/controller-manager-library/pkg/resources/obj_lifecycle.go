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
)

func (this *AbstractObject) Create() error {
	o, err := this.self.GetResource().Create(this.ObjectData)
	if err == nil {
		this.ObjectData = o.Data()
	}
	return err
}

func (this *AbstractObject) CreateOrUpdate() error {
	o, err := this.self.GetResource().CreateOrUpdate(this.ObjectData)
	if err == nil {
		this.ObjectData = o.Data()
	}
	return err
}

func (this *AbstractObject) IsDeleting() bool {
	return this.GetDeletionTimestamp() != nil
}

func (this *AbstractObject) Modify(modifier Modifier) (bool, error) {
	return this.modify(false, modifier)
}

func (this *AbstractObject) ModifyStatus(modifier Modifier) (bool, error) {
	return this.modifyStatus(modifier)
}

func (this *AbstractObject) CreateOrModify(modifier Modifier) (bool, error) {
	return this.modify(true, modifier)
}

func (this *AbstractObject) modifyStatus(modifier Modifier) (bool, error) {
	return this.self.I_modify(true, false, modifier)
}

func (this *AbstractObject) modify(create bool, modifier Modifier) (bool, error) {
	return this.self.I_modify(false, create, modifier)
}

////////////////////////////////////////////////////////////////////////////////
// Methods using internal Resource Interface

func (this *AbstractObject) Update() error {
	result, err := this.self.I_resource().I_update(this.ObjectData)
	if err == nil {
		this.ObjectData = result
	}
	return err
}

func (this *AbstractObject) UpdateStatus() error {
	rsc := this.self.I_resource()
	if !rsc.Info().HasStatusSubResource() {
		return fmt.Errorf("resource %q has no status sub resource", rsc.GroupVersionKind())
	}
	result, err := rsc.I_updateStatus(this.ObjectData)
	if err == nil {
		this.ObjectData = result
	}
	return err
}

func (this *AbstractObject) Delete() error {
	return this.self.I_resource().I_delete(this)
}

func (this *AbstractObject) UpdateFromCache() error {
	obj, err := this.self.GetResource().GetCached(this.ObjectName())
	if err == nil {
		this.ObjectData = obj.Data()
	}
	return err
}
