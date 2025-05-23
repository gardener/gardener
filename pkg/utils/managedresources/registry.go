// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresources

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/andybalholm/brotli"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	forkedyaml "github.com/gardener/gardener/third_party/gopkg.in/yaml.v2"
)

// Registry stores objects and their serialized form. It allows to compute a map of all registered objects that can be
// used as part of a Secret's data which is referenced by a ManagedResource.
type Registry struct {
	scheme           *runtime.Scheme
	codec            runtime.Codec
	nameToObject     map[string]*object
	isYAMLSerializer bool
}

type object struct {
	obj           client.Object
	serialization []byte
}

// NewRegistry returns a new registry for resources. The given scheme, codec, and serializer must know all the resource
// types that will later be added to the registry.
func NewRegistry(scheme *runtime.Scheme, codec serializer.CodecFactory, serializer *jsonserializer.Serializer) *Registry {
	var groupVersions schema.GroupVersions
	for k := range scheme.AllKnownTypes() {
		groupVersions = append(groupVersions, k.GroupVersion())
	}

	// Use set to remove duplicates
	groupVersions = sets.New(groupVersions...).UnsortedList()

	// Sort groupVersions to ensure groupVersions.Identifier() is stable key
	// for the map in https://github.com/kubernetes/apimachinery/blob/v0.26.1/pkg/runtime/serializer/versioning/versioning.go#L94
	slices.SortStableFunc(groupVersions, func(a, b schema.GroupVersion) int {
		if a.Group == b.Group {
			return cmp.Compare(a.Version, b.Version)
		}
		return cmp.Compare(a.Group, b.Group)
	})

	// A workaround to incosistent/unstable ordering in yaml.v2 when encoding maps
	// Can be removed once k8s.io/apimachinery/pkg/runtime/serializer/json migrates to yaml.v3
	// or the issue is resolved upstream in yaml.v2
	// Please see https://github.com/go-yaml/yaml/pull/736
	serializerIdentifier := struct {
		YAML string `json:"yaml"`
	}{}

	utilruntime.Must(json.Unmarshal([]byte(serializer.Identifier()), &serializerIdentifier))

	return &Registry{
		scheme:           scheme,
		codec:            codec.CodecForVersions(serializer, serializer, groupVersions, groupVersions),
		nameToObject:     make(map[string]*object),
		isYAMLSerializer: serializerIdentifier.YAML == "true",
	}
}

// Add adds the given object to the registry. It computes a filename based on its type, namespace, and name. It serializes
// the object to YAML and stores both representations (object and serialization) in the registry.
func (r *Registry) Add(objs ...client.Object) error {
	for _, obj := range objs {
		if obj == nil || reflect.ValueOf(obj) == reflect.Zero(reflect.TypeOf(obj)) {
			continue
		}

		objectName, err := r.objectName(obj)
		if err != nil {
			return err
		}
		filename := objectName + ".yaml"

		if _, ok := r.nameToObject[filename]; ok {
			return fmt.Errorf("duplicate filename in registry: %q", filename)
		}

		serializationYAML, err := runtime.Encode(r.codec, obj)
		if err != nil {
			return err
		}

		// We use a copy of the upstream package as a workaround
		// to incosistent/unstable ordering in yaml.v2 when encoding maps
		// Can be removed once k8s.io/apimachinery/pkg/runtime/serializer/json migrates to yaml.v3
		// or the issue is resolved upstream in yaml.v2
		// Please see https://github.com/go-yaml/yaml/pull/736
		if r.isYAMLSerializer {
			var anyObj any
			if err := forkedyaml.Unmarshal(serializationYAML, &anyObj); err != nil {
				return err
			}

			serBytes, err := forkedyaml.Marshal(anyObj)
			if err != nil {
				return err
			}
			serializationYAML = serBytes
		}

		r.nameToObject[filename] = &object{
			obj:           obj,
			serialization: serializationYAML,
		}
	}

	return nil
}

// AddSerialized adds the provided serialized YAML for the registry.
// The provided filename is required and determines the internal sorting order.
func (r *Registry) AddSerialized(filename string, serializationYAML []byte) {
	r.nameToObject[filename] = &object{serialization: serializationYAML}
}

// SerializedObjects returns a map which can be used as secret data of a managed resource.
// The map holds a single key `data.yaml.br` with a value containing all objects,
// concatenated and compressed by the Brotli algorithm.
func (r *Registry) SerializedObjects() (map[string][]byte, error) {
	objectKeys := maps.Keys(r.nameToObject)
	slices.Sort(objectKeys)

	var (
		buf bytes.Buffer
		w   = brotli.NewWriter(&buf)
	)

	for i, objectKey := range objectKeys {
		if _, err := w.Write(r.nameToObject[objectKey].serialization); err != nil {
			return nil, err
		}

		// Some manifests don't end with a new line, add it here.
		if !bytes.HasSuffix(r.nameToObject[objectKey].serialization, []byte("\n")) {
			if _, err := w.Write([]byte("\n")); err != nil {
				return nil, err
			}
		}
		// Add separator for manifests at the end, before next manifest.
		if !bytes.HasSuffix(r.nameToObject[objectKey].serialization, []byte("---\n")) && i < len(objectKeys)-1 {
			if _, err := w.Write([]byte("---\n")); err != nil {
				return nil, err
			}
		}
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return map[string][]byte{
		resourcesv1alpha1.CompressedDataKey: buf.Bytes(),
	}, nil
}

// AddAllAndSerialize calls Add() for all the given objects before calling SerializedObjects().
func (r *Registry) AddAllAndSerialize(objects ...client.Object) (map[string][]byte, error) {
	if err := r.Add(objects...); err != nil {
		return nil, err
	}
	return r.SerializedObjects()
}

// RegisteredObjects returns a slice of registered objects.
func (r *Registry) RegisteredObjects() []client.Object {
	objectKeys := maps.Keys(r.nameToObject)
	slices.Sort(objectKeys)

	out := make([]client.Object, 0, len(r.nameToObject))
	for _, objectKey := range objectKeys {
		out = append(out, r.nameToObject[objectKey].obj)
	}
	return out
}

// String returns the string representation of the registry.
func (r *Registry) String() string {
	out := make([]string, 0, len(r.nameToObject))
	for name, object := range r.nameToObject {
		out = append(out, fmt.Sprintf("* %s:\n%s", name, object.serialization))
	}
	return strings.Join(out, "\n\n")
}

func (r *Registry) objectName(obj client.Object) (string, error) {
	gvk, err := apiutil.GVKForObject(obj, r.scheme)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"%s__%s__%s",
		strings.ToLower(gvk.String()),
		obj.GetNamespace(),
		strings.ReplaceAll(obj.GetName(), ":", "_"),
	), nil
}
