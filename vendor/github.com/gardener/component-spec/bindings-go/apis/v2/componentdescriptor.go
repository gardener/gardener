// Copyright 2020 Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v2

import (
	"bytes"
	"encoding/json"
	"errors"
)

const SchemaVersion = "v2"

var (
	NotFound = errors.New("NotFound")
)

// Metadata defines the metadata of the component descriptor.
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type Metadata struct {
	// Version is the schema version of the component descriptor.
	Version string `json:"schemaVersion"`
}

// ProviderType describes the provider type of component in the origin's context.
// Defines whether the component is created by a third party or internally.
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type ProviderType string

const (
	// InternalProvider defines a internal provider type
	// which describes a internally maintained component in the origin's context.
	InternalProvider ProviderType = "internal"
	// ExternalProvider defines a external provider type
	// which describes a component maintained by a third party vendor in the origin's context.
	ExternalProvider ProviderType = "external"
)

// ResourceRelation describes the type of a resource.
// Defines whether the component is created by a third party or internally.
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type ResourceRelation string

const (
	// LocalRelation defines a internal relation
	// which describes a internally maintained resource in the origin's context.
	LocalRelation ResourceRelation = "local"
	// ExternalRelation defines a external relation
	// which describes a resource maintained by a third party vendor in the origin's context.
	ExternalRelation ResourceRelation = "external"
)

// Spec defines a versioned virtual component with a source and dependencies.
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type ComponentDescriptor struct {
	// Metadata specifies the schema version of the component.
	Metadata Metadata `json:"meta"`
	// Spec contains the specification of the component.
	ComponentSpec `json:"component"`
}

// ComponentSpec defines a virtual component with
// a repository context, source and dependencies.
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type ComponentSpec struct {
	ObjectMeta `json:",inline"`
	// RepositoryContexts defines the previous repositories of the component
	RepositoryContexts []RepositoryContext `json:"repositoryContexts"`
	// Provider defines the provider type of a component.
	// It can be external or internal.
	Provider ProviderType `json:"provider"`
	// Sources defines sources that produced the component
	Sources []Source `json:"sources"`
	// ComponentReferences references component dependencies that can be resolved in the current context.
	ComponentReferences []ComponentReference `json:"componentReferences"`
	// Resources defines all resources that are created by the component and by a third party.
	Resources []Resource `json:"resources"`
}

// +k8s:deepcopy-gen=true
// RepositoryContext describes a repository context.
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type RepositoryContext struct {
	// Type defines the type of the component repository to resolve references.
	Type string `json:"type"`
	// BaseURL is the base url of the repository to resolve components.
	BaseURL string `json:"baseUrl"`
}

// +k8s:deepcopy-gen=true
// ObjectMeta defines a object that is uniquely identified by its name and version.
type ObjectMeta struct {
	// Name is the context unique name of the object.
	Name string `json:"name"`
	// Version is the semver version of the object.
	Version string `json:"version"`
	// Labels defines an optional set of additional labels
	// describing the object.
	// +optional
	Labels Labels `json:"labels,omitempty"`
}

// GetName returns the name of the object.
func (o ObjectMeta) GetName() string {
	return o.Name
}

// SetName sets the name of the object.
func (o *ObjectMeta) SetName(name string) {
	o.Name = name
}

// GetVersion returns the version of the object.
func (o ObjectMeta) GetVersion() string {
	return o.Version
}

// SetVersion sets the version of the object.
func (o *ObjectMeta) SetVersion(version string) {
	o.Version = version
}

// GetLabels returns the label of the object.
func (o ObjectMeta) GetLabels() Labels {
	return o.Labels
}

// SetLabels sets the labels of the object.
func (o *ObjectMeta) SetLabels(labels []Label) {
	o.Labels = labels
}

const (
	SystemIdentityName    = "name"
	SystemIdentityVersion = "version"
)

// Identity describes the identity of an object.
// Only ascii characters are allowed
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type Identity map[string]string

// Digest returns the object digest of an identity
func (i Identity) Digest() []byte {
	data, _ := json.Marshal(i)
	return data
}

// IdentityObjectMeta defines a object that is uniquely identified by its identity.
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type IdentityObjectMeta struct {
	// Name is the context unique name of the object.
	Name string `json:"name"`
	// Version is the semver version of the object.
	Version string `json:"version"`
	// Type describes the type of the object.
	Type string `json:"type"`
	// ExtraIdentity is the identity of an object.
	// An additional label with key "name" ist not allowed
	ExtraIdentity Identity `json:"extraIdentity,omitempty"`
	// Labels defines an optional set of additional labels
	// describing the object.
	// +optional
	Labels Labels `json:"labels,omitempty"`
}

// GetName returns the name of the object.
func (o IdentityObjectMeta) GetName() string {
	return o.Name
}

// SetName sets the name of the object.
func (o *IdentityObjectMeta) SetName(name string) {
	o.Name = name
}

// GetVersion returns the version of the object.
func (o IdentityObjectMeta) GetVersion() string {
	return o.Version
}

// SetVersion sets the version of the object.
func (o *IdentityObjectMeta) SetVersion(version string) {
	o.Version = version
}

// GetType returns the type of the object.
func (o IdentityObjectMeta) GetType() string {
	return o.Type
}

// SetType sets the type of the object.
func (o *IdentityObjectMeta) SetType(ttype string) {
	o.Type = ttype
}

// GetLabels returns the label of the object.
func (o IdentityObjectMeta) GetLabels() Labels {
	return o.Labels
}

// SetLabels sets the labels of the object.
func (o *IdentityObjectMeta) SetLabels(labels []Label) {
	o.Labels = labels
}

// SetExtraIdentity sets the identity of the object.
func (o *IdentityObjectMeta) SetExtraIdentity(identity Identity) {
	o.ExtraIdentity = identity
}

// GetLabels returns the identity of the object.
func (o *IdentityObjectMeta) GetIdentity() Identity {
	identity := map[string]string{}
	for k, v := range o.ExtraIdentity {
		identity[k] = v
	}
	identity[SystemIdentityName] = o.Name
	return identity
}

// GetIdentityDigest returns the digest of the object's identity.
func (o *IdentityObjectMeta) GetIdentityDigest() []byte {
	return o.GetIdentity().Digest()
}

// +k8s:deepcopy-gen=true
// ObjectType describes the type of a object
type ObjectType struct {
	// Type describes the type of the object.
	Type string `json:"type"`
}

// GetType returns the type of the object.
func (t ObjectType) GetType() string {
	return t.Type
}

// SetType sets the type of the object.
func (t *ObjectType) SetType(ttype string) {
	t.Type = ttype
}

// Label is a label that can be set on objects.
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type Label struct {
	// Name is the unique name of the label.
	Name string `json:"name"`
	// Value is the json/yaml data of the label
	Value json.RawMessage `json:"value"`
}

// Labels describe a list of labels
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type Labels []Label

// Get returns the label witht the given name
func (l Labels) Get(name string) ([]byte, bool) {
	for _, label := range l {
		if label.Name == name {
			return label.Value, true
		}
	}
	return nil, false
}

// NameAccessor describes a accessor for a named object.
type NameAccessor interface {
	// GetName returns the name of the object.
	GetName() string
	// SetName sets the name of the object.
	SetName(name string)
}

// VersionAccessor describes a accessor for a versioned object.
type VersionAccessor interface {
	// GetVersion returns the version of the object.
	GetVersion() string
	// SetVersion sets the version of the object.
	SetVersion(version string)
}

// LabelsAccessor describes a accessor for a labeled object.
type LabelsAccessor interface {
	// GetLabels returns the labels of the object.
	GetLabels() Labels
	// SetLabels sets the labels of the object.
	SetLabels(labels []Label)
}

// ObjectMetaAccessor describes a accessor for named and versioned object.
type ObjectMetaAccessor interface {
	NameAccessor
	VersionAccessor
	LabelsAccessor
}

// TypedObjectAccessor defines the accessor for a typed component with additional data.
type TypedObjectAccessor interface {
	// GetType returns the type of the access object.
	GetType() string
	// SetType sets the type of the access object.
	SetType(ttype string)
	// GetData returns the custom data of a component.
	GetData() ([]byte, error)
	// SetData sets the custom data of a component.
	SetData([]byte) error
}

// NewEmptyUnstructured creates a new typed object without additional data.
func NewEmptyUnstructured(ttype string) *UnstructuredAccessType {
	return NewUnstructuredType(ttype, nil)
}

// NewCustomType creates a new custom typed object.
func NewUnstructuredType(ttype string, data map[string]interface{}) *UnstructuredAccessType {
	unstr := &UnstructuredAccessType{}
	unstr.Object = data
	unstr.SetType(ttype)
	return unstr
}

// UnstructuredAccessType describes a generic access type.
// +k8s:openapi-gen=true
type UnstructuredAccessType struct {
	ObjectType `json:",inline"`
	Raw        []byte                 `json:"-"`
	Object     map[string]interface{} `json:"object"`
}

func (u *UnstructuredAccessType) SetType(ttype string) {
	u.ObjectType.SetType(ttype)
	if u.Object == nil {
		u.Object = make(map[string]interface{})
	}
	u.Object["type"] = ttype
}

// DeepCopyInto is deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (u *UnstructuredAccessType) DeepCopyInto(out *UnstructuredAccessType) {
	*out = *u
	raw := make([]byte, len(u.Raw))
	copy(raw, u.Raw)
	_ = out.SetData(raw)
}

// DeepCopy is deepcopy function, copying the receiver, creating a new UnstructuredAccessType.
func (u *UnstructuredAccessType) DeepCopy() *UnstructuredAccessType {
	if u == nil {
		return nil
	}
	out := new(UnstructuredAccessType)
	u.DeepCopyInto(out)
	return out
}

func (u *UnstructuredAccessType) Decode(data []byte, into TypedObjectAccessor) error {
	uObj, ok := into.(*UnstructuredAccessType)
	if !ok {
		return errors.New("unable to decode data into non unstructured type")
	}

	return json.Unmarshal(data, uObj)
}

func (u *UnstructuredAccessType) Encode(acc TypedObjectAccessor) ([]byte, error) {
	uObj, ok := acc.(*UnstructuredAccessType)
	if !ok {
		return nil, errors.New("unable to decode data into non unstructured type")
	}
	return json.Marshal(uObj)
}

var _ TypedObjectCodec = &UnstructuredAccessType{}

func (u UnstructuredAccessType) GetData() ([]byte, error) {
	data, err := json.Marshal(u.Object)
	if err != nil {
		return nil, err
	}
	if n := bytes.Compare(data, u.Raw); n != 0 {
		u.Raw = data
	}
	return u.Raw, nil
}

func (u *UnstructuredAccessType) SetData(data []byte) error {
	obj := map[string]interface{}{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	u.Raw = data
	u.Object = obj
	return nil
}

// UnmarshalJSON implements a custom json unmarshal method for a unstructured typed object.
func (u *UnstructuredAccessType) UnmarshalJSON(data []byte) error {
	typedObj := ObjectType{}
	if err := json.Unmarshal(data, &typedObj); err != nil {
		return err
	}

	obj := UnstructuredAccessType{
		ObjectType: typedObj,
	}
	if err := obj.SetData(data); err != nil {
		return err
	}
	*u = obj
	return nil
}

// MarshalJSON implements a custom json unmarshal method for a unstructured type.
func (u *UnstructuredAccessType) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(u.Object)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Source is the definition of a component's source.
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type Source struct {
	IdentityObjectMeta `json:",inline"`
	Access             *UnstructuredAccessType `json:"access"`
}

// SourceRef defines a reference to a source
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type SourceRef struct {
	// IdentitySelector defines the identity that is used to match a source.
	IdentitySelector map[string]string `json:"identitySelector,omitempty"`
	// Labels defines an optional set of additional labels
	// describing the object.
	// +optional
	Labels Labels `json:"labels,omitempty"`
}

// Resource describes a resource dependency of a component.
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type Resource struct {
	IdentityObjectMeta `json:",inline"`

	// Relation describes the relation of the resource to the component.
	// Can be a local or external resource
	Relation ResourceRelation `json:"relation,omitempty"`

	// SourceRef defines a list of source names.
	// These names reference the sources defines in `component.sources`.
	SourceRef []SourceRef `json:"srcRef,omitempty"`

	// Access describes the type specific method to
	// access the defined resource.
	Access *UnstructuredAccessType `json:"access"`
}

// ComponentReference describes the reference to another component in the registry.
// +k8s:deepcopy-gen=true
// +k8s:openapi-gen=true
type ComponentReference struct {
	// Name is the context unique name of the object.
	Name string `json:"name"`
	// ComponentName describes the remote name of the referenced object
	ComponentName string `json:"componentName"`
	// Version is the semver version of the object.
	Version string `json:"version"`
	// ExtraIdentity is the identity of an object.
	// An additional label with key "name" ist not allowed
	ExtraIdentity Identity `json:"extraIdentity,omitempty"`
	// Labels defines an optional set of additional labels
	// describing the object.
	// +optional
	Labels Labels `json:"labels,omitempty"`
}

// GetName returns the name of the object.
func (o ComponentReference) GetName() string {
	return o.Name
}

// SetName sets the name of the object.
func (o *ComponentReference) SetName(name string) {
	o.Name = name
}

// GetVersion returns the version of the object.
func (o ComponentReference) GetVersion() string {
	return o.Version
}

// SetVersion sets the version of the object.
func (o *ComponentReference) SetVersion(version string) {
	o.Version = version
}

// GetLabels returns the label of the object.
func (o ComponentReference) GetLabels() Labels {
	return o.Labels
}

// SetLabels sets the labels of the object.
func (o *ComponentReference) SetLabels(labels []Label) {
	o.Labels = labels
}

// GetLabels returns the identity of the object.
func (o *ComponentReference) GetIdentity() Identity {
	identity := map[string]string{}
	for k, v := range o.ExtraIdentity {
		identity[k] = v
	}
	identity[SystemIdentityName] = o.Name
	return identity
}

// GetIdentityDigest returns the digest of the object's identity.
func (o *ComponentReference) GetIdentityDigest() []byte {
	return o.GetIdentity().Digest()
}
