{{- define "logging-dashboard" -}}
{
    "annotations": {
      "list": []
    },
    "editable": true,
    "gnetId": null,
    "graphTooltip": 0,
    "iteration": 1611650877419,
    "links": [],
    "panels": [
      {
        "cacheTimeout": null,
        "datasource": "prometheus",
        "description": "Shows the percentage of replicas that are up and running.",
        "fieldConfig": {
          "defaults": {
            "custom": {},
            "mappings": [],
            "thresholds": {
              "mode": "absolute",
              "steps": [
                {
                  "color": "red",
                  "value": null
                },
                {
                  "color": "#EAB839",
                  "value": 0.5
                },
                {
                  "color": "green",
                  "value": 1
                }
              ]
            },
            "unit": "percentunit"
          },
          "overrides": []
        },
        "gridPos": {
          "h": 7,
          "w": 4,
          "x": 0,
          "y": 0
        },
        "hideTimeOverride": false,
        "id": 1,
        "interval": null,
        "links": [],
        "maxDataPoints": 100,
        "options": {
          "colorMode": "value",
          "graphMode": "area",
          "justifyMode": "auto",
          "orientation": "auto",
          "reduceOptions": {
            "calcs": [
              "mean"
            ],
            "fields": "",
            "values": false
          },
          "textMode": "auto"
        },
        "pluginVersion": "7.2.1",
        "targets": [
          {
            "expr": "count(up{job=\"$component\"} == 1) / count(up{job=\"$component\"})",
            "format": "time_series",
            "instant": false,
            "interval": "",
            "intervalFactor": 1,
            "legendFormat": "",
            "refId": "A",
            "step": 600
          }
        ],
        "timeFrom": null,
        "timeShift": null,
        "title": "Replicas UP",
        "type": "stat"
      },
      {
        "aliasColors": {},
        "bars": false,
        "dashLength": 10,
        "dashes": false,
        "datasource": null,
        "description": "Shows the CPU usage and shows the requests and limits.",
        "fieldConfig": {
          "defaults": {
            "custom": {}
          },
          "overrides": []
        },
        "fill": 0,
        "fillGradient": 0,
        "gridPos": {
          "h": 7,
          "w": 10,
          "x": 4,
          "y": 0
        },
        "hiddenSeries": false,
        "id": 41,
        "legend": {
          "avg": false,
          "current": false,
          "max": false,
          "min": false,
          "show": true,
          "total": false,
          "values": false
        },
        "lines": true,
        "linewidth": 1,
        "links": [],
        "nullPointMode": "null",
        "options": {
          "dataLinks": []
        },
        "percentage": false,
        "pointradius": 2,
        "points": false,
        "renderer": "flot",
        "seriesOverrides": [],
        "spaceLength": 10,
        "stack": false,
        "steppedLine": false,
        "targets": [
          {
            "expr": "sum(rate(container_cpu_usage_seconds_total{pod=~\"$component-(.+)\"}[$__rate_interval])) by (pod)",
            "format": "time_series",
            "intervalFactor": 1,
            "legendFormat": "{{ "{{" }}pod}}-current",
            "refId": "A"
          },
          {
            "expr": "sum(kube_pod_container_resource_limits_cpu_cores{pod=~\"$component-(.+)\"}) by (pod)",
            "format": "time_series",
            "intervalFactor": 1,
            "legendFormat": "{{ "{{" }}pod}}-limits",
            "refId": "C"
          },
          {
            "expr": "sum(kube_pod_container_resource_requests_cpu_cores{pod=~\"$component-(.+)\"}) by (pod)",
            "format": "time_series",
            "intervalFactor": 1,
            "legendFormat": "{{ "{{" }}pod}}-requests",
            "refId": "B"
          }
        ],
        "thresholds": [],
        "timeFrom": null,
        "timeRegions": [],
        "timeShift": null,
        "title": "CPU usage",
        "tooltip": {
          "shared": true,
          "sort": 0,
          "value_type": "individual"
        },
        "type": "graph",
        "xaxis": {
          "buckets": null,
          "mode": "time",
          "name": null,
          "show": true,
          "values": []
        },
        "yaxes": [
          {
            "decimals": null,
            "format": "short",
            "label": null,
            "logBase": 1,
            "max": null,
            "min": "0",
            "show": true
          },
          {
            "format": "short",
            "label": null,
            "logBase": 1,
            "max": null,
            "min": null,
            "show": true
          }
        ],
        "yaxis": {
          "align": false,
          "alignLevel": null
        }
      },
      {
        "aliasColors": {},
        "bars": false,
        "dashLength": 10,
        "dashes": false,
        "datasource": null,
        "description": "Shows the memory usage.",
        "fieldConfig": {
          "defaults": {
            "custom": {}
          },
          "overrides": []
        },
        "fill": 0,
        "fillGradient": 0,
        "gridPos": {
          "h": 7,
          "w": 10,
          "x": 14,
          "y": 0
        },
        "hiddenSeries": false,
        "id": 24,
        "legend": {
          "avg": false,
          "current": false,
          "max": false,
          "min": false,
          "show": true,
          "total": false,
          "values": false
        },
        "lines": true,
        "linewidth": 1,
        "links": [],
        "nullPointMode": "null",
        "options": {
          "dataLinks": []
        },
        "percentage": false,
        "pointradius": 2,
        "points": false,
        "renderer": "flot",
        "seriesOverrides": [],
        "spaceLength": 10,
        "stack": false,
        "steppedLine": false,
        "targets": [
          {
            "expr": "sum(container_memory_working_set_bytes{pod=~\"$component-(.+)\"}) by (pod)",
            "format": "time_series",
            "intervalFactor": 1,
            "legendFormat": "{{ "{{" }}pod}}-current",
            "refId": "A"
          },
          {
            "expr": "sum(kube_pod_container_resource_limits_memory_bytes{pod=~\"$component-(.+)\"}) by (pod)",
            "format": "time_series",
            "intervalFactor": 1,
            "legendFormat": "{{ "{{" }}pod}}-limits",
            "refId": "B"
          },
          {
            "expr": "sum(kube_pod_container_resource_requests_memory_bytes{pod=~\"$component-(.+)\"}) by (pod)",
            "format": "time_series",
            "intervalFactor": 1,
            "legendFormat": "{{ "{{" }}pod}}-requests",
            "refId": "C"
          }
        ],
        "thresholds": [],
        "timeFrom": null,
        "timeRegions": [],
        "timeShift": null,
        "title": "Memory Usage",
        "tooltip": {
          "shared": true,
          "sort": 0,
          "value_type": "individual"
        },
        "type": "graph",
        "xaxis": {
          "buckets": null,
          "mode": "time",
          "name": null,
          "show": true,
          "values": []
        },
        "yaxes": [
          {
            "format": "bytes",
            "label": null,
            "logBase": 1,
            "max": null,
            "min": null,
            "show": true
          },
          {
            "format": "none",
            "label": null,
            "logBase": 1,
            "max": null,
            "min": null,
            "show": false
          }
        ],
        "yaxis": {
          "align": false,
          "alignLevel": null
        }
      },
      {
        "datasource": "loki",
        "fieldConfig": {
          "defaults": {
            "custom": {}
          },
          "overrides": []
        },
        "gridPos": {
          "h": 17,
          "w": 24,
          "x": 0,
          "y": 7
        },
        "id": 43,
        "interval": "",
        "options": {
          "showLabels": false,
          "showTime": true,
          "sortOrder": "Descending",
          "wrapLogMessage": false
        },
        "targets": [
          {
            "expr": "{pod_name=~\"$component-(.+)\", container_name=~\"$container\", severity=~\"$severity\"} |~ \"$search\"",
            "refId": "A"
          }
        ],
        "timeFrom": null,
        "timeShift": null,
        "title": "Logs",
        "type": "logs"
      }
    ],
    "refresh": "1m",
    "schemaVersion": 26,
    "style": "dark",
    "tags": [
      "controlplane",
      "seed",
      "logging"
    ],
    "templating": {
      "list": [
        {
          "allValue": "",
          "current": {
            "selected": true,
            "text": "{{ (index . 0).PodPrefix }}",
            "value": "{{ (index . 0).PodPrefix }}"
          },
          "hide": 0,
          "includeAll": false,
          "label": "Component",
          "multi": false,
          "name": "component",
          "options": [
            {
              "selected": true
            }{{ range $i, $c := . }},
            {
              "selected": false,
              "text": "{{ $c.PodPrefix }}",
              "value": "{{ $c.PodPrefix }}"
            }
              {{- end }}
          ],
          "query": "",
          "queryValue": "",
          "skipUrlSync": false,
          "type": "custom"
        },
        {
          "allValue": null,
          "current": {
            "selected": false,
            "text": "All",
            "value": "$__all"
          },
          "datasource": "prometheus",
          "definition": "label_values(kube_pod_container_info{type=~\"seed\", pod=~\"$component.+\"}, container)",
          "hide": 0,
          "includeAll": true,
          "label": "Container",
          "multi": false,
          "name": "container",
          "options": [],
          "query": "label_values(kube_pod_container_info{type=~\"seed\", pod=~\"$component.+\"}, container)",
          "refresh": 2,
          "regex": "",
          "skipUrlSync": false,
          "sort": 0,
          "tagValuesQuery": "",
          "tags": [],
          "tagsQuery": "",
          "type": "query",
          "useTags": false
        },
        {
          "allValue": ".+",
          "current": {
            "selected": true,
            "text": [
              "All"
            ],
            "value": [
              "$__all"
            ]
          },
          "hide": 0,
          "includeAll": true,
          "label": "Severity",
          "multi": true,
          "name": "severity",
          "options": [
            {
              "selected": true,
              "text": "All",
              "value": "$__all"
            },
            {
              "selected": false,
              "text": "INFO",
              "value": "INFO"
            },
            {
              "selected": false,
              "text": "WARN",
              "value": "WARN"
            },
            {
              "selected": false,
              "text": "ERR",
              "value": "ERR"
            },
            {
              "selected": false,
              "text": "DBG",
              "value": "DBG"
            },
            {
              "selected": false,
              "text": "NOTICE",
              "value": "NOTICE"
            },
            {
              "selected": false,
              "text": "FATAL",
              "value": "FATAL"
            }
          ],
          "query": "INFO,WARN,ERR,DBG,NOTICE,FATAL",
          "queryValue": "",
          "skipUrlSync": false,
          "type": "custom"
        },
        {
          "current": {
            "selected": false,
            "text": "",
            "value": ""
          },
          "hide": 0,
          "label": "Search",
          "name": "search",
          "options": [
            {
              "selected": true,
              "text": "",
              "value": ""
            }
          ],
          "query": "",
          "skipUrlSync": false,
          "type": "textbox"
        }
      ]
    },
    "time": {
      "from": "now-30m",
      "to": "now"
    },
    "timepicker": {
      "refresh_intervals": [
        "5s",
        "10s",
        "30s",
        "1m",
        "5m",
        "15m",
        "30m",
        "1h"
      ],
      "time_options": [
        "5m",
        "15m",
        "1h",
        "3h",
        "6h",
        "12h",
        "24h",
        "2d",
        "7d",
        "14d"
      ]
    },
    "timezone": "utc",
    "title": "Controlplane Logs Dashboard",
    "version": 1
  }
{{- end -}}
