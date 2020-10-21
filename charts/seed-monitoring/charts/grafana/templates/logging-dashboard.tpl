{{- define "logging-dashboard" -}}
{
    "annotations": {
      "list": []
    },
    "editable": true,
    "gnetId": null,
    "graphTooltip": 0,
    "iteration": 1601888883889,
    "links": [],
    "panels": [
      {
        "cacheTimeout": null,
        "colorBackground": false,
        "colorValue": false,
        "colors": [
          "rgba(245, 54, 54, 0.9)",
          "rgba(237, 129, 40, 0.89)",
          "rgba(50, 172, 45, 0.97)"
        ],
        "datasource": "prometheus",
        "description": "Current uptime status.",
        "editable": false,
        "fieldConfig": {
          "defaults": {
            "custom": {}
          },
          "overrides": []
        },
        "format": "percent",
        "gauge": {
          "maxValue": 100,
          "minValue": 0,
          "show": true,
          "thresholdLabels": false,
          "thresholdMarkers": true
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
        "mappingType": 1,
        "mappingTypes": [
          {
            "name": "value to text",
            "value": 1
          },
          {
            "name": "range to text",
            "value": 2
          }
        ],
        "maxDataPoints": 100,
        "nullPointMode": "connected",
        "nullText": null,
        "postfix": "",
        "postfixFontSize": "50%",
        "prefix": "",
        "prefixFontSize": "50%",
        "rangeMaps": [
          {
            "from": "null",
            "text": "N/A",
            "to": "null"
          }
        ],
        "sparkline": {
          "fillColor": "rgba(31, 118, 189, 0.18)",
          "full": false,
          "lineColor": "rgb(31, 120, 193)",
          "show": false
        },
        "tableColumn": "",
        "targets": [
          {
            "expr": "(sum(up{job=\"{{ $.jobName }}\"} == 1) / sum(up{job=\"{{ $.jobName }}\"})) * 100",
            "format": "time_series",
            "intervalFactor": 2,
            "refId": "A",
            "step": 600
          }
        ],
        "thresholds": "50, 80",
        "title": "UP Time",
        "type": "singlestat",
        "valueFontSize": "80%",
        "valueMaps": [
          {
            "op": "=",
            "text": "N/A",
            "value": "null"
          }
        ],
        "valueName": "avg"
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
            "expr": "sum(rate(container_cpu_usage_seconds_total{pod=~\"{{ $.podPrefix }}-(.+)\"}[$__rate_interval])) by (pod)",
            "format": "time_series",
            "intervalFactor": 1,
            "legendFormat": "{{ "{{" }}pod}}-current",
            "refId": "A"
          },
          {
            "expr": "sum(kube_pod_container_resource_limits_cpu_cores{pod=~\"{{ $.podPrefix }}-(.+)\"}) by (pod)",
            "format": "time_series",
            "intervalFactor": 1,
            "legendFormat": "{{ "{{" }}pod}}-limits",
            "refId": "C"
          },
          {
            "expr": "sum(kube_pod_container_resource_requests_cpu_cores{pod=~\"{{ $.podPrefix }}-(.+)\"}) by (pod)",
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
            "expr": "sum(container_memory_working_set_bytes{pod=~\"{{ $.podPrefix }}-(.+)\"}) by (pod)",
            "format": "time_series",
            "intervalFactor": 1,
            "legendFormat": "{{ "{{" }}pod}}-current",
            "refId": "A"
          },
          {
            "expr": "sum(kube_pod_container_resource_limits_memory_bytes{pod=~\"{{ $.podPrefix }}-(.+)\"}) by (pod)",
            "format": "time_series",
            "intervalFactor": 1,
            "legendFormat": "{{ "{{" }}pod}}-limits",
            "refId": "B"
          },
          {
            "expr": "sum(kube_pod_container_resource_requests_memory_bytes{pod=~\"{{ $.podPrefix }}-(.+)\"}) by (pod)",
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
            "expr": "{pod_name=~\"{{ $.podPrefix }}-(.+)\", severity=~\"$severity\"} |~ \"$search\"",
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
    "schemaVersion": 25,
    "style": "dark",
    "tags": [
      "controlplane",
      "seed",
      "logging"
    ],
    "templating": {
      "list": [
        {
            "allValue": ".+",
            "current": {
            "selected": true,
            "tags": [],
            "text": "All",
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
    "timezone": "browser",
    "title": "{{ $.dashboardName }}",
    "uid": "{{ $.jobName }}",
    "version": 1
  }
{{- end -}}
