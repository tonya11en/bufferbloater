def makeSegment(rps, duration_sec):  
    print("- rps: {}\n  duration: {}ms".format(rps, duration_sec))

start = 4000
for i in range(1000):
    makeSegment(start + i, 100)
