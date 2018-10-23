# Important

To add this chart to another as dependency, execute

```bash
mkdir -p ./charts/PATH-TO-MY-CHART/charts
ln -sr ./charts/api-versions ./charts/PATH-TO-MY-CHART/charts/api-versions

# for example

mkdir -p ./charts/seed-controlplane/charts/kube-apiserver/charts
ln -sr ./charts/api-versions ./charts/seed-controlplane/charts/kube-apiserver/charts/api-versions
```

Then check for broken links with

```
find -L charts -type l
```

or

```
make verify
```
