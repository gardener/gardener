package output

import (
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// The file output plugin allows to write the data received through the input plugin to file. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/outputs/file**
type File struct {
	// Absolute directory path to store files. If not set, Fluent Bit will write the files on it's own positioned directory.
	Path string `json:"path,omitempty"`
	// Set file name to store the records. If not set, the file name will be the tag associated with the records.
	File string `json:"file,omitempty"`
	// The format of the file content. See also Format section. Default: out_file.
	// +kubebuilder:validation:Enum:=out_file;plain;csv;ltsv;template
	Format string `json:"format,omitempty"`
	// The character to separate each pair. Applicable only if format is csv or ltsv.
	Delimiter string `json:"delimiter,omitempty"`
	// The character to separate each pair. Applicable only if format is ltsv.
	LabelDelimiter string `json:"labelDelimiter,omitempty"`
	// The format string. Applicable only if format is template.
	Template string `json:"template,omitempty"`
}

func (_ *File) Name() string {
	return "file"
}

func (f *File) Params(_ plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	if f.Path != "" {
		kvs.Insert("Path", f.Path)
	}
	if f.File != "" {
		kvs.Insert("File", f.File)
	}
	if f.Format != "" {
		kvs.Insert("Format", f.Format)
	}
	if f.Delimiter != "" {
		kvs.Insert("Delimiter", f.Delimiter)
	}
	if f.LabelDelimiter != "" {
		kvs.Insert("Label_Delimiter", f.LabelDelimiter)
	}
	if f.Template != "" {
		kvs.Insert("Template", f.Template)
	}
	return kvs, nil
}
