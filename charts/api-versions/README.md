# Important

To add this chart to another as dependency, execute

```bash
ln -sr ./charts/api-versions ./charts/PATH-TO-MY-CHART/charts

# for example

ln -sr ./charts/api-versions ./charts/seed-controlplane/charts/kube-apiserver/charts
```

Then check for broken links with

```
find -L charts -type l
```

or

```
make verify
```