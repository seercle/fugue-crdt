package main

import (
	"testing"
	"time"
)

func BenchmarkLocalInsert(b *testing.B) {
	// Initialize the CRDT document
	doc := newDoc()
	client := Client(1)

	// Start measuring time
	start := time.Now()

	// Reset the timer to exclude setup time
	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		// Insert a single character at position i
		err := doc.localInsert(client, i, Content("A"))
		if err != nil {
			b.Fatalf("failed to insert: %v", err)
		}
	}

	// Stop measuring time
	elapsed := time.Since(start)

	// Print the total time spent
	b.ReportMetric(float64(elapsed.Milliseconds()), "ms_total")
}

func BenchmarkLocalDelete(b *testing.B) {
	// Initialize the CRDT document
	doc := newDoc()
	client := Client(1)

	// Prepopulate the document with some content
	for i := 0; i < b.N; i++ {
		err := doc.localInsert(client, i, Content("A"))
		if err != nil {
			b.Fatalf("failed to insert: %v", err)
		}
	}

	// Start measuring time
	start := time.Now()

	// Reset the timer to exclude setup time
	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		// Delete a single character from position 0
		err := doc.localDelete(0, 1)
		if err != nil {
			b.Fatalf("failed to delete: %v", err)
		}
	}

	// Stop measuring time
	elapsed := time.Since(start)

	// Print the total time spent
	b.ReportMetric(float64(elapsed.Milliseconds()), "ms_total")
}

func BenchmarkMerge(b *testing.B) {
	// Initialize two CRDT documents
	doc1 := newDoc()
	doc2 := newDoc()
	client1 := Client(1)
	client2 := Client(2)

	// Prepopulate both documents with some content
	for i := 0; i < 1000; i++ {
		err := doc1.localInsert(client1, i, Content("A"))
		if err != nil {
			b.Fatalf("failed to insert into doc1: %v", err)
		}
		err = doc2.localInsert(client2, i, Content("B"))
		if err != nil {
			b.Fatalf("failed to insert into doc2: %v", err)
		}
	}

	// Start measuring time
	start := time.Now()

	// Reset the timer to exclude setup time
	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		doc := doc1
		err := doc.mergeFrom(doc2)
		if err != nil {
			b.Fatalf("failed to merge: %v", err)
		}
	}

	// Stop measuring time
	elapsed := time.Since(start)

	// Print the total time spent
	b.ReportMetric(float64(elapsed.Milliseconds()), "ms_total")
}
