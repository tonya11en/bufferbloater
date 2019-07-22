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
    with open(data_dir + "/" + filename,'r') as csvfile:
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
timeout_stamps, _ = xy_from_csv("client.rq.timeout.csv")

sim_start = min(rq_rate_x + rq_latency_x + timeout_stamps)
rq_rate_x = map(lambda x: (x - sim_start) / 1e9, rq_rate_x)
rq_latency_x = map(lambda x: (x - sim_start) / 1e9, rq_latency_x)
timeout_stamps = map(lambda x: (x - sim_start) / 1e9, timeout_stamps)

relative_sim_end = max(rq_rate_x + rq_latency_x + timeout_stamps)

fig, (ax1, ax2, ax3) = plt.subplots(3)

ax1.set_xlabel('Time (s)')
ax1.set_xlim([0,relative_sim_end])
ax1.set_ylabel('Request Latency')
ax1.plot(rq_latency_x,rq_latency_y)
ax1.tick_params(axis='y', labelcolor="black")
ax1.set_title("Request Latency")

ax2.set_xlabel('Time (s)')
ax2.set_xlim([0,relative_sim_end])
ax2.set_ylabel('RPS')
ax2.plot(rq_rate_x,rq_rate_y, '--')
ax2.tick_params(axis='y', labelcolor="blue")
ax2.set_title("Client RPS")

# Get timeout vertical lines.
ax3.set_title("Request Timeouts")
ax3.set_xlabel('Time (s)')
ax3.set_xlim([0,relative_sim_end])
for stamp in timeout_stamps:
    ax3.axvline(x=stamp, color="red")

plt.legend()
plt.show()
