#! /usr/bin/python

import matplotlib.pyplot as plt
import csv, sys, os
import numpy as np
from operator import add

SIMULATION_LENGTH_SECS = 30

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

# We want to plot the request are, latency, and the moment timeouts happen.
# While we're at it, let's just adjust the timestamp to be relative to the
# simulation start.
rq_rate_x1, rq_rate_y1 = xy_from_csv("client1.rps.csv")
rq_rate_x2, rq_rate_y2 = xy_from_csv("client2.rps.csv")

rq_latency_x1, rq_latency_y1 = xy_from_csv("client1.rq.latency.csv")
rq_latency_x2, rq_latency_y2 = xy_from_csv("client2.rq.latency.csv")

rq_sr_x1, rq_sr_y1 = xy_from_csv("client1.rq.success_rate.csv")
rq_sr_x2, rq_sr_y2 = xy_from_csv("client2.rq.success_rate.csv")

rq_goodput_x1, rq_goodput_y1 = xy_from_csv("client1.rq.success.count.csv")
rq_goodput_x2, rq_goodput_y2 = xy_from_csv("client2.rq.success.count.csv")

active_rq_x, active_rq_y = xy_from_csv("server.active_rq.csv")

qsize_x1, qsize_y1 = xy_from_csv("server.1.queued_rq.csv")
qsize_x2, qsize_y2 = xy_from_csv("server.2.queued_rq.csv")

qtimeout_x, qtimeout_y = xy_from_csv("server.queue_timeout.csv")

timeout_stamps1, _ = xy_from_csv("client1.rq.timeout.csv")
timeout_stamps2, _ = xy_from_csv("client2.rq.timeout.csv")

service_unavail_stamps1, _ = xy_from_csv("client1.rq.503.csv")
service_unavail_stamps2, _ = xy_from_csv("client2.rq.503.csv")

sim_start = min(rq_rate_x1 + rq_latency_x1 + timeout_stamps1 + rq_rate_x2 + rq_latency_x2 + timeout_stamps1 + timeout_stamps2)

# Normalize start times for x-vals.
def adjust_x_val_starts(vals):
    return list(map(lambda x: (x - sim_start) / 1e9, vals))

rq_rate_x1 = adjust_x_val_starts(rq_rate_x1)
rq_rate_x2 = adjust_x_val_starts(rq_rate_x2)
rq_latency_x1 = adjust_x_val_starts(rq_latency_x1)
rq_latency_x2 = adjust_x_val_starts(rq_latency_x2)
qtimeout_x = adjust_x_val_starts(qtimeout_x)
timeout_stamps1 = adjust_x_val_starts(timeout_stamps1)
timeout_stamps2 = adjust_x_val_starts(timeout_stamps2)
service_unavail_stamps1 = adjust_x_val_starts(service_unavail_stamps1)
service_unavail_stamps2 = adjust_x_val_starts(service_unavail_stamps2)
rq_sr_x1 = adjust_x_val_starts(rq_sr_x1)
rq_sr_x2 = adjust_x_val_starts(rq_sr_x2)
active_rq_x = adjust_x_val_starts(active_rq_x)
qsize_x1 = adjust_x_val_starts(qsize_x1)
qsize_x2 = adjust_x_val_starts(qsize_x2)
rq_goodput_x1 = adjust_x_val_starts(rq_goodput_x1)
rq_goodput_x2 = adjust_x_val_starts(rq_goodput_x2)

relative_sim_end = max(rq_rate_x1 + rq_rate_x2 +
                       rq_latency_x1 + rq_latency_x2 +
                       qtimeout_x + timeout_stamps1 + timeout_stamps2 +
                       service_unavail_stamps1 + service_unavail_stamps2 + 
                       rq_sr_x1 + rq_sr_x2 + active_rq_x + qsize_x1 + qsize_x2)
relative_sim_end = min(relative_sim_end, SIMULATION_LENGTH_SECS)

def adjust_x_val_ends(vals):
    return list(map(lambda x: x * (1.0 * relative_sim_end / vals[-1]), vals))

rq_sr_x1 = adjust_x_val_ends(rq_sr_x1)
rq_sr_x2 = adjust_x_val_ends(rq_sr_x2)
rq_goodput_x1 = adjust_x_val_ends(rq_goodput_x1)
rq_goodput_x2 = adjust_x_val_ends(rq_goodput_x2)
active_rq_x = adjust_x_val_ends(active_rq_x)
qsize_x1 = adjust_x_val_ends(qsize_x1)
qsize_x2 = adjust_x_val_ends(qsize_x2)

c1color = "orange"
c2color = "blue"

def show_latency(ax):
    ax.set_xlabel('Time (s)')
    ax.set_xlim([0,relative_sim_end])
    ax.set_ylabel('Request Latency')
    ax.scatter(rq_latency_x1,rq_latency_y1, color=c1color)
    ax.scatter(rq_latency_x2,rq_latency_y2, color=c2color)
    ax.tick_params(axis='y', labelcolor="black")

def show_rps(ax):
    ax.set_xlabel('Time (s)')
    ax.set_xlim([0,relative_sim_end])
    ax.set_ylabel('RPS')
    ax.plot(rq_rate_x1,rq_rate_y1, '--', color=c1color)
    ax.plot(rq_rate_x2,rq_rate_y2, '--', color=c2color)
    ax.tick_params(axis='y', labelcolor="black")

def show_timeouts(ax):
    # Get timeout vertical lines.
    ax.set_ylabel("Rq Timeouts")
    ax.set_xlabel('Time (s)')
    ax.set_xlim([0,relative_sim_end])
    if len(timeout_stamps1 + timeout_stamps2) > 0:
        ax.hist([timeout_stamps1, timeout_stamps2], bins=1000, density=True, histtype='bar',
                stacked=True, range=(0,relative_sim_end), color=[c1color, c2color])

def show_goodput(ax):
    ax.set_ylabel("Goodput")
    ax.set_xlabel('Time (s)')
    ax.set_xlim([0,relative_sim_end])
    ax.plot(rq_goodput_x1, rq_goodput_y1, color=c1color)
    ax.plot(rq_goodput_x2, rq_goodput_y2, color=c2color)

def show_throttling(ax):
    # Get 503 vertical lines.
    ax.set_ylabel("Rq Throttled (503)")
    ax.set_xlabel('Time (s)')
    ax.set_xlim([0,relative_sim_end])
    if len(service_unavail_stamps1 + service_unavail_stamps2) > 0:
        ax.hist([service_unavail_stamps1,service_unavail_stamps2], bins=1000, density=True, histtype='bar',
                stacked=True, range=(0,relative_sim_end), color=[c1color, c2color])

def show_qlen(ax):
    # TODO: stacked graph
    ax.set_xlabel('Time (s)')
    ax.set_xlim([0,relative_sim_end])
    ax.set_ylabel('Queue Length')
    ax.plot(qsize_x1, qsize_y1, '-', color=c1color)
    ax.plot(qsize_x2, qsize_y2, '-', color=c2color)
    ax.tick_params(axis='y', labelcolor="black")

def show_sr(ax):
    ax.set_xlabel('Time (s)')
    ax.set_xlim([0,relative_sim_end])
    ax.set_ylabel('Rq Success %')
    ax.plot(rq_sr_x1, rq_sr_y1, '-', color=c1color)
    ax.plot(rq_sr_x2, rq_sr_y2, '-', color=c2color)
    ax.tick_params(axis='y', labelcolor="black")

show = [
        show_latency,
        show_rps,
        show_timeouts,
        show_goodput,
        show_throttling,
        show_qlen,
        show_sr,
]

fig, axs = plt.subplots(len(show))
for idx in range(len(axs)):
    ax = axs[idx]
    fn = show[idx]
    fn(ax)

plt.legend()
plt.show()
