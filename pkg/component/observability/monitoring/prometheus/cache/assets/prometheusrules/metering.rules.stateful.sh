#!/bin/bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0


for NAME in cpu_usage cpu_requests memory_usage working_set_memory memory_requests network_transmit network_receive persistent_volume_usage; do
cat <<EOF
# - metering  :$NAME                   :sum_by_namespace                       :sum_over_time
# - metering  :$NAME                   :sum_by_namespace                       :avg_over_time
# - metering  :$NAME                   :sum_by_namespace                       :avg_over_time             :this_month
EOF
done

echo

for NAME in cpu_usage cpu_requests memory_usage working_set_memory memory_requests network_transmit network_receive persistent_volume_usage; do
cat <<EOF
  - record: metering:$NAME:sum_by_namespace:sum_over_time
    expr: |2
        metering:$NAME:sum_by_namespace
      +
        (
            last_over_time(metering:$NAME:sum_by_namespace:sum_over_time[10m])
          or
            metering:$NAME:sum_by_namespace * 0
        )

  - record: metering:$NAME:sum_by_namespace:avg_over_time
    expr: |2
          metering:$NAME:sum_by_namespace:sum_over_time * 60
        /
          (metering:memory_usage_seconds != 0)
      or
        metering:$NAME:sum_by_namespace:sum_over_time


  - record: metering:$NAME:sum_by_namespace:avg_over_time:this_month
    expr: |2
        metering:$NAME:sum_by_namespace:avg_over_time
      or
          last_over_time(metering:$NAME:sum_by_namespace:avg_over_time:this_month[10m])
        + on (year, month) group_left ()
          _year_month2

EOF
done
