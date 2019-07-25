#! /usr/bin/python

import matplotlib.pyplot as plt
import csv, sys, os

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
qsize_x, qsize_y = xy_from_csv("server.queue.size.csv")
timeout_stamps, _ = xy_from_csv("client.rq.timeout.csv")
service_unavail_stamps, _ = xy_from_csv("client.rq.503.csv")

sim_start = min(rq_rate_x + rq_latency_x + timeout_stamps)
rq_rate_x = map(lambda x: (x - sim_start) / 1e9, rq_rate_x)
rq_latency_x = map(lambda x: (x - sim_start) / 1e9, rq_latency_x)
qsize_x = map(lambda x: (x - sim_start) / 1e9, qsize_x)
timeout_stamps = map(lambda x: (x - sim_start) / 1e9, timeout_stamps)
service_unavail_stamps = map(lambda x: (x - sim_start) / 1e9, service_unavail_stamps)

relative_sim_end = max(rq_rate_x + rq_latency_x + timeout_stamps + service_unavail_stamps + qsize_x)

fig, (ax1, ax2, ax3, ax4, ax5) = plt.subplots(5)

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
for stamp in timeout_stamps:
    ax3.axvline(x=stamp, color="red")

# Get 503 vertical lines.
ax4.set_ylabel("Request 503s")
ax4.set_xlabel('Time (s)')
ax4.set_xlim([0,relative_sim_end])
for v in service_unavail_stamps:
    ax4.axvline(x=v, color="green")

ax5.set_xlabel('Time (s)')
ax5.set_xlim([0,relative_sim_end])
ax5.set_ylabel('Service Queue Size')
ax5.plot(qsize_x,qsize_y, '-')
ax5.tick_params(axis='y', labelcolor="blue")

plt.legend()
plt.show()
