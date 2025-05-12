import matplotlib.pyplot as plt
import csv

# Read the benchmark results
x = []  # Total changes
y = []  # Average time
with open("out.csv", "r") as f:
    reader = csv.reader(f)
    next(reader)  # Skip the header
    for row in reader:
        x.append(int(row[0]))
        y.append(float(row[1]))

# Plot the results
plt.figure(figsize=(10, 6))
plt.plot(x, y, label="Average Time per Change")
plt.xlabel("Total Changes")
plt.ylabel("Average Time (Milliseconds)")
plt.title("CRDT Benchmark: Average Time per Change vs Total Changes")
plt.legend()
plt.grid(True)
plt.show()
