package v1alpha1

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/fluent/fluent-operator/apis/fluentd/v1alpha1/plugins"
	"github.com/fluent/fluent-operator/apis/fluentd/v1alpha1/plugins/filter"
	"github.com/fluent/fluent-operator/apis/fluentd/v1alpha1/plugins/input"
	"github.com/fluent/fluent-operator/apis/fluentd/v1alpha1/plugins/output"
	"github.com/fluent/fluent-operator/apis/fluentd/v1alpha1/plugins/params"
	fluentdRouter "github.com/fluent/fluent-operator/pkg/fluentd/router"
)

// +kubebuilder:object:generate=false
type Renderer interface {
	GetNamespace() string
	GetName() string
	GetCfgId() string
	GetWatchedLabels() map[string]string
	GetWatchedNamespaces() []string
	GetWatchedContainers() []string
	GetWatchedHosts() []string
}

// +kubebuilder:object:generate=false
// Global pluginstores for the fluentd.
type PluginResources struct {
	InputPlugins         []params.PluginStore
	MainRouterPlugins    params.PluginStore
	LabelPluginResources []params.PluginStore
}

// +kubebuilder:object:generate=false
// All the filter/output selected to this cfg
type CfgResources struct {
	FilterPlugins []params.PluginStore
	OutputPlugins []params.PluginStore

	// the hash codes used to depulicate removel
	FiltersHashcodes map[string]bool
	OutputsHashcodes map[string]bool
}

// NewGlobalPluginResources represents a combined global fluentd resources
func NewGlobalPluginResources(globalId string) *PluginResources {
	globalMainRouter := fluentdRouter.NewGlobalRouter(globalId)
	return &PluginResources{
		InputPlugins:         make([]params.PluginStore, 0),
		MainRouterPlugins:    *globalMainRouter,
		LabelPluginResources: make([]params.PluginStore, 0),
	}
}

func NewCfgResources() *CfgResources {
	return &CfgResources{
		FilterPlugins: make([]params.PluginStore, 0),
		OutputPlugins: make([]params.PluginStore, 0),

		FiltersHashcodes: make(map[string]bool),
		OutputsHashcodes: make(map[string]bool),
	}
}

func (pgr *PluginResources) CombineGlobalInputsPlugins(sl plugins.SecretLoader, inputs []input.Input) []string {
	errs := make([]string, 0)
	for _, f := range inputs {
		ps, err := f.Params(sl)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		pgr.InputPlugins = append(pgr.InputPlugins, *ps)
	}
	return errs
}

func (pgr *PluginResources) BuildCfgRouter(cfg Renderer) (*fluentdRouter.Route, error) {
	matches := []*fluentdRouter.RouteMatch{
		{
			Labels:         cfg.GetWatchedLabels(),
			Namespaces:     cfg.GetWatchedNamespaces(),
			Hosts:          cfg.GetWatchedHosts(),
			ContainerNames: cfg.GetWatchedContainers(),
		},
	}

	cfgRoute, err := fluentdRouter.NewRoute(cfg.GetCfgId(), cfg.GetNamespace(), cfg.GetName(), matches)
	if err != nil {
		return nil, err
	}

	// Each fluentd config has its own route plugin
	routePluginStore, err := cfgRoute.NewRoutePlugin()
	if err != nil {
		return nil, err
	}

	pgr.MainRouterPlugins.InsertChilds(routePluginStore)

	return cfgRoute, nil
}

// PatchAndFilterClusterLevelResources will combine and patch all the cluster CRs that the fluentdconfig selected,
// convert the related filter/output pluginstores to the global pluginresources.
func (pgr *PluginResources) PatchAndFilterClusterLevelResources(sl plugins.SecretLoader, cfgId string,
	clusterfilters []ClusterFilter, clusteroutputs []ClusterOutput) (*CfgResources, []string) {
	// To store all filters/outputs plugins that this cfg selected
	cfgResources := NewCfgResources()

	errs := make([]string, 0)

	// List all filters matching the label selector.
	for _, i := range clusterfilters {
		// patch filterId
		err := cfgResources.filterForFilters(cfgId, "cluster", i.Name, "clusterfilter", sl, i.Spec.Filters)
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	// List all outputs matching the label selector.
	for _, i := range clusteroutputs {
		// patch outputId
		err := cfgResources.filterForOutputs(cfgId, "cluster", i.Name, "clusteroutput", sl, i.Spec.Outputs)
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	return cfgResources, errs
}

// PatchAndFilterNamespacedLevelResources will combine and patch all the cluster CRs that the fluentdconfig selected,
// convert the related filter/output pluginstores to the global pluginresources.
func (pgr *PluginResources) PatchAndFilterNamespacedLevelResources(sl plugins.SecretLoader, cfgId string,
	filters []Filter, outputs []Output) (*CfgResources, []string) {
	// To store all filters/outputs plugins that this cfg selected
	cfgResources := NewCfgResources()

	errs := make([]string, 0)

	// List all filters matching the label selector.
	for _, i := range filters {
		// patch filterId
		err := cfgResources.filterForFilters(cfgId, i.Namespace, i.Name, "filter", sl, i.Spec.Filters)
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	// List all outputs matching the label selector.
	for _, i := range outputs {
		// patch outputId
		err := cfgResources.filterForOutputs(cfgId, i.Namespace, i.Name, "output", sl, i.Spec.Outputs)
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	return cfgResources, errs
}

func (r *CfgResources) filterForFilters(cfgId, namespace, name, crdtype string,
	sl plugins.SecretLoader, filters []filter.Filter) error {
	for n, filter := range filters {
		filterId := fmt.Sprintf("%s::%s::%s::%s-%d", cfgId, namespace, crdtype, name, n)
		filter.FilterCommon.Id = &filterId
		filter.FilterCommon.Tag = &params.DefaultTag

		ps, err := filter.Params(sl)
		if err != nil {
			return err
		}

		hashcode := ps.Hash()
		if _, ok := r.FiltersHashcodes[hashcode]; ok {
			continue
		}

		r.FiltersHashcodes[hashcode] = true
		r.FilterPlugins = append(r.FilterPlugins, *ps)
	}

	return nil
}

func (r *CfgResources) filterForOutputs(cfgId, namespace, name, crdtype string,
	sl plugins.SecretLoader, outputs []output.Output) error {
	for n, output := range outputs {
		outputId := fmt.Sprintf("%s::%s::%s::%s-%d", cfgId, namespace, crdtype, name, n)
		output.OutputCommon.Id = &outputId
		output.OutputCommon.Tag = &params.DefaultTag

		ps, err := output.Params(sl)
		if err != nil {
			return err
		}

		hashcode := ps.Hash()
		if _, ok := r.OutputsHashcodes[hashcode]; ok {
			continue
		}

		r.OutputsHashcodes[hashcode] = true
		r.OutputPlugins = append(r.OutputPlugins, *ps)
	}

	return nil
}

// convert the cfg plugins to a label plugin, appends to the global label plugins
func (pgr *PluginResources) WithCfgResources(cfgRouteLabel string, r *CfgResources) error {
	if len(r.FilterPlugins) == 0 && len(r.OutputPlugins) == 0 {
		return errors.New("no filter plugins or output plugins matched")
	}

	cfgLabelPlugin := params.NewPluginStore("label")
	cfgLabelPlugin.InsertPairs("tag", cfgRouteLabel)

	// insert filter plugins of this fluentd config
	for _, filter := range r.FilterPlugins {
		childFilter := filter
		cfgLabelPlugin.InsertChilds(&childFilter)
	}

	// insert output plugins of this fluentd config
	for _, output := range r.OutputPlugins {
		childOutput := output
		cfgLabelPlugin.InsertChilds(&childOutput)
	}

	pgr.LabelPluginResources = append(pgr.LabelPluginResources, *cfgLabelPlugin)
	return nil
}

func (pgr *PluginResources) RenderMainConfig(enableMultiWorkers bool) (string, error) {
	if len(pgr.InputPlugins) == 0 && len(pgr.LabelPluginResources) == 0 {
		return "", fmt.Errorf("no plugins detect")
	}

	var buf bytes.Buffer

	// sort global inputs
	inputs := ByHashcode(pgr.InputPlugins)
	for _, pluginStore := range inputs {
		if enableMultiWorkers {
			pluginStore.SetIgnorePath()
		}
		buf.WriteString(pluginStore.String())
	}

	// sort main routers
	childRouters := ByRouteLabelsPointers(pgr.MainRouterPlugins.Childs)
	pgr.MainRouterPlugins.Childs = childRouters
	if enableMultiWorkers {
		pgr.MainRouterPlugins.SetIgnorePath()
	}
	buf.WriteString(pgr.MainRouterPlugins.String())

	// sort label plugins
	labelPlugins := ByRouteLabels(pgr.LabelPluginResources)
	for _, labelPlugin := range labelPlugins {
		if enableMultiWorkers {
			labelPlugin.SetIgnorePath()
		}
		buf.WriteString(labelPlugin.String())
	}

	return strings.TrimRight(buf.String(), "\n"), nil
}

// +kubebuilder:object:generate:=false
type ByHashcode []params.PluginStore

// +kubebuilder:object:generate:=false
type ByRouteLabelsPointers []*params.PluginStore

// +kubebuilder:object:generate:=false
type ByRouteLabels []params.PluginStore

func (a ByHashcode) Less(i, j int) bool            { return a[i].Hash() < a[j].Hash() }
func (a ByRouteLabelsPointers) Less(i, j int) bool { return a[i].RouteLabel() < a[j].RouteLabel() }
func (a ByRouteLabels) Less(i, j int) bool         { return a[i].RouteLabel() < a[j].RouteLabel() }

var _ Renderer = &FluentdConfig{}
var _ Renderer = &ClusterFluentdConfig{}
