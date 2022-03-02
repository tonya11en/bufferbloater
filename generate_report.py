#! /usr/bin/python

import matplotlib
import matplotlib.colors as mcolors
import matplotlib.pyplot as plt
import csv
import sys
import os
import numpy as np

matplotlib.rcParams.update({'font.size': 18})

# Stats dump interval so we can calculate rates.
dt = 0.5

if len(sys.argv) == 2:
    data_dir = sys.argv[1]
else:
    data_dir = "data"

if not os.path.exists(data_dir):
    print("No data directory provided or found.")
    sys.exit(1)

rq_latency_x = []
rq_latency_y = []

timeout_timestamps = []


def xy_from_csv(filename):
    x = []
    y = []
    path = data_dir + "/" + filename
    if os.path.exists(path):
        with open(path, 'r') as csvfile:
            plots = csv.reader(csvfile, delimiter=',')
            for row in plots:
                x.append(float(row[0]))
                y.append(float(row[1]))
    return x, y


colors = ["blue", "green", "red"]
fig, (ax1, ax3, ax2) = plt.subplots(3)
for i in range(1):
    # We want to plot the request are, latency, and the moment timeouts happen.
    # While we're at it, let's just adjust the timestamp to be relative to the
    # simulation start.
    in_rq_rate_x, in_rq_rate_y = xy_from_csv("client.rps.{}.csv".format(i))
    out_rq_rate_x, out_rq_rate_y = xy_from_csv(
        "client.rq.total.count.{}.csv".format(i))
    retry_rate_x, retry_rate_y = xy_from_csv(
        "client.rq.retry.count.{}.csv".format(i))
    rq_latency_x, rq_latency_y = xy_from_csv(
        "client.rq.latency.{}.csv".format(i))
    rq_timeout_x, rq_timeout_y = xy_from_csv(
        "client.rq.timeout.{}.csv".format(i))
    rq_503_x, rq_503_y = xy_from_csv("client.rq.503.{}.csv".format(i))
    success_stamps, _ = xy_from_csv("client.rq.success_hist.{}.csv".format(i))
    goodput_x, goodput_y = xy_from_csv(
        "client.rq.success.count.{}.csv".format(i))
    failures_x, failures_y = xy_from_csv(
        "client.rq.failure.count.{}.csv".format(i))
    expected_latency_x, expected_latency_y = xy_from_csv(
        "server.expected_latency.{}.csv".format(i))
    active_rq_x, active_rq_y = xy_from_csv("client.active_rq.{}.csv".format(i))

    high_client_x, high_client_y = xy_from_csv(
        "client.rq.high.count.{}.csv".format(i))
    default_client_x, default_client_y = xy_from_csv(
        "client.rq.default.count.{}.csv".format(i))
    low_client_x, low_client_y = xy_from_csv(
        "client.rq.low.count.{}.csv".format(i))

    high_admitted_x, high_admitted_y = xy_from_csv(
        "server.high.processed.success.{}.csv".format(i))
    default_admitted_x, default_admitted_y = xy_from_csv(
        "server.default.processed.success.{}.csv".format(i))
    low_admitted_x, low_admitted_y = xy_from_csv(
        "server.low.processed.success.{}.csv".format(i))

    high_throttled_x, high_throttled_y = xy_from_csv(
        "server.high.processed.throttled.{}.csv".format(i))
    default_throttled_x, default_throttled_y = xy_from_csv(
        "server.default.processed.throttled.{}.csv".format(i))
    low_throttled_x, low_throttled_y = xy_from_csv(
        "server.low.processed.throttled.{}.csv".format(i))

    xstart = min(low_admitted_x + default_admitted_x + high_admitted_x + in_rq_rate_x +
                 out_rq_rate_x + rq_latency_x + failures_x + goodput_x + rq_timeout_x + rq_503_x)
    xend = (max(low_admitted_x + default_admitted_x + high_admitted_x + in_rq_rate_x +
            out_rq_rate_x + rq_latency_x + failures_x + goodput_x + rq_timeout_x + rq_503_x) - xstart)/1e9

    def adjust(xs):
        return list(map(lambda x: (x - xstart)/1e9, xs))

    def adjust_dt_y(ys):
        return list(map(lambda x: x / dt, ys))

    high_admitted_y = adjust_dt_y(high_admitted_y)
    default_admitted_y = adjust_dt_y(default_admitted_y)
    low_admitted_y = adjust_dt_y(low_admitted_y)

    high_client_y = adjust_dt_y(high_client_y)
    default_client_y = adjust_dt_y(default_client_y)
    low_client_y = adjust_dt_y(low_client_y)

    ax1.set_ylabel("Admitted (stackplot)")
    ax1.set_xlabel('Time (s)')
    ax1.set_xlim([0, xend])
    ax1.stackplot(adjust(high_admitted_x), high_admitted_y, default_admitted_y[:len(high_admitted_x)], low_admitted_y[:len(high_admitted_x)],
                  colors=["orange", "blue", "gray"], labels=["high_pri", "default_pri", "low_pri"])
    ax1.legend()

    ax3.set_ylabel("Admitted (line plot)")
    ax3.set_xlabel('Time (s)')
    ax3.set_xlim([0, xend])
    ax3.plot(adjust(high_admitted_x), high_admitted_y,
             color="orange", label="high_pri")
    ax3.plot(adjust(default_admitted_x), default_admitted_y,
             color="blue", label="default_pri")
    ax3.plot(adjust(low_admitted_x), low_admitted_y,
             color="gray", label="low_pri")

    print(adjust(high_admitted_x))
    print(high_admitted_y)

    ax2.set_ylabel("Sent")
    ax2.set_xlabel('Time (s)')
    if len(high_client_y) > 0:
        ax2.plot(adjust(high_client_x), high_client_y,
                 color="orange", label="high_pri")
    if len(default_client_y) > 0:
        ax2.plot(adjust(default_client_x), default_client_y,
                 color="blue", label="default_pri")
    if len(low_client_y) > 0:
        ax2.plot(adjust(low_client_x), low_client_y,
                 color="gray", label="low_pri")
    ax2.set_xlim([0, xend])
    ax2.legend()


# plt.legend()
plt.show()
