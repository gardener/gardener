package filter

import (
	"encoding/json"
	"fmt"

	"github.com/fluent/fluent-operator/apis/fluentd/v1alpha1/plugins"
	"github.com/fluent/fluent-operator/apis/fluentd/v1alpha1/plugins/params"
)

// FilterCommon defines the common parameters for the filter plugin.
type FilterCommon struct {
	// The @id parameter specifies a unique name for the configuration.
	Id *string `json:"-"`
	// The @log_level parameter specifies the plugin-specific logging level
	LogLevel *string `json:"logLevel,omitempty"`
	// Which tag to be matched.
	Tag *string `json:"-"`
}

// Filter defines all available filter plugins and their parameters.
type Filter struct {
	// The common fields
	FilterCommon `json:",inline,omitempty"`

	// The filter_grep filter plugin
	Grep *Grep `json:"grep,omitempty"`
	// The filter_record_transformer filter plugin
	RecordTransformer *RecordTransformer `json:"recordTransformer,omitempty"`
	// The filter_parser filter plugin
	Parser *Parser `json:"parser,omitempty"`
	// The filter_stdout filter plugin
	Stdout *Stdout `json:"stdout,omitempty"`
}

// DeepCopyInto implements the DeepCopyInto interface.
func (in *Filter) DeepCopyInto(out *Filter) {
	bytes, err := json.Marshal(*in)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(bytes, &out)
	if err != nil {
		panic(err)
	}
}

func (f *Filter) Name() string {
	return "filter"
}

func (f *Filter) Params(loader plugins.SecretLoader) (*params.PluginStore, error) {
	ps := params.NewPluginStore(f.Name())

	if f.Id != nil {
		ps.InsertPairs("@id", fmt.Sprint(*f.Id))
	}

	if f.LogLevel != nil {
		ps.InsertPairs("@log_level", fmt.Sprint(*f.LogLevel))
	}

	if f.Tag != nil {
		ps.InsertPairs("tag", fmt.Sprint(*f.Tag))
	}

	if f.Grep != nil {
		ps.InsertType(string(params.GrepFilterType))
		return f.grepPlugin(ps, loader), nil
	}

	if f.RecordTransformer != nil {
		ps.InsertType(string(params.RecordTransformerFilterType))
		return f.recordTransformerPlugin(ps, loader), nil
	}

	if f.Parser != nil {
		ps.InsertType(string(params.ParserFilterType))
		return f.parserPlugin(ps, loader), nil
	}

	// 	// if nothing defined, supposed it is a filter_stdout plugin
	ps.InsertType(string(params.StdoutFilterType))
	return f.stdoutPlugin(ps, loader), nil
}

func (f *Filter) grepPlugin(parent *params.PluginStore, loader plugins.SecretLoader) *params.PluginStore {
	childs := make([]*params.PluginStore, 0)
	if len(f.Grep.Regexps) > 0 {
		for _, r := range f.Grep.Regexps {
			if r != nil && r.Key != nil && r.Pattern != nil {
				child, _ := r.Params(loader)
				childs = append(childs, child)
			}
		}
	}

	if len(f.Grep.Excludes) > 0 {
		for _, e := range f.Grep.Excludes {
			if e != nil && e.Key != nil && e.Pattern != nil {
				child, _ := e.Params(loader)
				childs = append(childs, child)
			}
		}
	}

	if len(f.Grep.Ands) > 0 {
		for _, e := range f.Grep.Ands {
			if e != nil && (e.Regexp != nil || e.Exclude != nil) {
				child := params.NewPluginStore("and")
				if e.Regexp != nil && e.Regexp.Key != nil && e.Regexp.Pattern != nil {
					subchild, _ := e.Regexp.Params(loader)
					child.InsertChilds(subchild)
				}
				if e.Exclude != nil && e.Exclude.Key != nil && e.Exclude.Pattern != nil {
					subchild, _ := e.Exclude.Params(loader)
					child.InsertChilds(subchild)
				}
				childs = append(childs, child)
			}
		}
	}

	if len(f.Grep.Ors) > 0 {
		for _, o := range f.Grep.Ors {
			if o != nil && (o.Regexp != nil || o.Exclude != nil) {
				child := params.NewPluginStore("or")
				if o.Regexp != nil && o.Regexp.Key != nil && o.Regexp.Pattern != nil {
					subchild, _ := o.Regexp.Params(loader)
					child.InsertChilds(subchild)
				}
				if o.Exclude != nil && o.Exclude.Key != nil && o.Exclude.Pattern != nil {
					subchild, _ := o.Exclude.Params(loader)
					child.InsertChilds(subchild)
				}
				childs = append(childs, child)
			}
		}
	}
	parent.InsertChilds(childs...)
	return parent
}

func (f *Filter) recordTransformerPlugin(parent *params.PluginStore, loader plugins.SecretLoader) *params.PluginStore {
	childs := make([]*params.PluginStore, 0)
	if f.RecordTransformer != nil {
		if len(f.RecordTransformer.Records) > 0 {
			child := params.NewPluginStore("record")
			for _, r := range f.RecordTransformer.Records {
				if r != nil && r.Key != nil && r.Value != nil {
					child.InsertPairs(fmt.Sprint(*r.Key), fmt.Sprint(*r.Value))
				}
			}
			childs = append(childs, child)
		}
		if f.RecordTransformer.EnableRuby != nil {
			parent.InsertPairs("enable_ruby", fmt.Sprint(*f.RecordTransformer.EnableRuby))
		}
		if f.RecordTransformer.AutoTypeCast != nil {
			parent.InsertPairs("renew_record", fmt.Sprint(*f.RecordTransformer.AutoTypeCast))
		}
		if f.RecordTransformer.RenewTimeKey != nil {
			parent.InsertPairs("renew_time_key", fmt.Sprint(*f.RecordTransformer.RenewTimeKey))
		}
		if f.RecordTransformer.KeepKeys != nil {
			parent.InsertPairs("keep_keys", fmt.Sprint(*f.RecordTransformer.KeepKeys))
		}
		if f.RecordTransformer.RemoveKeys != nil {
			parent.InsertPairs("remove_keys", fmt.Sprint(*f.RecordTransformer.RemoveKeys))
		}

	}

	if len(childs) > 0 {
		parent.InsertChilds(childs...)
	}
	return parent
}

func (f *Filter) parserPlugin(parent *params.PluginStore, loader plugins.SecretLoader) *params.PluginStore {
	var child *params.PluginStore
	if f.Parser.Parse != nil {
		parseModel := f.Parser.Parse
		child, _ = parseModel.Params(loader)
	}

	if f.Parser.KeyName != nil {
		parent.InsertPairs("key_name", fmt.Sprint(f.Parser.KeyName))
	}
	if f.Parser.ReserveTime != nil {
		parent.InsertPairs("reserve_time", fmt.Sprint(f.Parser.ReserveTime))
	}
	if f.Parser.ReserveData != nil {
		parent.InsertPairs("reserve_data", fmt.Sprint(f.Parser.ReserveData))
	}
	if f.Parser.RemoveKeyNameField != nil {
		parent.InsertPairs("remove_key_name_field", fmt.Sprint(f.Parser.RemoveKeyNameField))
	}
	if f.Parser.ReplaceInvalidSequence != nil {
		parent.InsertPairs("replace_invalid_sequence", fmt.Sprint(f.Parser.ReplaceInvalidSequence))
	}
	if f.Parser.InjectKeyPrefix != nil {
		parent.InsertPairs("inject_key_prefix", fmt.Sprint(f.Parser.InjectKeyPrefix))
	}
	if f.Parser.HashValueField != nil {
		parent.InsertPairs("hash_value_field", fmt.Sprint(f.Parser.HashValueField))
	}
	if f.Parser.EmitInvalidRecordToError != nil {
		parent.InsertPairs("emit_invalid_record_to_error", fmt.Sprint(f.Parser.EmitInvalidRecordToError))
	}

	if child != nil {
		parent.InsertChilds(child)
	}
	return parent
}

func (f *Filter) stdoutPlugin(parent *params.PluginStore, loader plugins.SecretLoader) *params.PluginStore {
	if f.Stdout == nil {
		return parent
	}
	childs := make([]*params.PluginStore, 0)
	if f.Stdout.Format != nil {
		formatModel := f.Stdout.Format
		child, _ := formatModel.Params(loader)
		childs = append(childs, child)
	}
	if f.Stdout.Inject != nil {
		injectModel := f.Stdout.Inject
		child, _ := injectModel.Params(loader)
		childs = append(childs, child)
	}
	parent.InsertChilds(childs...)
	return parent
}

var _ plugins.Plugin = &Filter{}
