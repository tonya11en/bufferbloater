import matplotlib.pyplot as plt
import csv, sys

if not len(sys.argv) == 2:
    print("Provide the name of the csv file as the only argument")
    sys.exit(1)

x = []
y = []

with open(sys.argv[1],'r') as csvfile:
    plots = csv.reader(csvfile, delimiter=',')
    for row in plots:
        x.append(float(row[0]))
        y.append(float(row[1]))

plt.plot(x,y)
plt.xlabel('time')
plt.title(sys.argv[1])
plt.legend()
plt.show()
