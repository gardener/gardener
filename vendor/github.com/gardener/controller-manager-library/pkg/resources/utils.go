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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func SetAnnotation(o ObjectData, key, value string) bool {
	annos := o.GetAnnotations()
	if annos == nil {
		annos = map[string]string{}
	}
	old, ok := annos[key]
	if !ok || old != value {
		annos[key] = value
		o.SetAnnotations(annos)
		return true
	}
	return false
}

func RemoveAnnotation(o ObjectData, key string) bool {
	annos := o.GetAnnotations()
	if annos != nil {
		if _, ok := annos[key]; ok {
			delete(annos, key)
			o.SetAnnotations(annos)
			return true
		}
	}
	return false
}

func SetOwnerReference(o ObjectData, ref *metav1.OwnerReference) bool {
	refs := o.GetOwnerReferences()
	for _, r := range refs {
		if r.UID == ref.UID {
			return false
		}
	}
	refs = append(refs, *ref)
	o.SetOwnerReferences(refs)
	return true
}

func RemoveOwnerReference(o ObjectData, ref *metav1.OwnerReference) bool {
	refs := o.GetOwnerReferences()
	for i, r := range refs {
		if r.UID == ref.UID {
			refs = append(refs[:i], refs[i+1:]...)
			o.SetOwnerReferences(refs)
			return true
		}
	}
	return false
}

func FilterKeysByGroupKinds(keys ClusterObjectKeySet, kinds ...schema.GroupKind) ClusterObjectKeySet {

	if len(kinds) == 0 {
		return keys.Copy()
	}
	new := ClusterObjectKeySet{}
outer:
	for k := range keys {
		for _, g := range kinds {
			if k.GroupKind() == g {
				new.Add(k)
				continue outer
			}
		}
	}
	return new
}
