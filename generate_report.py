#! /usr/bin/python

import matplotlib.pyplot as plt
import csv, sys, os
import numpy as np
from operator import add

plt.rcParams.update({'font.size': 18})

SIMULATION_LENGTH_SECS = 30
NUM_CLIENTS = 3

if len(sys.argv) == 2:
    data_dir = sys.argv[1]
else:
    data_dir = "data"
    
if not os.path.exists(data_dir):
    print("No data directory provided or found.")
    sys.exit(1)

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
def get_per_client_xy(prefix, suffix):
    xr = []
    yr = []
    for i in range(NUM_CLIENTS):
        x, y = xy_from_csv(prefix + str(i) + suffix)
        xr.append(x)
        yr.append(y)
    return xr, yr

rq_rate_x, rq_rate_y = get_per_client_xy("client", ".rps.csv")

rq_latency_x, rq_latency_y = get_per_client_xy("client", ".rq.latency.csv")

rq_sr_x, rq_sr_y = get_per_client_xy("client", ".rq.success_rate.csv")

rq_goodput_x, rq_goodput_y = get_per_client_xy("client", ".rq.success.count.csv")

active_rq_x, active_rq_y = xy_from_csv("server.active_rq.csv")

qsize_x, qsize_y = get_per_client_xy("server.", ".queued_rq.csv")

qtimeout_x, qtimeout_y = xy_from_csv("server.queue_timeout.csv")

timeout_stamps, _ = get_per_client_xy("client", ".rq.timeout.csv")

service_unavail_x, service_unavail_y = get_per_client_xy("client", ".rq.503.csv")

def recursive_min(l):
    if not isinstance(l, list):
        return l
    newl = list(map(lambda x: recursive_min(x), l))
    if newl:
        return min(newl)
    else:
        return 1e20

sim_start = recursive_min(rq_rate_x + rq_latency_x + timeout_stamps)
print (sim_start)

# Normalize start times for x-vals.
def adjust_x_val_starts(vals):
    return list(map(lambda x: (x - sim_start) / 1e9, vals))

rq_rate_x = list(map(lambda x: adjust_x_val_starts(x), rq_rate_x))
rq_latency_x = list(map(lambda x: adjust_x_val_starts(x), rq_latency_x))
qtimeout_x = adjust_x_val_starts(qtimeout_x)
timeout_stamps = list(map(lambda x: adjust_x_val_starts(x), timeout_stamps))
service_unavail_x = list(map(lambda x: adjust_x_val_starts(x), service_unavail_x))
rq_sr_x = list(map(lambda x: adjust_x_val_starts(x), rq_sr_x))
active_rq_x = adjust_x_val_starts(active_rq_x)
qsize_x = list(map(lambda x: adjust_x_val_starts(x), qsize_x))
rq_goodput_x = list(map(lambda x: adjust_x_val_starts(x), rq_goodput_x))

def recursive_max(l):
    if not isinstance(l, list):
        return l
    newl = list(map(lambda x: recursive_max(x), l))
    if newl:
        return max(newl)
    else:
        return 0

relative_sim_end = recursive_max(rq_rate_x +
                       service_unavail_x +
                       rq_latency_x +
                       qtimeout_x + timeout_stamps +
                       service_unavail_x +
                       rq_sr_x + active_rq_x + qsize_x + 
                       [SIMULATION_LENGTH_SECS])

def adjust_x_val_ends(vals):
    return list(map(lambda x: x * (1.0 * relative_sim_end / vals[-1]), vals))

rq_sr_x = list(map(lambda x: adjust_x_val_ends(x), rq_sr_x))
rq_goodput_x = list(map(lambda x: adjust_x_val_ends(x), rq_goodput_x))
active_rq_x = adjust_x_val_ends(active_rq_x)
qsize_x = list(map(lambda x: adjust_x_val_ends(x), qsize_x))
service_unavail_x = list(map(lambda x: adjust_x_val_ends(x), service_unavail_x))

def show_latency(ax):
    ax.set_xlabel('Time (s)')
    ax.set_xlim([0,relative_sim_end])
    ax.set_ylabel('Request Latency')

    for i in range(NUM_CLIENTS):
        ax.scatter(rq_latency_x[i],rq_latency_y[i])

    ax.tick_params(axis='y', labelcolor="black")

def show_rps(ax):
    ax.set_xlabel('Time (s)')
    ax.set_xlim([0,relative_sim_end])
    ax.set_ylabel('RPS')

    for i in range(NUM_CLIENTS):
        ax.plot(rq_rate_x[i],rq_rate_y[i], '--')

    ax.tick_params(axis='y', labelcolor="black")

def show_timeouts(ax):
    # Get timeout vertical lines.
    ax.set_ylabel("Rq Timeouts")
    ax.set_xlabel('Time (s)')
    ax.set_xlim([0,relative_sim_end])
    if len(timeout_stamps) > 0:
        ax.hist(timeout_stamps, bins=1000, density=True, histtype='bar',
                stacked=True, range=(0,relative_sim_end))

def show_goodput(ax):
    ax.set_ylabel("Goodput")
    ax.set_xlabel('Time (s)')
    ax.set_xlim([0,relative_sim_end])

    for i in range(NUM_CLIENTS):
        ax.plot(rq_goodput_x[i], rq_goodput_y[i])

def show_throttling(ax):
    # Get 503 vertical lines.
    ax.set_ylabel("Rq Throttled (503)")
    ax.set_xlabel('Time (s)')
    ax.set_xlim([0,relative_sim_end])

    for i in range(NUM_CLIENTS):
        ax.plot(service_unavail_x[i], service_unavail_y[i], '-')


def show_qlen(ax):
    # TODO: stacked graph
    ax.set_xlabel('Time (s)')
    ax.set_xlim([0,relative_sim_end])
    ax.set_ylabel('Queue Length')

    for i in range(NUM_CLIENTS):
        ax.plot(qsize_x[i], qsize_y[i], '-')

    ax.tick_params(axis='y', labelcolor="black")

def show_sr(ax):
    ax.set_xlabel('Time (s)')
    ax.set_xlim([0,relative_sim_end])
    ax.set_ylabel('Rq Success %')

    for i in range(NUM_CLIENTS):
        ax.plot(rq_sr_x[i], rq_sr_y[i], '-')

    ax.tick_params(axis='y', labelcolor="black")

show = [
        show_latency,
        show_rps,
        show_timeouts,
        show_goodput,
        show_throttling,
#        show_qlen,
#        show_sr,
]

fig, axs = plt.subplots(len(show))
for idx in range(len(axs)):
    ax = axs[idx]
    fn = show[idx]
    fn(ax)

plt.legend()
plt.show()
