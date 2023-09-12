# Developing Gardener Locally

[This document](../deployment/getting_started_locally.md) explains how to set up a KinD-based environment for developing Gardener locally.

For the best development experience you should especially check the [Developing Gardener](../deployment/getting_started_locally.md#developing-gardener) section.

In case you plan a debugging session please check the [Debugging Gardener](../deployment/getting_started_locally.md#debugging-gardener) section.

## Benefiting from Google Cloud Code

Google Cloud Code is a set of IDE plugins for popular IDEs that make it easier to create, deploy and integrate applications with Google Cloud. Supported environments include VSCode, the Jetbrains suite (IntelliJ, PyCharm, GoLand, 
WebStorm), and Cloud Shell Editor. Gardener environments increasingly leverage Skaffold for developer productivity, Cloud Code exposes it's power under a friendly user experience.

There are several benefits to this environment:

* Several containers are built in the lifecycle, each time they are rebuilt and restarted, the Delve debugger connection must be re-established from the IDE 
to the new container.
* Skaffold can be additionally configured to specify how changes to the source artifacts trigger rebuilds of the respective containers.
* Logs are consolidated into a master-detail user interface orientation that helps users rapidly navigate to, watch and act on the information they need

### Developer Lifecycle

There are two primary lifecycles that the developer will need to consider. The first is a cold boot of the system, the second is iterative changes and individual container restarts. This documentation is currently provided for users 
running MacOS with a Jetbrains IDE. Contributions are welcome for other platforms. 

As prerequisites to these instructions, the user should have a working deployment environment [as above](../deployment/getting_started_locally.md). They should confirm that they are able to create a stable environment using
the `make kind-up && make gardener-up` sequence.

#### Cold Booting

Before iterative container lifecycle development using this technique may begin, system inception must be performed:
* Any leftover environment from a previous inception must be destroyed. For this example, we choose `gardener-local`, but this name is properly determined by the next step.
* The environment must be (re)created. Using `make kind-up` will create a cluster in KinD with the name `gardener-local`. The [project `Makefile`](../../Makefile) can also generate other cluster configurations with KinD, these will have 
  other names that must be substituted as necessary.
* Once the baseline cluster is running, we start the additional Gardener components via Google Cloud Code.

These steps can be automated with the following "external tool" configurations in Jetbrains products. This configuration is clipped from `~/Library/Application Support/JetBrains/<release/>/tools/External Tools.xml`

```xml
<toolSet name="External Tools">
  <tool name="Kind Kubeconfig" showInMainMenu="false" showInEditor="false" showInProject="false" showInSearchPopup="false" disabled="false" useConsole="false" showConsoleOnStdOut="false" showConsoleOnStdErr="false" synchronizeAfterRun="true">
    <exec>
      <option name="COMMAND" value="/opt/homebrew/bin/kind" />
      <option name="PARAMETERS" value="export kubeconfig --name gardener-local" />
      <option name="WORKING_DIRECTORY" value="$GOPATH$/src/github.com/gardener/gardener" />
    </exec>
  </tool>
  <tool name="Delete cluster" showInMainMenu="false" showInEditor="false" showInProject="false" showInSearchPopup="false" disabled="false" useConsole="false" showConsoleOnStdOut="false" showConsoleOnStdErr="false" synchronizeAfterRun="true">
    <exec>
      <option name="COMMAND" value="/opt/homebrew/bin/kind" />
      <option name="PARAMETERS" value="delete cluster --name gardener-local" />
      <option name="WORKING_DIRECTORY" value="$GOPATH$/src/github.com/gardener/gardener" />
    </exec>
  </tool>
  <tool name="Create cluster" showInMainMenu="false" showInEditor="false" showInProject="false" showInSearchPopup="false" disabled="false" useConsole="true" showConsoleOnStdOut="false" showConsoleOnStdErr="false" synchronizeAfterRun="true">
    <exec>
      <option name="COMMAND" value="/opt/homebrew/bin/make" />
      <option name="PARAMETERS" value="kind-up" />
      <option name="WORKING_DIRECTORY" value="$GOPATH$/src/github.com/gardener/gardener" />
    </exec>
  </tool>
</toolSet>
```

Referencing these tools, a run configuration must be created that references these three tools. This configuration is found in `$(PROJECT_ROOT)/runConfigurations`:

```xml
<component name="ProjectRunConfigurationManager">
  <configuration default="false" name="Skaffold g/g" type="google-container-tools-skaffold-run-config" factoryName="google-container-tools-skaffold-run-config-dev" activateToolWindowBeforeRun="false" show_console_on_std_err="false" show_console_on_std_out="false">
    <option name="allowRunningInParallel" value="false" />
    <option name="buildEnvironment" />
    <option name="cleanupDeployments" value="true" />
    <option name="deployToCurrentContext" value="true" />
    <option name="deployToMinikube" value="false" />
    <option name="envVariables">
      <map>
        <entry key="ENABLE_HEALTH_CHECKS" value="admission,controller,scheduler,gardenlet,operator" />
        <entry key="SKAFFOLD_BUILD_CONCURRENCY" value="0" />
        <entry key="SKAFFOLD_CACHE_ARTIFACTS" value="false" />
        <entry key="SKAFFOLD_CLEANUP" value="false" />
        <entry key="SKAFFOLD_DEFAULT_REPO" value="localhost:5001" />
        <entry key="SKAFFOLD_LABEL" value="skaffold.dev/run-id=gardener-local" />
        <entry key="SKAFFOLD_PUSH" value="true" />
      </map>
    </option>
    <option name="imageRepositoryOverride" value="localhost:5001" />
    <option name="kubernetesContext" />
    <option name="mappings">
      <list />
    </option>
    <option name="moduleDeploymentType" value="DEPLOY_EVERYTHING" />
    <option name="projectPathOnTarget" />
    <option name="resourceDeletionTimeoutMins" value="2" />
    <option name="selectedOptions">
      <list />
    </option>
    <option name="skaffoldConfigurationFilePath" value="$PROJECT_DIR$/src/github.com/gardener/gardener/skaffold.yaml" />
    <option name="skaffoldModules">
      <list>
        <option value="etcd" />
        <option value="controlplane" />
        <option value="gardenlet" />
      </list>
    </option>
    <option name="skaffoldNamespace" />
    <option name="skaffoldProfile" />
    <option name="skaffoldWatchMode" value="ON_DEMAND" />
    <option name="statusCheck" value="true" />
    <option name="verbosity" value="INFO" />
    <method v="2">
      <option name="ToolBeforeRunTask" enabled="true" actionId="Tool_External Tools_Delete cluster" />
      <option name="ToolBeforeRunTask" enabled="true" actionId="Tool_External Tools_Create cluster" />
      <option name="ToolBeforeRunTask" enabled="true" actionId="Tool_External Tools_Kind Kubeconfig" />
    </method>
  </configuration>
</component>
```

Note the environment variables in `/component/configuration/option[@name='envVariables'>]`. These environment variables come from two places:
1. The `ENABLE_HEALTH_CHECKS` entry is described [here](../getting_started_locally.md#readiness-and-leader-election-checks). This starter configuration will help ensure that the Gardener stack fully boots without health checks 
   distracting from initial success.
2. The rest of the environment entries are copied and pasted from the output of the `Makefile` target. The appear as such when Skaffold is launched:
   ```text
   $ make gardener-up
   env | grep SKAFFOLD_ | tr '\n' ';'
   SKAFFOLD_BUILD_CONCURRENCY=0;SKAFFOLD_LABEL=skaffold.dev/run-id=gardener-local;SKAFFOLD_PUSH=true;SKAFFOLD_DEFAULT_REPO=localhost:5001;hack/tools/bin/skaffold run
   ```
 The last line of output are the variables configuring Skaffold and can be copied verbatim into the Cloud Code configuration in the environment variables section.

#### Iterative Development

At this point, your IDE will contain a "Run Configuration" that will accomplish the steps outlined above. The only thing left to do is run it! 

In the "Run" menu, select either "Run..." or "Debug...". In the selection window, select the name you gave to your Run Configuration in the previous section. 

Sequentially, this will execute the pre-build steps, which sequentially:
1. Deletes any existing KinD cluster
1. Creates a new one
1. Finally, extract the Kubeconfig for the _new_ cluster and set it to default
1. Execute either `skaffold run` or `skaffold debug` as requested, using the `SKAFFOLD_*` environment variables

You will want to tune some parameters as you gain some experience with the environment:
* "Watch Mode" defines either that container builds should happen on a manual rebuild basis or automatically. The default is manually.
* `ENABLE_HEALTH_CHECKS` is set to a conservative default. As defined [here](../getting_started_locally.md#readiness-and-leader-election-checks), containers that are paused in the debugger **_WILL BE RESTARTED_** if they are stopped in 
  the debugger too long. As documented, some developers will prefer this, most will want to see the system running and [experiment by breaking things](https://www.knowledgehut.com/blog/devops/chaos-engineering) to turn them off. 
