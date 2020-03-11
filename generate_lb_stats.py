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

fig, (ax1, ax2) = plt.subplots(2)

# We want to plot the request are, latency, and the moment timeouts happen.
# While we're at it, let's just adjust the timestamp to be relative to the
# simulation start.
rq_count = []
legend = []
#plt.title("Weighted Least Request LB", fontsize=20)
for i in range(9002, 9007):
    x, y = xy_from_csv("server." + str(i) + ".rq_count.csv")
    ax1.plot(x, y)
    legend.append("endpoint_" + str(i))

ax1.set_xlabel("time", fontsize=18)
ax1.set_ylabel("request count", fontsize=18)
ax1.legend(legend)


legend = []
for i in range(9002, 9007):
    x, y = xy_from_csv("server." + str(i) + ".queue.size.csv")
    ax2.plot(x, y)
    legend.append("endpoint_" + str(i))

ax2.set_xlabel("time", fontsize=18)
ax2.set_ylabel("queue size", fontsize=18)
ax2.legend(legend)

plt.show()
