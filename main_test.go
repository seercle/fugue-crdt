package fugue

import (
	"testing"
)

func TestBasicInsertions(t *testing.T) {
	doc := newDoc()

	doc.localInsertOne(1, 0, "a")
	if doc.getContent() != "a" {
		t.Errorf("Expected content 'a', got '%s'", doc.getContent())
	}

	doc.localInsertOne(1, 1, "b")
	if doc.getContent() != "ab" {
		t.Errorf("Expected content 'ab', got '%s'", doc.getContent())
	}

	doc.localInsertOne(1, 0, "c")
	if doc.getContent() != "cab" {
		t.Errorf("Expected content 'cab', got '%s'", doc.getContent())
	}

	doc.localInsertOne(1, 0, "d")
	if doc.getContent() != "dcab" {
		t.Errorf("Expected content 'dcab', got '%s'", doc.getContent())
	}

	doc.localInsertOne(1, 1, "e")
	if doc.getContent() != "decab" {
		t.Errorf("Expected content 'decab', got '%s'", doc.getContent())
	}

	doc.localInsertOne(1, 2, "f")
	if doc.getContent() != "defcab" {
		t.Errorf("Expected content 'defcab', got '%s'", doc.getContent())
	}
}

func TestBasicDeletions(t *testing.T) {
	doc := newDoc()

	doc.localInsertOne(1, 0, "a")
	doc.localInsertOne(1, 1, "b")
	doc.localInsertOne(1, 2, "c")
	doc.localInsertOne(1, 3, "d")

	doc.localDelete(1, 2)
	if doc.getContent() != "ad" {
		t.Errorf("Expected content 'ad', got '%s'", doc.getContent())
	}

	doc.localDelete(0, 1)
	if doc.getContent() != "d" {
		t.Errorf("Expected content 'd', got '%s'", doc.getContent())
	}

	doc.localDelete(0, 1)
	if doc.getContent() != "" {
		t.Errorf("Expected content '', got '%s'", doc.getContent())
	}
}

func TestMergeBasic(t *testing.T) {
	doc1 := newDoc()
	doc2 := newDoc()

	doc1.localInsertOne(1, 0, "a")
	doc1.localInsertOne(1, 1, "b")

	doc2.localInsertOne(2, 0, "x")
	doc2.localInsertOne(2, 1, "y")

	err := doc1.mergeFrom(doc2)
	if err != nil {
		t.Errorf("Unexpected error during merge: %v", err)
	}

	expected := Content("abxy")
	if doc1.getContent() != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, doc1.getContent())
	}
}

func TestMergeWithConflicts(t *testing.T) {
	doc1 := newDoc()
	doc2 := newDoc()

	doc1.localInsertOne(1, 0, "a")
	doc1.localInsertOne(1, 1, "b")

	doc2.localInsertOne(2, 0, "b")
	doc2.localInsertOne(2, 1, "a")

	err := doc1.mergeFrom(doc2)
	if err != nil {
		t.Errorf("Unexpected error during merge: %v", err)
	}

	// The exact order may depend on the implementation, but both "ab" and "ba" should be present
	content := doc1.getContent()
	if content != "abba" && content != "baba" {
		t.Errorf("Unexpected content after merge: '%s'", content)
	}
}

func TestMergeWithDeletions(t *testing.T) {
	doc1 := newDoc()
	doc2 := newDoc()

	doc1.localInsertOne(1, 0, "a")
	doc1.localInsertOne(1, 1, "b")
	doc1.localInsertOne(1, 2, "c")

	doc2.localInsertOne(2, 0, "x")
	doc2.localInsertOne(2, 1, "y")
	doc2.localDelete(0, 1) // Delete "x"

	err := doc1.mergeFrom(doc2)
	if err != nil {
		t.Errorf("Unexpected error during merge: %v", err)
	}

	expected := "abcy"
	if doc1.getContent() != Content(expected) {
		t.Errorf("Expected content '%s', got '%s'", expected, doc1.getContent())
	}
}

func TestConcurrentEdits(t *testing.T) {
	doc1 := newDoc()
	doc2 := newDoc()

	doc1.localInsertOne(1, 0, "a")
	doc2.localInsertOne(2, 0, "b")

	err := doc1.mergeFrom(doc2)
	if err != nil {
		t.Errorf("Unexpected error during merge: %v", err)
	}

	err = doc2.mergeFrom(doc1)
	if err != nil {
		t.Errorf("Unexpected error during merge: %v", err)
	}

	content1 := doc1.getContent()
	content2 := doc2.getContent()

	if content1 != content2 {
		t.Errorf("Documents diverged after merge: doc1='%s', doc2='%s'", content1, content2)
	}

	if content1 != "ab" && content1 != "ba" {
		t.Errorf("Unexpected content after concurrent edits: '%s'", content1)
	}
}

func TestEdgeCases(t *testing.T) {
	doc := newDoc()
	err := doc.localInsertOne(1, -1, "x")
	if err == nil {
		t.Errorf("Expected error for negative position, got nil")
	}

	err = doc.localInsertOne(1, 10, "x")
	if err == nil {
		t.Errorf("Expected error for invalid position, got nil")
	}
}

func TestEmptyMerge(t *testing.T) {
	doc1 := newDoc()
	doc2 := newDoc()

	err := doc1.mergeFrom(doc2)
	if err != nil {
		t.Errorf("Unexpected error during merge: %v", err)
	}

	if doc1.getContent() != "" {
		t.Errorf("Expected empty content, got '%s'", doc1.getContent())
	}
}

func TestMergeWithOverlappingDeletions(t *testing.T) {
	doc1 := newDoc()
	doc2 := newDoc()

	doc1.localInsertOne(1, 0, "a")
	doc1.localInsertOne(1, 1, "b")
	doc1.localInsertOne(1, 2, "c")

	doc2.localInsertOne(2, 0, "x")
	doc2.localInsertOne(2, 1, "y")
	doc2.localDelete(0, 2) // Delete "xy"

	err := doc1.mergeFrom(doc2)
	if err != nil {
		t.Errorf("Unexpected error during merge: %v", err)
	}

	expected := "abc"
	if doc1.getContent() != Content(expected) {
		t.Errorf("Expected content '%s', got '%s'", expected, doc1.getContent())
	}
}

func TestMergeWithMultipleClients(t *testing.T) {
	doc1 := newDoc()
	doc2 := newDoc()
	doc3 := newDoc()

	doc1.localInsertOne(1, 0, "a")
	doc2.localInsertOne(2, 0, "b")
	doc3.localInsertOne(3, 0, "c")

	err := doc1.mergeFrom(doc2)
	if err != nil {
		t.Errorf("Unexpected error during merge: %v", err)
	}

	err = doc1.mergeFrom(doc3)
	if err != nil {
		t.Errorf("Unexpected error during merge: %v", err)
	}

	content := doc1.getContent()
	if content != "abc" && content != "acb" && content != "bac" && content != "bca" && content != "cab" && content != "cba" {
		t.Errorf("Unexpected content after merge: '%s'", content)
	}
}
