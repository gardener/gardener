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
)

func AssureBoolValue(mod bool, old bool, value bool) (bool, bool) {
	if old != value {
		return value, true
	}
	return old, mod
}

func AssureStringValue(mod bool, old string, value string) (string, bool) {
	if old != value {
		return value, true
	}
	return old, mod
}

func AssureIntValue(mod bool, old int, value int) (int, bool) {
	if old != value {
		return value, true
	}
	return old, mod
}

func AssureInt64Value(mod bool, old int64, value int64) (int64, bool) {
	if old != value {
		return value, true
	}
	return old, mod
}

///////////////////////////////////////////////////////////////////////////////

func AssureStringPtrValue(mod bool, old *string, value string) (*string, bool) {
	if old == nil || *old != value {
		return &value, true
	}
	return old, mod
}

func AssureIntPtrValue(mod bool, old *int, value int) (*int, bool) {
	if old == nil || *old != value {
		return &value, true
	}
	return old, mod
}

func AssureInt64PtrValue(mod bool, old *int64, value int64) (*int64, bool) {
	if old == nil || *old != value {
		return &value, true
	}
	return old, mod
}

func AssureStringPtrPtr(mod bool, old *string, ptr *string) (*string, bool) {
	if ptr != nil {
		return AssureStringPtrValue(mod, old, *ptr)
	}
	if old != nil {
		return nil, true
	}
	return old, mod
}

func AssureInt64PtrPtr(mod bool, old *int64, ptr *int64) (*int64, bool) {
	if ptr != nil {
		return AssureInt64PtrValue(mod, old, *ptr)
	}
	if old != nil {
		return nil, true
	}
	return old, mod
}

func AssureStringSet(mod bool, old []string, value StringSet) ([]string, bool) {
	if !value.Equals(NewStringSetByArray(old)) {
		return value.AsArray(), true
	}
	return old, mod
}

///////////////////////////////////////////////////////////////////////////////

type ModificationState struct {
	Modified bool
}

func (this *ModificationState) IsModified() bool {
	return this.Modified
}

func (this *ModificationState) OnModified(f func() error) error {
	if this.Modified {
		return f()
	}
	return nil
}

func (this *ModificationState) Modify(m bool) *ModificationState {
	this.Modified = this.Modified || m
	return this
}

func (this *ModificationState) AssureBoolValue(dst *bool, val bool) *ModificationState {
	*dst, this.Modified = AssureBoolValue(this.Modified, *dst, val)
	return this
}

func (this *ModificationState) AssureStringValue(dst *string, val string) *ModificationState {
	*dst, this.Modified = AssureStringValue(this.Modified, *dst, val)
	return this
}

func (this *ModificationState) AssureIntValue(dst *int, val int) *ModificationState {
	*dst, this.Modified = AssureIntValue(this.Modified, *dst, val)
	return this
}

func (this *ModificationState) AssureInt64Value(dst *int64, val int64) *ModificationState {
	*dst, this.Modified = AssureInt64Value(this.Modified, *dst, val)
	return this
}

func (this *ModificationState) AssureStringPtrValue(dst **string, val string) *ModificationState {
	*dst, this.Modified = AssureStringPtrValue(this.Modified, *dst, val)
	return this
}

func (this *ModificationState) AssureIntPtrValue(dst **int, val int) *ModificationState {
	*dst, this.Modified = AssureIntPtrValue(this.Modified, *dst, val)
	return this
}

func (this *ModificationState) AssureStringPtrPtr(dst **string, ptr *string) *ModificationState {
	*dst, this.Modified = AssureStringPtrPtr(this.Modified, *dst, ptr)
	return this
}

func (this *ModificationState) AssureInt64PtrValue(dst **int64, val int64) *ModificationState {
	*dst, this.Modified = AssureInt64PtrValue(this.Modified, *dst, val)
	return this
}

func (this *ModificationState) AssureInt64PtrPtr(dst **int64, ptr *int64) *ModificationState {
	*dst, this.Modified = AssureInt64PtrPtr(this.Modified, *dst, ptr)
	return this
}

func (this *ModificationState) AssureStringSet(dst *[]string, val StringSet) *ModificationState {
	*dst, this.Modified = AssureStringSet(this.Modified, *dst, val)
	return this
}

////////////////////////////////////////////////////////////////////////////////

func FillStringValue(msg string, variable *string, value string) error {
	if *variable != "" && value != "" && *variable != value {
		return fmt.Errorf("%s: value mismatch", msg)
	}
	if value != "" {
		*variable = value
	}
	return nil
}
