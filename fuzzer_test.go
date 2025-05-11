package main

import (
	"math"
	"math/rand"
	"testing"
)

func TestFuzzer(t *testing.T) {
	const trials int64 = 1000
	chars := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	chars_len := len(chars)
	for i := range trials {
		rng := rand.New(rand.NewSource(i))
		docs := []*Doc{newDoc(), newDoc(), newDoc()}
		for range 1000 {
			for range len(docs) {
				i := rng.Intn(len(docs))
				doc := docs[i]
				len := doc.content.length
				var weight float32 = 0.65
				if len > 10 {
					weight = 0.35
				}
				if doc.content.length == 0 || rng.Float32() < weight {
					//insert
					position := rng.Intn(len + 1)
					rune := chars[rng.Intn(chars_len)]
					//t.Logf("%d %d %s", i, position, string(rune))
					doc.localInsert(Client(i), position, Content(rune))
				} else {
					//delete
					position := rng.Intn(len + 1)
					length := rng.Intn(int(math.Min(float64(len-position+1), float64(3))))
					doc.localDelete(position, length)
				}
			}
		}
		//t.Log("---")
		// Pick 2 random documents to merge
		rng.Shuffle(len(docs), func(i, j int) { docs[i], docs[j] = docs[j], docs[i] })
		doc1 := docs[0]
		doc2 := docs[1]
		// Merge doc1 into doc2
		err := doc1.mergeFrom(doc2)
		if err != nil {
			t.Errorf("Unexpected error during merge: %v", err)
		}
		err = doc2.mergeFrom(doc1)
		if err != nil {
			t.Errorf("Unexpected error during merge: %v", err)
		}
		// Check if the merged content is consistent
		if doc1.getContent() != doc2.getContent() {
			t.Errorf("Trial %d after merge: doc1='%s', doc2='%s'", i, doc1.getContent(), doc2.getContent())
		}
		for j := range len(docs) {
			for item := docs[j].content.head; item != nil; item = item.next {
				if item.item.content == "" {
					t.Errorf("Trial %d: empty content in doc %d", i, j)
				}
			}
		}
	}
}
