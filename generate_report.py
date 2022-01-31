#! /usr/bin/python

import matplotlib
import matplotlib.colors as mcolors
import matplotlib.pyplot as plt
import csv, sys, os
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
        with open(path,'r') as csvfile:
            plots = csv.reader(csvfile, delimiter=',')
            for row in plots:
                x.append(float(row[0]))
                y.append(float(row[1]))
    return x, y

colors = ["blue", "green", "red"]
fig, (ax2, ax1, ax3, ax5) = plt.subplots(4)
for i in range(1):
    # We want to plot the request are, latency, and the moment timeouts happen.
    # While we're at it, let's just adjust the timestamp to be relative to the
    # simulation start.
    in_rq_rate_x, in_rq_rate_y = xy_from_csv("client.rps.{}.csv".format(i))
    out_rq_rate_x, out_rq_rate_y = xy_from_csv("client.rq.total.count.{}.csv".format(i))
    retry_rate_x, retry_rate_y = xy_from_csv("client.rq.retry.count.{}.csv".format(i))
    rq_latency_x, rq_latency_y = xy_from_csv("client.rq.latency.{}.csv".format(i))
    rq_timeout_x, rq_timeout_y = xy_from_csv("client.rq.timeout.{}.csv".format(i))
    rq_503_x, rq_503_y = xy_from_csv("client.rq.503.{}.csv".format(i))
    success_stamps, _ = xy_from_csv("client.rq.success_hist.{}.csv".format(i))
    goodput_x, goodput_y = xy_from_csv("client.rq.success.count.{}.csv".format(i))
    failures_x, failures_y = xy_from_csv("client.rq.failure.count.{}.csv".format(i))
    expected_latency_x, expected_latency_y = xy_from_csv("server.expected_latency.{}.csv".format(i))
    active_rq_x, active_rq_y = xy_from_csv("client.active_rq.{}.csv".format(i))

    # Adjust for dt.
    goodput_y = list(map(lambda x: x / dt, goodput_y))

    xstart = min(in_rq_rate_x + out_rq_rate_x + rq_latency_x + failures_x + goodput_x + rq_timeout_x + rq_503_x)
    xend = (max(in_rq_rate_x + out_rq_rate_x + rq_latency_x + failures_x + goodput_x + rq_timeout_x + rq_503_x) - xstart)/1e9
    def adjust(xs):
        return list(map(lambda x: (x - xstart)/1e9, xs))

    ax1.set_xlabel('Time (s)')
    ax1.set_ylabel('Request Latency')
    #ax1.set_yscale('log') # log scale
    #ax1.scatter(adjust(rq_latency_x),rq_latency_y, color=colors[i], label="observed latency")
    ax1.hist2d(adjust(rq_latency_x),rq_latency_y, bins=128, norm=mcolors.LogNorm())
    ax1.axhline(y=1.0, color='r')

#    ax1.tick_params(axis='y', labelcolor="black")
    ax1.set_xlim([0,xend])
#    ax1.set_ylim([0,2.0])
#    ax1.legend()

    ax2.set_xlabel('Time (s)')
    ax2.set_ylabel('Offered Load')
    ax2.plot(adjust(in_rq_rate_x),in_rq_rate_y, label="inbound load")
    ax2.plot(adjust(retry_rate_x),retry_rate_y, color="red", label="retries")
    ax2.tick_params(axis='y', labelcolor="blue")
    ax2.set_xlim([0,xend])
    ax2.legend()

    ax3.set_ylabel("Goodput")
    ax3.set_xlabel('Time (s)')
    ax3.plot(adjust(goodput_x), goodput_y, color=colors[i], label="goodput")
    ax3.plot(adjust(failures_x), failures_y, color="red", label="failed rq")
    ax3.set_xlim([0,xend])


    ax5.set_xlabel('Time (s)')
    ax5.set_ylabel('Active Requests')
    ax5.plot(adjust(active_rq_x),active_rq_y)
    ax5.set_xlim([0,xend])
    

plt.legend()
plt.show()
