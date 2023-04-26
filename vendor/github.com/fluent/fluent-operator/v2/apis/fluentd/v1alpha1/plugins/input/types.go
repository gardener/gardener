package input

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins/params"
)

// InputCommon defines the common parameters for input plugins
type InputCommon struct {
	// The @id parameter specifies a unique name for the configuration.
	Id *string `json:"id,omitempty"`
	// The @log_level parameter specifies the plugin-specific logging level
	LogLevel *string `json:"logLevel,omitempty"`
	// The @label parameter is to route the input events to <label> sections.
	Label *string `json:"label,omitempty"`
}

// Input defines all available input plugins and their parameters
type Input struct {
	InputCommon `json:",inline"`
	// in_forward plugin
	Forward *Forward `json:"forward,omitempty"`
	// in_http plugin
	Http *Http `json:"http,omitempty"`
}

// DeepCopyInto implements the DeepCopyInto interface.
func (in *Input) DeepCopyInto(out *Input) {
	bytes, err := json.Marshal(*in)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(bytes, &out)
	if err != nil {
		panic(err)
	}
}

func (i *Input) Name() string {
	return "source"
}

func (i *Input) Params(loader plugins.SecretLoader) (*params.PluginStore, error) {
	ps := params.NewPluginStore(i.Name())

	if i.Id != nil {
		ps.InsertPairs("@id", fmt.Sprint(*i.Id))
	}

	if i.LogLevel != nil {
		ps.InsertPairs("@log_level", fmt.Sprint(*i.LogLevel))
	}

	if i.Label != nil {
		ps.InsertPairs("@label", fmt.Sprint(*i.Label))
	}

	if i.Forward != nil {
		ps.InsertType(string(params.ForwardInputType))
		return i.forwardPlugin(ps, loader), nil
	}

	if i.Http != nil {
		ps.InsertType(string(params.HttpInputType))
		return i.httpPlugin(ps, loader), nil
	}

	return nil, errors.New("you must define an input plugin")
}

func (i *Input) forwardPlugin(parent *params.PluginStore, loader plugins.SecretLoader) *params.PluginStore {
	forwardModel := i.Forward
	childs := make([]*params.PluginStore, 0)
	if forwardModel.Transport != nil {
		child, _ := forwardModel.Transport.Params(loader)
		childs = append(childs, child)
	}

	if forwardModel.Security != nil {
		child, _ := forwardModel.Security.Params(loader)
		childs = append(childs, child)
	}

	if forwardModel.Client != nil {
		child, _ := forwardModel.Client.Params(loader)
		childs = append(childs, child)
	}

	if forwardModel.User != nil {
		if forwardModel.User.Username != nil && forwardModel.User.Password != nil {
			child, _ := forwardModel.User.Params(loader)
			childs = append(childs, child)
		}
	}

	parent.InsertChilds(childs...)

	if forwardModel.Port != nil {
		parent.InsertPairs("port", fmt.Sprint(*forwardModel.Port))
	}

	if forwardModel.Bind != nil {
		parent.InsertPairs("bind", fmt.Sprint(*forwardModel.Bind))
	}

	if forwardModel.Tag != nil {
		parent.InsertPairs("tag", fmt.Sprint(*forwardModel.Tag))
	}

	if forwardModel.AddTagPrefix != nil {
		parent.InsertPairs("add_tag_prefix", fmt.Sprint(*forwardModel.AddTagPrefix))
	}

	if forwardModel.LingerTimeout != nil {
		parent.InsertPairs("linger_timeout", fmt.Sprint(*forwardModel.LingerTimeout))
	}

	if forwardModel.ResolveHostname != nil {
		parent.InsertPairs("resolve_hostname", fmt.Sprint(*forwardModel.ResolveHostname))
	}

	if forwardModel.DenyKeepalive != nil {
		parent.InsertPairs("deny_keepalive", fmt.Sprint(*forwardModel.DenyKeepalive))
	}

	if forwardModel.SendKeepalivePacket != nil {
		parent.InsertPairs("send_keepalive_packet", fmt.Sprint(*forwardModel.SendKeepalivePacket))
	}

	if forwardModel.ChunkSizeLimit != nil {
		parent.InsertPairs("chunk_size_limit", fmt.Sprint(*forwardModel.ChunkSizeLimit))
	}

	if forwardModel.ChunkSizeWarnLimit != nil {
		parent.InsertPairs("chunk_size_warn_limit", fmt.Sprint(*forwardModel.ChunkSizeWarnLimit))
	}

	if forwardModel.SkipInvalidEvent != nil {
		parent.InsertPairs("skip_invalid_event", fmt.Sprint(*forwardModel.SkipInvalidEvent))
	}

	if forwardModel.SourceAddressKey != nil {
		parent.InsertPairs("source_address_key", fmt.Sprint(*forwardModel.SourceAddressKey))
	}

	return parent
}

func (i *Input) httpPlugin(parent *params.PluginStore, loader plugins.SecretLoader) *params.PluginStore {
	httpModel := i.Http
	childs := make([]*params.PluginStore, 0)

	if httpModel.Transport != nil {
		child, _ := httpModel.Transport.Params(loader)
		childs = append(childs, child)
	}

	if httpModel.Parse != nil {
		child, _ := httpModel.Parse.Params(loader)
		childs = append(childs, child)
	}

	parent.InsertChilds(childs...)

	if httpModel.Port != nil {
		parent.InsertPairs("port", fmt.Sprint(*httpModel.Port))
	}

	if httpModel.Bind != nil {
		parent.InsertPairs("bind", fmt.Sprint(*httpModel.Bind))
	}

	if httpModel.BodySizeLimit != nil {
		parent.InsertPairs("body_size_limit", fmt.Sprint(*httpModel.BodySizeLimit))
	}

	if httpModel.KeepLiveTimeout != nil {
		parent.InsertPairs("keepLive_timeout", fmt.Sprint(*httpModel.KeepLiveTimeout))
	}

	if httpModel.AddHttpHeaders != nil {
		parent.InsertPairs("add_http_headers", fmt.Sprint(*httpModel.AddHttpHeaders))
	}

	if httpModel.AddRemoteAddr != nil {
		parent.InsertPairs("add_remote_addr", fmt.Sprint(*httpModel.AddRemoteAddr))
	}

	if httpModel.CorsAllowOrigins != nil {
		parent.InsertPairs("cors_allow_origins", fmt.Sprint(*httpModel.CorsAllowOrigins))
	}

	if httpModel.CorsAllowCredentials != nil {
		parent.InsertPairs("cors_allow_credentials", fmt.Sprint(*httpModel.CorsAllowCredentials))
	}

	if httpModel.RespondsWithEmptyImg != nil {
		parent.InsertPairs("responds_with_empty_img", fmt.Sprint(*httpModel.RespondsWithEmptyImg))
	}

	return parent
}

var _ plugins.Plugin = &Input{}
