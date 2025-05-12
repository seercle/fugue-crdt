package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

type Operation struct {
	Position int
	Type     bool
	String   string
}

func ParseTrace() ([]Operation, error) {
	file, err := os.Open("editing-trace.js")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var operations []Operation
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "];" { // End of the array
			break
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "],") {
			// Remove the brackets and trailing comma
			line = strings.TrimSuffix(strings.TrimPrefix(line, "["), "],")
			parts := strings.Split(line, ",")
			if len(parts) < 2 {
				return nil, fmt.Errorf("invalid edit format: %s", line)
			}

			// Parse the position
			position, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid position: %w", err)
			}

			// Parse the type
			editType, err := strconv.ParseBool(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid type: %w", err)
			}

			// Parse the character (if present)
			char := ""
			if len(parts) == 4 { // If we split a "," character by mistake
				parts[2] = parts[2] + "," + parts[3]
			}
			if !editType && len(parts) > 2 {
				char = strings.Trim(strings.TrimSpace(parts[2]), "\"")
			}

			operations = append(operations, Operation{
				Position: position,
				Type:     editType,
				String:   char,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read JavaScript file: %w", err)
	}
	return operations, nil
}
func BenchmarkTrace(b *testing.B) {
	operations, err := ParseTrace()
	if err != nil {
		b.Fatalf("Failed to parse trace: %v", err)
	}
	doc := newDoc()
	var totalTime time.Duration
	var averageTimes []float64
	for i, op := range operations {
		start := time.Now()
		if op.Type {
			err = doc.localDelete(op.Position, 1)
		} else {
			err = doc.localInsert(Client(0), op.Position, Content(op.String))
		}
		if err != nil {
			b.Fatalf("Failed to apply operation: %v", err)
		}

		elapsed := time.Since(start)
		totalTime += elapsed

		if i%2500 == 0 {
			fmt.Printf("%d,%.2f\n", i, float64(totalTime.Milliseconds())/2500)
			totalTime = 0
		}
		//averageTime := float64(totalTime.Microseconds()) / float64(i+1)
		//averageTimes = append(averageTimes, averageTime)
	}
	// Output the results for plotting
	fmt.Println("Total Changes,Average Time (microseconds)")
	for i, avgTime := range averageTimes {
		fmt.Printf("%d,%.2f\n", i+1, avgTime)
	}
}
