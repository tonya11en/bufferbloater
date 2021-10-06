#! /usr/bin/python

import matplotlib.pyplot as plt
import csv, sys, os
import numpy as np
import pandas as pd

# Stats dump interval so we can calculate rates.
dt = 0.25

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
fig, (ax1, ax2, ax3) = plt.subplots(3)
for i in range(3):
    # We want to plot the request are, latency, and the moment timeouts happen.
    # While we're at it, let's just adjust the timestamp to be relative to the
    # simulation start.
    rq_rate_x, rq_rate_y = xy_from_csv("client.rps.{}.csv".format(i))
    rq_latency_x, rq_latency_y = xy_from_csv("client.rq.latency.{}.csv".format(i))
    success_stamps, _ = xy_from_csv("client.rq.success_hist.{}.csv".format(i))
    goodput_x, goodput_y = xy_from_csv("client.rq.success.count.{}.csv".format(i))

    # Adjust for dt.
    goodput_y = list(map(lambda x: x / dt, goodput_y))

    xstart = min(rq_rate_x + rq_latency_x + goodput_x)
    xend = max(rq_rate_x + rq_latency_x + goodput_x)

    ax1.set_xlabel('Time (s)')
    ax1.set_ylabel('Request Latency')
    #ax1.set_yscale('log') # log scale
    ax1.plot(rq_latency_x,rq_latency_y, color=colors[i])
    ax1.tick_params(axis='y', labelcolor="black")
    ax1.set_xlim([xstart,xend])

    ax2.set_xlabel('Time (s)')
    ax2.set_ylabel('RPS')
    ax2.plot(rq_rate_x,rq_rate_y, '--')
    ax2.tick_params(axis='y', labelcolor="blue")
    ax2.set_xlim([xstart,xend])

    ax3.set_ylabel("Goodput")
    ax3.set_xlabel('Time (s)')
    ax3.plot(goodput_x, goodput_y, color=colors[i])
    ax3.set_xlim([xstart,xend])

plt.legend()
plt.show()
