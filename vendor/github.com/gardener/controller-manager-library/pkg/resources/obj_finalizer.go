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

import "github.com/gardener/controller-manager-library/pkg/logger"

func hasFinalizer(key string, obj ObjectData) bool {
	for _, name := range obj.GetFinalizers() {
		if name == key {
			return true
		}
	}
	return false
}

func (this *_object) HasFinalizer(key string) bool {
	for _, name := range this.GetFinalizers() {
		if name == key {
			return true
		}
	}
	return false
}

func (this *_object) SetFinalizer(key string) error {
	f := func(obj ObjectData) (bool, error) {
		if !hasFinalizer(key, obj) {
			logger.Infof("setting finalizer %q for %q (%s)", key, this.Description(), this.GetResourceVersion())
			obj.SetFinalizers(append(obj.GetFinalizers(), key))
			return true, nil
		}
		return false, nil
	}
	_, err := this.Modify(f)
	return err
}

func (this *_object) RemoveFinalizer(key string) error {
	f := func(obj ObjectData) (bool, error) {
		list := obj.GetFinalizers()
		for i, name := range list {
			if name == key {
				logger.Infof("removing finalizer %q for %q (%s)", key, this.Description(), this.GetResourceVersion())
				obj.SetFinalizers(append(list[:i], list[i+1:]...))
				return true, nil
			}
		}
		return false, nil
	}
	_, err := this.Modify(f)
	return err
}
