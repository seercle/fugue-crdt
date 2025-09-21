# Fugue CRDT

Fugue CRDT is a collaborative text editing system built using Conflict-free Replicated Data Types (CRDTs). It ensures eventual consistency in distributed systems, allowing multiple clients to concurrently edit a shared document without conflicts.

## Features

- **CRDT-based Document Model**: Supports concurrent editing with eventual consistency.
- **Local and Remote Operations**: Insert and delete operations can be performed locally or merged from remote clients.
- **Fuzz Testing**: Includes a fuzzer to test the robustness of the CRDT implementation.
- **Benchmarking**: Provides tools to benchmark the performance of the CRDT under various editing traces.
- **Profiling Support**: Includes scripts for CPU and time profiling during benchmarks.

## Project Structure

- `main.go`: Contains the core CRDT implementation.
- `llist.go`: Implements the linked list data structure used for managing document content.
- `fuzzer_test.go`: Fuzz testing to ensure the robustness of the CRDT implementation.
- `benchmark_test.go`: Benchmarking tests to evaluate performance using editing traces.
- `benchmark.sh`: A script to automate benchmarking and profiling.

## Getting Started

### Prerequisites

- Go 1.18 or later
- Python 3 (optional, for time profiling)
- `go tool pprof` (for CPU profiling)

### Installation

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd fugue-crdt
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

### Running the Project

The main entry point is `main.go`. You can extend it to test specific CRDT operations.

### Running Tests

To run the tests, including the fuzzer and benchmarks:

   ```bash
   go test ./
   ```

### Benchmarking with `benchmark.sh`

The `benchmark.sh` script automates benchmarking and profiling:

   ```bash
   ./benchmark.sh
   ```

   Follow the prompts to choose whether to redo the benchmark, display the CPU profile, or display the time profile.
