# REUSE API compliance

We are using the [REUSE API](https://api.reuse.software/) to check and display the compliance with the REUSE guidelines.

To run the REUSE linter to ensure the compliance of the project in case you add new files to the project you need to 
run the following steps:

1. Install the `reuse` tool

```shell
# setup a virtual python environment in a location of your choice
virtualenv -p python3 dev
# active your python environment
source dev/bin/activate
# install the reuse tool cli
pip install reuse
```

2. Run the `resue` linter

```shell
reuse lint
```

3. Ensure that the codebase is compliant

```shell
Congratulations! Your project is compliant with version 3.0 of the REUSE Specification :-)
```

In case the codebase is not compliant you need to add a corresponding license header (`hack/LICENSE_BOILERPLATE.txt`)
to all non-compliant files. If that can not be achieved (e.g. because it is a JSON or YAML file), you can add the file
to the `.reuse/dep5` file.

Please also ensure, that all third party code is also reflected in the `.reuse/dep5` file with the correct license and 
authors.
