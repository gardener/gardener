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

package utils

import (
	"fmt"
	"reflect"

	"github.com/Masterminds/semver"
)

// Versioned maintains a set of provided object versions and yields the best
// such version for given required version.
type Versioned struct {
	etype    reflect.Type
	versions map[string]interface{}
	def      interface{}
}

func NewVersioned(proto interface{}) *Versioned {
	t, ok := proto.(reflect.Type)
	if !ok {
		t = reflect.TypeOf(proto)
	}
	return &Versioned{t, map[string]interface{}{}, nil}
}

func (this *Versioned) SetDefault(obj interface{}) error {
	if reflect.TypeOf(obj) != this.etype {
		return fmt.Errorf("invalid element type, found %s, but expected %s", reflect.TypeOf(obj), this.etype)
	}
	this.def = obj
	return nil
}

func (this *Versioned) RegisterVersion(v *semver.Version, obj interface{}) error {
	if reflect.TypeOf(obj) != this.etype {
		return fmt.Errorf("invalid element type, found %s, but expected %s", reflect.TypeOf(obj), this.etype)
	}
	this.versions[v.String()] = obj
	return nil
}

func (this *Versioned) MustRegisterVersion(v *semver.Version, obj interface{}) {
	err := this.RegisterVersion(v, obj)
	if err != nil {
		panic(fmt.Sprintf("cannot register versioned object: %s", err))
	}
}

func (this *Versioned) GetFor(req *semver.Version) interface{} {
	var found *semver.Version
	obj := this.def

	for v, o := range this.versions {
		vers, _ := semver.NewVersion(v)
		if !vers.GreaterThan(req) {
			if found == nil || vers.GreaterThan(found) {
				found, obj = vers, o
			}
		}
	}
	return obj
}
