package main

import (
	"errors"
	"fmt"
	"slices"
)

type Client int8
type Seq int32
type Content string

type Id struct {
	client Client // up to 127 clients
	seq    Seq    // up to 2^32 - 1 operations
}

type Item struct {
	id           Id
	origin_left  *Id
	origin_right *Id
	deleted      bool
	content      Content // Now multiple characters
}

type LinkedItem struct {
	item Item
	prev *LinkedItem
	next *LinkedItem
}

type LinkedList struct {
	length int
	head   *LinkedItem
	tail   *LinkedItem
}

type SkipList map[uint8]*LinkedItem

type Version map[Client]Seq

type Doc struct {
	content LinkedList
	version Version
}

func newDoc() *Doc {
	return &Doc{
		content: LinkedList{},
		version: make(Version),
	}
}

func (doc *Doc) getContent() Content {
	var content Content = ""
	for linked_item := doc.content.head; linked_item != nil; linked_item = linked_item.next {
		if !(linked_item.item.deleted) {
			content += linked_item.item.content
		}
	}
	return content
}

func (a *Id) idEq(b *Id) bool {
	return a == b || (a != nil && b != nil && a.client == b.client && a.seq == b.seq)
}

func (doc *Doc) findItem(id *Id) (*LinkedItem, int, error) {
	if id == nil {
		return nil, -1, nil
	}
	i := -1
	for linked_item := doc.content.head; linked_item != nil; linked_item = linked_item.next {
		i++
		if linked_item.item.id.idEq(id) {
			return linked_item, i, nil
		}
	}
	return nil, -1, errors.New("item not found")
}

func (doc *Doc) integrate(item Item) error {
	id := item.id
	var prev_seq Seq = -1
	if val, ok := doc.version[id.client]; ok {
		prev_seq = val
	}
	if id.seq != prev_seq+1 {
		return errors.New("invalid sequence number")
	}
	doc.version[id.client] = id.seq
	left_item, left_index, err := doc.findItem(item.origin_left)
	if err != nil {
		return fmt.Errorf("origin_left not found: %w", err)
	}
	var dest_item *LinkedItem = doc.content.head
	if left_item != nil {
		dest_item = left_item.next
	}
	var right_item *LinkedItem = nil
	right_index := doc.content.length
	if item.origin_right != nil {
		right_item, right_index, err = doc.findItem(item.origin_right)
		if err != nil {
			return fmt.Errorf("origin_right not found: %w", err)
		}
	}
	scanning := false
	for other := dest_item; ; other = other.next {
		if !scanning {
			dest_item = other
		}
		if other == nil || other == right_item {
			break
		}
		_, oleft_index, err := doc.findItem(other.item.origin_left)
		if err != nil {
			return fmt.Errorf("origin_left not found: %w", err)
		}
		oright_index := doc.content.length
		if other.item.origin_right != nil {
			_, oright_index, err = doc.findItem(other.item.origin_right)
			if err != nil {
				return fmt.Errorf("origin_right not found: %w", err)
			}
		}

		if oleft_index < left_index || (oleft_index == left_index && oright_index == right_index && item.id.client < other.item.id.client) {
			break
		}
		if oleft_index == left_index {
			scanning = oright_index < right_index
		}
	}
	// Put the item in the list before the destination item
	doc.content.length++
	new_item := &LinkedItem{item: item}
	if doc.content.head == nil { // empty list
		doc.content.head = new_item
		doc.content.tail = new_item
		return nil
	}
	if dest_item == nil { // insert at the end
		doc.content.tail.next = new_item
		new_item.prev = doc.content.tail
		doc.content.tail = new_item
		return nil
	}
	if dest_item.prev == nil { // insert at the beginning
		new_item.next = doc.content.head
		doc.content.head.prev = new_item
		doc.content.head = new_item
		return nil
	}
	// insert in the middle
	new_item.prev = dest_item.prev
	new_item.next = dest_item
	dest_item.prev.next = new_item
	dest_item.prev = new_item
	return nil
}

func (doc *Doc) findItemAt(position int, stickEnd bool) (*LinkedItem, error) {
	item := doc.content.head
	for ; item != nil; item = item.next {
		if stickEnd && position == 0 {
			return item, nil
		} else if item.item.deleted {
			continue
		} else if position == 0 {
			return item, nil
		}
		position--
	}
	if position == 0 {
		return item, nil
	}
	return nil, errors.New("item not found")
}

func (doc *Doc) localInsertOne(client Client, position int, content Content) error {
	item, err := doc.findItemAt(position, true)
	if err != nil {
		return fmt.Errorf("item not found: %w", err)
	}
	var origin_left *Id = nil
	var origin_right *Id = nil
	if item == nil && doc.content.tail != nil {
		origin_left = &doc.content.tail.item.id
	} else if item != nil && item.prev != nil {
		origin_left = &item.prev.item.id
	}
	if item != nil {
		origin_right = &item.item.id
	}
	var seq Seq = 0
	if val, ok := doc.version[client]; ok {
		seq = val + 1
	}
	return doc.integrate(Item{
		id: Id{
			client,
			seq,
		},
		origin_left:  origin_left,
		origin_right: origin_right,
		deleted:      false,
		content:      content,
	})
}

func isInVersion(id *Id, version *Version) bool {
	if id == nil {
		return true
	}
	if seq, ok := (*version)[id.client]; ok {
		return id.seq <= seq
	}
	return false
}

func (doc *Doc) canInsertNow(item *Item) bool {
	return !isInVersion(&item.id, &doc.version) &&
		(item.id.seq == 0 || isInVersion(&Id{
			client: item.id.client,
			seq:    item.id.seq - 1,
		}, &doc.version)) &&
		isInVersion(item.origin_left, &doc.version) &&
		isInVersion(item.origin_right, &doc.version)
}

func (doc *Doc) remoteInsert(item Item) {
	doc.integrate(item)
}

func (doc *Doc) localDelete(position int, length int) error {
	item, err := doc.findItemAt(position, false)
	if err != nil {
		return fmt.Errorf("item not found: %w", err)
	}
	// Traverse and mark items as deleted
	for length > 0 && item != nil {
		if !item.item.deleted {
			item.item.deleted = true
			length--
		}
		item = item.next
	}
	// If length > 0, it means we ran out of items to delete
	if length > 0 {
		return errors.New("not enough items to delete")
	}

	return nil
}

func (dest *Doc) mergeFrom(from *Doc) error {
	// Handle insertions
	var missing []Item
	for linked_item := from.content.head; linked_item != nil; linked_item = linked_item.next {
		if !isInVersion(&linked_item.item.id, &dest.version) {
			missing = append(missing, linked_item.item)
		}
	}
	remaining := len(missing)
	for remaining > 0 {
		changed := false
		for i := range len(missing) {
			item := missing[i]
			if !dest.canInsertNow(&item) {
				continue
			}
			dest.remoteInsert(item)
			missing = slices.Delete(missing, i, i)
			remaining--
			changed = true
		}
		if !changed {
			return errors.New("deadlock")
		}
	}
	// Handle deletions
	dest_item := dest.content.head
	for from_item := from.content.head; from_item != nil; from_item = from_item.next {
		for dest_item != nil && !from_item.item.id.idEq(&dest_item.item.id) {
			dest_item = dest_item.next
		}
		if dest_item == nil {
			break
		}
		if from_item.item.deleted {
			dest_item.item.deleted = true
		}
		dest_item = dest_item.next
	}
	return nil
}

func (doc *Doc) debugPrint() {
	for item := doc.content.head; item != nil; item = item.next {
		fmt.Printf("Content: '%s' ID: {client: %d, seq: %d} Origins: left=%v, right=%v Deleted=%t\n",
			item.item.content,
			item.item.id.client,
			item.item.id.seq,
			item.item.origin_left,
			item.item.origin_right,
			item.item.deleted)
	}
	fmt.Println("---")
}

func main() {
	doc := newDoc()
	doc2 := newDoc()
	doc.localInsertOne(1, 0, "H")
	doc.localInsertOne(1, 1, "e")
	doc2.mergeFrom(doc)

	doc.localDelete(1, 1)
	doc2.localInsertOne(2, 2, "l")
	doc.mergeFrom(doc2)
	doc.debugPrint()
}
