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
	"k8s.io/apimachinery/pkg/api/errors"
)

// I_Object is the internal object interface used by the Abstract Object
// implementation to implement standart object methods.
type I_Object interface {
	Object

	I_modify(status_only, create bool, modifier Modifier) (bool, error)
	I_resource() Internal
}

// _i_object is the implementation of the internal object interface used by
// the abstract object.
// To avoid to potentially  expose those additional methods the object
// implementation does NOT implement the internal interface. Instead,
// it uses an internal wrapper object, that implements this interface.
// This wrapper is then passed to the abstract object implementation
// to be used to implement a broader set of methods in a generic manner.
type _i_object struct {
	*_object
}

func (this *_i_object) I_resource() Internal {
	return this.resource
}

func (this *_i_object) I_modify(status_only, create bool, modifier Modifier) (bool, error) {
	var lasterr error

	data := this.Data().DeepCopyObject().(ObjectData)

	cnt := 10

	if create {
		err := this.resource.I_get(data)
		if err != nil {
			if !errors.IsNotFound(err) {
				return false, err
			}
			_, err := modifier(data)
			if err != nil {
				return false, err
			}
			created, err := this.resource.I_create(data)
			if err == nil {
				this.ObjectData = created
				return true, nil
			}
			if !errors.IsAlreadyExists(err) {
				return false, err
			}
			err = this.resource.I_get(data)
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
				modified, lasterr = this.resource.I_updateStatus(data)
			} else {
				modified, lasterr = this.resource.I_update(data)
			}
			if lasterr == nil {
				this.ObjectData = modified
				return mod, nil
			}
			if !errors.IsConflict(lasterr) {
				return mod, lasterr
			}
			err = this.resource.I_get(data)
		}
		if err != nil {
			return mod, err
		}
		cnt--
	}
	return true, lasterr
}
