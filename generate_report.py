#! /usr/bin/python

import matplotlib.pyplot as plt
import csv, sys, os
import numpy as np

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
rq_rate_x, rq_rate_y = xy_from_csv("client.rps.csv")
rq_latency_x, rq_latency_y = xy_from_csv("client.rq.latency.csv")
rq_sr_x, rq_sr_y = xy_from_csv("client.rq.success_rate.csv")
qsize_x, qsize_y = xy_from_csv("server.queue.size.csv")
timeout_stamps, _ = xy_from_csv("client.rq.timeout.csv")
service_unavail_stamps, _ = xy_from_csv("client.rq.503.csv")

sim_start = min(rq_rate_x + rq_latency_x + timeout_stamps)
rq_rate_x = map(lambda x: (x - sim_start) / 1e9, rq_rate_x)
rq_latency_x = map(lambda x: (x - sim_start) / 1e9, rq_latency_x)
rq_sr_x = map(lambda x: (x - sim_start) / 1e9, rq_sr_x)
qsize_x = map(lambda x: (x - sim_start) / 1e9, qsize_x)
timeout_stamps = map(lambda x: (x - sim_start) / 1e9, timeout_stamps)
service_unavail_stamps = map(lambda x: (x - sim_start) / 1e9, service_unavail_stamps)

relative_sim_end = max(rq_rate_x + rq_latency_x + timeout_stamps + service_unavail_stamps + qsize_x)

fig, (ax1, ax2, ax3, ax4, ax5, ax6) = plt.subplots(6)

ax1.set_xlabel('Time (s)')
ax1.set_xlim([0,relative_sim_end])
ax1.set_ylabel('Request Latency')
ax1.scatter(rq_latency_x,rq_latency_y)
ax1.tick_params(axis='y', labelcolor="black")

ax2.set_xlabel('Time (s)')
ax2.set_xlim([0,relative_sim_end])
ax2.set_ylabel('RPS')
ax2.plot(rq_rate_x,rq_rate_y, '--')
ax2.tick_params(axis='y', labelcolor="blue")

# Get timeout vertical lines.
ax3.set_ylabel("Request Timeouts")
ax3.set_xlabel('Time (s)')
ax3.set_xlim([0,relative_sim_end])
if len(timeout_stamps) > 0:
    bins = np.arange(0, relative_sim_end, 0.01)
    hist, _ = np.histogram(timeout_stamps, bins)
    extent = [bins.min(), bins.max(), 0, 1]
    ax3.hist(timeout_stamps, bins=1000, range=(0,relative_sim_end), color="red")

# Get 503 vertical lines.
ax4.set_ylabel("Request 503s")
ax4.set_xlabel('Time (s)')
ax4.set_xlim([0,relative_sim_end])
if len(service_unavail_stamps) > 0:
    bins = np.arange(0, relative_sim_end, 0.01)
    hist, _ = np.histogram(service_unavail_stamps, bins)
    extent = [bins.min(), bins.max(), 0, 1]
    ax4.hist(service_unavail_stamps, bins=1000, range=(0,relative_sim_end), color="green")

ax5.set_xlabel('Time (s)')
ax5.set_xlim([0,relative_sim_end])
ax5.set_ylabel('Service Queue Size')
ax5.plot(qsize_x,qsize_y, '-')
ax5.tick_params(axis='y', labelcolor="blue")

ax6.set_xlabel('Time (s)')
ax6.set_xlim([0,relative_sim_end])
ax6.set_ylabel('Request Success Rate')
ax6.plot(rq_sr_x, rq_sr_y, '-')
ax6.tick_params(axis='y', labelcolor="blue")

plt.legend()
plt.show()
