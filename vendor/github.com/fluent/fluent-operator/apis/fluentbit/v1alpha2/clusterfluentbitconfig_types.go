/*
Copyright 2021.

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
	"fmt"
	"sort"

	"github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2/plugins/params"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FluentBitConfigSpec defines the desired state of ClusterFluentBitConfig
type FluentBitConfigSpec struct {
	// Service defines the global behaviour of the Fluent Bit engine.
	Service *Service `json:"service,omitempty"`
	// Select input plugins
	InputSelector metav1.LabelSelector `json:"inputSelector,omitempty"`
	// Select filter plugins
	FilterSelector metav1.LabelSelector `json:"filterSelector,omitempty"`
	// Select output plugins
	OutputSelector metav1.LabelSelector `json:"outputSelector,omitempty"`
	// Select parser plugins
	ParserSelector metav1.LabelSelector `json:"parserSelector,omitempty"`
	//If namespace is defined, then the configmap and secret for fluent-bit is in this namespace.
	//If it is not defined, it is in the namespace of the fluentd-operator
	Namespace *string `json:"namespace,omitempty"`
}

type Service struct {
	// If true go to background on start
	Daemon *bool `json:"daemon,omitempty"`
	// Interval to flush output
	FlushSeconds *int64 `json:"flushSeconds,omitempty"`
	// Wait time on exit
	GraceSeconds *int64 `json:"graceSeconds,omitempty"`
	// Address to listen
	// +kubebuilder:validation:Pattern:="^\\d{1,3}.\\d{1,3}.\\d{1,3}.\\d{1,3}$"
	HttpListen string `json:"httpListen,omitempty"`
	// Port to listen
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=65535
	HttpPort *int32 `json:"httpPort,omitempty"`
	// If true enable statistics HTTP server
	HttpServer *bool `json:"httpServer,omitempty"`
	// File to log diagnostic output
	LogFile string `json:"logFile,omitempty"`
	// Diagnostic level (error/warning/info/debug/trace)
	// +kubebuilder:validation:Enum:=error;warning;info;debug;trace
	LogLevel string `json:"logLevel,omitempty"`
	// Optional 'parsers' config file (can be multiple)
	ParsersFile string `json:"parsersFile,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=fbc,scope=Cluster
// +genclient
// +genclient:nonNamespaced

// ClusterFluentBitConfig is the Schema for the cluster-level fluentbitconfigs API
type ClusterFluentBitConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec FluentBitConfigSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterFluentBitConfigList contains a list of ClusterFluentBitConfig
type ClusterFluentBitConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterFluentBitConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterFluentBitConfig{}, &ClusterFluentBitConfigList{})
}

func (s *Service) Params() *params.KVs {
	m := params.NewKVs()
	if s.Daemon != nil {
		m.Insert("Daemon", fmt.Sprint(*s.Daemon))
	}
	if s.FlushSeconds != nil {
		m.Insert("Flush", fmt.Sprint(*s.FlushSeconds))
	}
	if s.GraceSeconds != nil {
		m.Insert("Grace", fmt.Sprint(*s.GraceSeconds))
	}
	if s.HttpListen != "" {
		m.Insert("Http_Listen", s.HttpListen)
	}
	if s.HttpPort != nil {
		m.Insert("Http_Port", fmt.Sprint(*s.HttpPort))
	}
	if s.HttpServer != nil {
		m.Insert("Http_Server", fmt.Sprint(*s.HttpServer))
	}
	if s.LogFile != "" {
		m.Insert("Log_File", s.LogFile)
	}
	if s.LogLevel != "" {
		m.Insert("Log_Level", s.LogLevel)
	}
	if s.ParsersFile != "" {
		m.Insert("Parsers_File", s.ParsersFile)
	}
	return m
}

func (cfg ClusterFluentBitConfig) RenderMainConfig(sl plugins.SecretLoader, inputs ClusterInputList, filters ClusterFilterList, outputs ClusterOutputList) (string, error) {
	var buf bytes.Buffer

	// The Service defines the global behaviour of the Fluent Bit engine.
	if cfg.Spec.Service != nil {
		buf.WriteString("[Service]\n")
		buf.WriteString(cfg.Spec.Service.Params().String())
	}

	inputSections, err := inputs.Load(sl)
	if err != nil {
		return "", err
	}

	filterSections, err := filters.Load(sl)
	if err != nil {
		return "", err
	}

	outputSections, err := outputs.Load(sl)
	if err != nil {
		return "", err
	}
	if inputSections != "" && outputSections == "" {
		outputSections = `[Output]
    Name    null
    Match   *`
	}

	buf.WriteString(inputSections)
	buf.WriteString(filterSections)
	buf.WriteString(outputSections)

	return buf.String(), nil
}

func (cfg ClusterFluentBitConfig) RenderParserConfig(sl plugins.SecretLoader, parsers ClusterParserList) (string, error) {
	var buf bytes.Buffer

	parserSections, err := parsers.Load(sl)
	if err != nil {
		return "", err
	}

	buf.WriteString(parserSections)

	return buf.String(), nil
}

// +kubebuilder:object:generate:=false
type Script struct {
	Name    string
	Content string
}

// +kubebuilder:object:generate:=false
// ByName implements sort.Interface for []Script based on the Name field.
type ByName []Script

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].Name < a[j].Name }

func (cfg ClusterFluentBitConfig) RenderLuaScript(cl plugins.ConfigMapLoader, filters ClusterFilterList, namespace string) ([]Script, error) {

	scripts := make([]Script, 0)
	for _, f := range filters.Items {
		for _, p := range f.Spec.FilterItems {
			if p.Lua != nil {
				script, err := cl.LoadConfigMap(p.Lua.Script, namespace)
				if err != nil {
					return nil, err
				}
				scripts = append(scripts, Script{Name: p.Lua.Script.Key, Content: script})
			}
		}
	}

	sort.Sort(ByName(scripts))

	return scripts, nil
}
