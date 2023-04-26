/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha2

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"sort"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=fbp
// +genclient

// Parser is the Schema for namespace level parser API
type Parser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ParserSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ParserList contains a list of Parsers
type ParserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Parser `json:"items"`
}

type NSParserByName []Parser

func (a NSParserByName) Len() int           { return len(a) }
func (a NSParserByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a NSParserByName) Less(i, j int) bool { return a[i].Name < a[j].Name }

func (list ParserList) Load(sl plugins.SecretLoader) (string, error) {
	var buf bytes.Buffer

	sort.Sort(NSParserByName(list.Items))

	for _, item := range list.Items {
		merge := func(p plugins.Plugin) error {
			if p == nil || reflect.ValueOf(p).IsNil() {
				return nil
			}

			buf.WriteString("[PARSER]\n")
			buf.WriteString(fmt.Sprintf("    Name    %s\n", fmt.Sprintf("%s-%x", item.Name, md5.Sum([]byte(item.Namespace)))))
			buf.WriteString(fmt.Sprintf("    Format    %s\n", p.Name()))

			kvs, err := p.Params(sl)
			if err != nil {
				return err
			}
			buf.WriteString(kvs.String())

			for _, decorder := range item.Spec.Decoders {
				if decorder.DecodeField != "" {
					buf.WriteString(fmt.Sprintf("    Decode_Field    %s\n", decorder.DecodeField))
				}
				if decorder.DecodeFieldAs != "" {
					buf.WriteString(fmt.Sprintf("    Decode_Field_As    %s\n", decorder.DecodeFieldAs))
				}
			}
			return nil
		}

		for i := 0; i < reflect.ValueOf(item.Spec).NumField()-1; i++ {
			p, _ := reflect.ValueOf(item.Spec).Field(i).Interface().(plugins.Plugin)
			if err := merge(p); err != nil {
				return "", err
			}
		}
	}

	return buf.String(), nil
}

func init() {
	SchemeBuilder.Register(&Parser{}, &ParserList{})
}
