# Important

To add this chart to another as dependency, execute

```bash
mkdir -p ./charts/PATH-TO-MY-CHART/charts
ln -sr ./charts/utils-templates ./charts/PATH-TO-MY-CHART/charts/utils-templates

# for example

mkdir -p ./charts/seed-controlplane/charts/kube-apiserver/charts
ln -sr ./charts/utils-templates ./charts/seed-controlplane/charts/kube-apiserver/charts/utils-templates
```

Then check for broken links with

```
find -L charts -type l
```

or

```
make verify
```
