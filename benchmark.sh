#!/bin/bash

# Function to display a yes/no prompt
ask_user() {
    while true; do
        read -p "$1 (y/n): " choice
        case "$choice" in
            y|Y ) return 0;;  # Yes
            n|N ) return 1;;  # No
            * ) echo "Please answer y or n.";;
        esac
    done
}

# Ask if we want to redo the benchmark
redo_benchmark=false
if ask_user "Do you want to redo the benchmark?"; then
    redo_benchmark=true
fi

# Ask if we want to display the CPU profile
cpu_profile=false
if ask_user "Do you want to display the CPU profile?"; then
    cpu_profile=true
fi

# Ask if we want to display the time profile
time_profile=false
if ask_user "Do you want to display the time profile?"; then
    time_profile=true
fi

# If neither CPU nor time profile is selected and benchmark is not redone, exit
if ! $redo_benchmark && ! $cpu_profile && ! $time_profile; then
    echo "No actions selected. Exiting."
    exit 0
fi

# If redo_benchmark is true, run the benchmark command
if $redo_benchmark; then
    echo "Running benchmark command..."
    go test -bench BenchmarkTrace -cpuprofile=temp/cpu.prof -benchtime=1x

    # Remove the v0.test file if it exists
    if [ -f "v0.test" ]; then
        echo "Cleaning up temporary test binary (v0.test)..."
        rm -f v0.test
    fi
else
    echo "Skipping benchmark command as per user request."
fi

# If CPU profile is true, run `go tool pprof` and display the profile
if $cpu_profile; then
    echo "Displaying CPU profile..."
    (echo "web" | go tool pprof temp/cpu.prof) &
fi

# If time profile is true, run the Python script to display the time profile
if $time_profile; then
    echo "Displaying time profile..."
    if [ -f "benchmark/trace.py" ]; then
        python3 benchmark/trace.py &
    else
        echo "Error: Python script benchmark/trace.py not found."
    fi
fi
wait
