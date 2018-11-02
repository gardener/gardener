# Important

To add this chart to another as dependency, execute

```bash
mkdir -p ./charts/PATH-TO-MY-CHART/charts
ln -sr ./charts/api-versions ./charts/PATH-TO-MY-CHART/charts/utils-api-versions

# for example

mkdir -p ./charts/seed-controlplane/charts/kube-apiserver/charts
ln -sr ./charts/utils-api-versions ./charts/seed-controlplane/charts/kube-apiserver/charts/utils-api-versions
```

Then check for broken links with

```
find -L charts -type l
```

or

```
make verify
```
