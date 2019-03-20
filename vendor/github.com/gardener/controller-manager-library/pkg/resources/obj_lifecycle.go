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
	"k8s.io/apimachinery/pkg/api/errors"
	restclient "k8s.io/client-go/rest"
)

func (this *_object) Create() error {
	o, err := this.resource.Create(this.ObjectData)
	if err == nil {
		this.ObjectData = o.Data()
	}
	return err
}

func (this *_object) CreateOrUpdate() error {
	o, err := this.resource.CreateOrUpdate(this.ObjectData)
	if err == nil {
		this.ObjectData = o.Data()
	}
	return err
}

func (this *_object) IsDeleting() bool {
	return this.GetDeletionTimestamp() != nil
}

func (this *_object) Update() error {
	result, err := this.resource._update(this.ObjectData)
	if err == nil {
		this.ObjectData = result
	}
	return err
}

func (this *_object) UpdateStatus() error {
	if !this.resource.Info().HasStatusSubResource() {
		return fmt.Errorf("resource %q has no status sub resource", this.resource.GroupVersionKind())
	}
	result, err := this.resource._updateStatus(this.ObjectData)
	if err == nil {
		this.ObjectData = result
	}
	return err
}

func (this *_object) Modify(modifier Modifier) (bool, error) {
	return this.modify(false, modifier)
}

func (this *_object) ModifyStatus(modifier Modifier) (bool, error) {
	return this.modifyStatus(modifier)
}

func (this *_object) CreateOrModify(modifier Modifier) (bool, error) {
	return this.modify(true, modifier)
}

func (this *_object) modifyStatus(modifier Modifier) (bool, error) {
	return this._modify(true, false, modifier)
}

func (this *_object) modify(create bool, modifier Modifier) (bool, error) {
	return this._modify(false, create, modifier)
}

func (this *_object) _modify(status_only, create bool, modifier Modifier) (bool, error) {
	var lasterr error

	data := this.GetObject().DeepCopyObject().(ObjectData)

	cnt := 10

	if create {
		err := this.resource._get(data)
		if err != nil {
			if !errors.IsNotFound(err) {
				return false, err
			}
			_, err := modifier(data)
			if err != nil {
				return false, err
			}
			created, err := this.resource._create(data)
			if err == nil {
				this.ObjectData = created
				return true, nil
			}
			if !errors.IsAlreadyExists(err) {
				return false, err
			}
			err = this.resource._get(data)
			if err != nil {
				return false, err
			}
		}
	}

	for cnt > 0 {
		var modified ObjectData
		mod, err := modifier(data)
		if !mod {
			return mod, err
		}
		if err == nil {
			if status_only {
				modified, lasterr = this.resource._updateStatus(data)
			} else {
				modified, lasterr = this.resource._update(data)
			}
			if lasterr == nil {
				this.ObjectData = modified
				return mod, nil
			}
			if !errors.IsConflict(lasterr) {
				return mod, lasterr
			}
			err = this.resource._get(data)
		}
		if err != nil {
			return mod, err
		}
		cnt--
	}
	return true, lasterr
}

func (this *_object) delete(client restclient.Interface) error {
	return this.resource.objectRequest(client.Delete(), this).Do().Error()
}

func (this *_object) Delete() error {
	return this.delete(this.resource.getClient())
}
