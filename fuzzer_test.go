package main

import (
	"math/rand"
	"testing"
)

func RandomDoc(rng *rand.Rand, docs []*Doc) *Doc {
	return docs[rng.Intn(len(docs))]
}

func TestFuzzer(t *testing.T) {
	const trials int64 = 1
	chars := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	chars_len := len(chars)
	for i := range trials {
		rng := rand.New(rand.NewSource(i))
		docs := []*Doc{newDoc(), newDoc(), newDoc()}
		for j := 0; j < 100; j++ {
			for d := 0; d < len(docs); d++ {
				doc := RandomDoc(rng, docs)
				len := doc.content.length
				var weight float32 = 0.65
				if len > 100 {
					weight = 0.35
				}
				if doc.content.length == 0 || rng.Float32() < weight {
					//insert
					position := rng.Intn(len + 1)
					rune := chars[rng.Intn(chars_len)]
					client := Client(rng.Intn(3))
					doc.localInsertOne(client, position, Content(rune))
				} else {
					//delete
					position := rng.Intn(len + 1)
					length := rng.Intn(math.min(len - position + 1))
					doc.localDelete(position, length)
				}
			}
		}
		docs[0].debugPrint()
		docs[1].debugPrint()
		docs[2].debugPrint()
		// Pick 2 random documents to merge
		rand.Shuffle(len(docs), func(i, j int) { docs[i], docs[j] = docs[j], docs[i] })
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
			t.Errorf("Content mismatch after merge: doc1='%s', doc2='%s'", doc1.getContent(), doc2.getContent())
		}
	}
}
