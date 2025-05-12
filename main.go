package main

import (
	"errors"
	"fmt"
	"slices"
)

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

// findItemFromId finds the item in the list that contains the id
//
// returns the item and the position of the id in the item
// returns an error if the item is not found
// returns -1 if the item is nil
func (doc *Doc) findItemFromId(id *Id) (*LinkedItem, int, error) {
	if id == nil {
		return nil, -1, nil
	}
	i := 0
	for linked_item := doc.content.head; linked_item != nil; linked_item = linked_item.next {
		if linked_item.item.id.client != id.client {
			// We can skip the item if the client is different
			i += linked_item.item.length
			continue
		}
		if linked_item.item.id.seq <= id.seq &&
			id.seq <= linked_item.item.id.seq+Seq(linked_item.item.length-1) {
			// The item's id is in the range of the linked_item
			return linked_item, i + int(id.seq-linked_item.item.id.seq), nil
		}
		// Skip to the next item
		i += linked_item.item.length
	}
	return nil, -1, ErrNotFound
}

// findItemAt finds the item at the given position, ignoring deleted items
//
// returns the item and the relative position of the item inside the returned linked item
// returns an error with the overflow if the position is out of bounds
//
// example: this function returns 'abc' as the item and position 1
// if we call findItemAt(1, ...) on document {'abc'}
func (doc *Doc) findItemAt(position int, stick_end bool) (*LinkedItem, int, *OutOfBoundErr) {
	if position < 0 {
		return nil, -1, &OutOfBoundErr{position}
	}
	for item := doc.content.head; item != nil; item = item.next {
		if stick_end && position == 0 {
			return item, position, nil
		} else if item.item.deleted {
			// We skip deleted items without counting them
			continue
		} else if item.item.length > position {
			// The item is contained in the linked_item
			return item, position, nil
		}
		position -= item.item.length // skip to the next item
	}
	return nil, -1, &OutOfBoundErr{position}
}

// localInsert inserts the content at the given position for the given client
//
// returns an error if the position is out of bounds
func (doc *Doc) localInsert(client Client, position int, content Content) error {
	if position < 0 {
		return fmt.Errorf("position must be greater than 0")
	}
	var origin_left *Id = nil
	var origin_right *Id = nil
	item, item_position, err := doc.findItemAt(position, true)
	if err != nil && err.overflow > 0 {
		// Overflow of 0 means that we are at the end of the document
		// We only allow insertions before or at the end of the document
		return fmt.Errorf("item not found: %w", err)
	}
	var seq Seq = 0
	if val, ok := doc.version[client]; ok {
		seq = val + 1
	}
	// Find the left and right origins
	if item == nil {
		if doc.content.tail != nil {
			// We insert at the end of the document
			origin_left = &Id{
				client: doc.content.tail.item.id.client,
				seq:    doc.content.tail.item.id.seq,
			}
			origin_left.seq += Seq(doc.content.tail.item.length - 1)
		}
		// We don't need to set the origin_right if we are at the end of the document
	} else {
		// We insert in the document
		origin_right = &Id{
			client: item.item.id.client,
			seq:    item.item.id.seq + Seq(item_position),
		}
		if item_position == 0 {
			// We insert at the beginning of the item
			if item.prev != nil {
				// We insert after some item
				prev_id := item.prev.item.id
				origin_left = &Id{
					client: prev_id.client,
					seq:    prev_id.seq + Seq(item.prev.item.length-1), // the right most item of the previous item
				}
			}
			// We don't need to set the origin_left if we are at the beginning of the document
		} else {
			// We insert in the middle of the item
			origin_left = &Id{
				client: item.item.id.client,
				seq:    item.item.id.seq + Seq(item_position-1),
			}
		}
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
		length:       content.length(),
	})
}

// localDelete deletes the content at the given position for the given length
//
// returns an error if the position is out of bounds
// returns an error if the length is negative or greater than the length of the document
func (doc *Doc) localDelete(position int, length int) error {
	if length <= 0 {
		return fmt.Errorf("length must be greater than 0")
	}
	item, item_position, err := doc.findItemAt(position, false)
	if err != nil {
		return fmt.Errorf("item not found: %w", err)
	}
	// If we start deleting in the middle of a non-deleted item, we need to split the item
	// The left part of the item will be kept
	// The right part of the item will be deleted
	if !item.item.deleted && item_position > 0 {
		_, right_split, err := doc.content.splitTwo(item, item_position)
		if err != nil {
			return fmt.Errorf("delete error: %w", err)
		}
		item = right_split
	}

	for length > 0 && item != nil {
		if !item.item.deleted {
			// We only care about the non-deleted items
			if length >= item.item.length {
				// We can delete the whole item
				item.item.deleted = true
				length -= item.item.length
				// See if we can merge the item with the previous item
				if item.canMergeLeft() {
					doc.content.mergeLeft(item)
					// Move to the previous item, so that we can do merging in both directions
					item = item.prev
				}
				// See if we can merge the item with the next item
				if item.canMergeRight() {
					doc.content.mergeRight(item)
				}
			} else {
				// We need to split the last item to delete a part of it
				left, _, err := doc.content.splitTwo(item, length)
				if err != nil {
					return fmt.Errorf("delete error: %w", err)
				}
				left.item.deleted = true
				//See if we can merge the left part of the split with the previous item
				if left.canMergeLeft() {
					doc.content.mergeLeft(left)
				}
				return nil
			}
		}
		// Move to the next item, we still have work to do
		item = item.next
	}
	// If length > 0, it means we ran out of items to delete
	if length > 0 {
		return errors.New("not enough items to delete")
	}

	return nil
}

// cropOutVersion crops the item to remove the part that is already in the version
//
// returns the cropped item and an error if the item is fully in the version
func cropOutVersion(item Item, version *Version) (Item, error) {
	if seq, ok := (*version)[item.id.client]; ok {
		if seq >= item.id.seq+Seq(item.length-1) {
			// The item is fully in the version
			return Item{}, errors.New("item is fully in the version")
		}
		if seq < item.id.seq {
			// The item is fully out of the version
			return item, nil
		}
		// The item is partially in the version
		crop := item
		crop.content = item.content[seq-item.id.seq+1:]
		crop.id.seq = seq + 1
		crop.origin_left = &Id{
			client: item.id.client,
			seq:    seq,
		}
		return crop, nil
	}
	return item, nil
}

// isInVersion checks if the id is in the version
//
// returns true if the id is nil or is in the version
func isInVersion(id *Id, version *Version) bool {
	if id == nil {
		return true
	}
	// An id is in the version if we have seen a greater id from the same client before
	if seq, ok := (*version)[id.client]; ok {
		return id.seq <= seq
	}
	return false
}

// canInsertNow checks if the item can be inserted in the document
//
// returns true if the item can be inserted
func (doc *Doc) canInsertNow(item Item) bool {
	// Check if the items related to the given item are in the version
	return !isInVersion(&item.id, &doc.version) &&
		(item.id.seq == 0 || isInVersion(&Id{
			client: item.id.client,
			seq:    item.id.seq - 1,
		}, &doc.version)) &&
		isInVersion(item.origin_left, &doc.version) &&
		isInVersion(item.origin_right, &doc.version)
}

// remoteInsert inserts the item in the document from the remote client
func (doc *Doc) remoteInsert(item Item) {
	doc.integrate(item)
}

// integrate integrates the item in the document
//
// returns an error if the item is malformed
func (doc *Doc) integrate(item Item) error {
	id := item.id
	if val, ok := doc.version[id.client]; (!ok && id.seq != 0) || (ok && id.seq != val+1) {
		// The item seq needs to be in order
		return errors.New("invalid Seq number")
	}
	// The version also increase with the length of the item
	doc.version[id.client] = id.seq + Seq(item.length-1)
	left_item, left_index, err := doc.findItemFromId(item.origin_left)
	if err != nil {
		return fmt.Errorf("origin_left not found: %w", err)
	}
	var dest_item *LinkedItem = doc.content.head
	var position = 0
	if left_item != nil {
		position = int(item.origin_left.seq - left_item.item.id.seq)
		dest_item = left_item
		// We will place the item after the left item
		position++
		if position > left_item.item.length-1 {
			// Go to the next item if we were already at the end of the left item
			dest_item = dest_item.next
			position = 0
		}
	}
	var right_item *LinkedItem = nil
	right_index := doc.content.count
	if item.origin_right != nil {
		right_item, right_index, err = doc.findItemFromId(item.origin_right)
		if err != nil {
			fmt.Println(item, item.origin_right)
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
		_, oleft_index, err := doc.findItemFromId(other.item.origin_left)
		oleft_index += int(position)
		if err != nil {
			return fmt.Errorf("origin_left not found: %w", err)
		}
		oright_index := doc.content.count
		if other.item.origin_right != nil {
			_, oright_index, err = doc.findItemFromId(other.item.origin_right)
			if err != nil {
				fmt.Println(other.item.origin_right)
				return fmt.Errorf("origin_right not found: %w", err)
			}
		}
		if oleft_index < left_index || (oleft_index == left_index && oright_index == right_index && item.id.client < other.item.id.client) {
			break
		}
		if oleft_index == left_index {
			scanning = oright_index < right_index
		}
		// Search at the beginning of every new item
		position = 0
	}
	if dest_item == nil {
		// We insert at the end of the list
		doc.content.insertAfter(doc.content.tail, item)
		if doc.content.tail.canMergeLeft() {
			// The new tail can be merged with the previous item
			doc.content.mergeLeft(doc.content.tail)
		}
		return nil
	}
	// We insert in the rest of the list
	_, middle, _, err := doc.content.insertAt(dest_item, int(position), item)
	if err != nil {
		return fmt.Errorf("error inserting item: %w", err)
	}
	if middle.canMergeLeft() {
		doc.content.mergeLeft(middle)
	}
	return nil
}

// equals checks if the two ids are equal
//
// returns true if the ids are equal
func (id1 *Id) equals(id2 *Id) bool {
	return id1 == id2 || (id1 != nil && id2 != nil && id1.client == id2.client && id1.seq == id2.seq)
}

// canMergeLeft checks if the item can be merged with the previous item
//
// returns true if the item can be merged
func (at *LinkedItem) canMergeLeft() bool {
	// We can merge if the item is the continuation of the previous item
	return at != nil && at.prev != nil && at.prev.item.deleted == at.item.deleted && // both items are deleted or not
		at.prev.item.origin_right.equals(at.item.origin_right) && // in case new item is placed at the left of a merged item
		at.prev.item.id.client == at.item.id.client && // if the item is from the same client
		at.prev.item.id.seq+Seq(at.prev.item.length) == at.item.id.seq
}

// canMergeRight checks if the item can be merged with the next item
//
// returns true if the item can be merged
func (at *LinkedItem) canMergeRight() bool {
	// We just use the next item to check if we can merge on the left
	return at != nil && at.next.canMergeLeft()
}

// contains checks if the item is contained in the other item
//
// returns true if the item is contained
//
// example: 'abc' contains 'a','ab','cd'
func (source Item) contains(item Item) bool {
	return source.id.client == item.id.client &&
		max(source.id.seq, item.id.seq) <= min(source.id.seq+Seq(source.length-1), item.id.seq+Seq(item.length-1))
}

// mergeFrom merges the content from the other document into this document
//
// returns an error if the merge fails
func (dest *Doc) mergeFrom(from *Doc) error {
	var missing []Item
	// We find the items that are in from but not in dest
	for linked_item := from.content.head; linked_item != nil; linked_item = linked_item.next {
		if cropped, err := cropOutVersion(linked_item.item, &dest.version); err == nil {
			missing = append(missing, cropped)
		}
	}
	remaining := len(missing)
	// Go through all the missing items and try to insert them in dest
	for remaining > 0 {
		changed := false
		for i := range len(missing) {
			item := missing[i]
			if !dest.canInsertNow(item) {
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

	// Now we need to delete the items that are deleted in from but not in dest
	for from_item := from.content.head; from_item != nil; from_item = from_item.next {
		if !from_item.item.deleted {
			// Skip deleted items
			continue
		}
		for dest_item := dest.content.head; dest_item != nil; dest_item = dest_item.next {
			if !dest_item.item.deleted && from_item.item.contains(dest_item.item) {
				// Split the item into three parts: before, the deleted part, and after
				left_split_count := max(int(from_item.item.id.seq-dest_item.item.id.seq), 0)
				left, middle_right, err1 := dest.content.splitTwo(dest_item, left_split_count)
				if err1 != nil {
					return fmt.Errorf("error splitting item: %w", err1)
				}
				if middle_right == nil {
					// No right split means we deleted the whole item
					left.item.deleted = true
					// Try to merge with the previous item
					if left.canMergeLeft() {
						dest.content.mergeLeft(left)
					}
					continue
				}
				deleted_item_count := dest_item.item.length - left_split_count + min(int(from_item.item.id.seq)+from_item.item.length-int(dest_item.item.id.seq)-dest_item.item.length, 0)
				middle, _, err2 := dest.content.splitTwo(middle_right, deleted_item_count)
				if err2 != nil {
					return fmt.Errorf("error splitting item: %w", err2)
				}
				middle.item.deleted = true
				if middle.canMergeLeft() {
					// We can merge the deleted part with the previous item
					dest.content.mergeLeft(middle)
					// Move to the previous item, so that we can do merging in both directions
					middle = middle.prev
				}
				if middle.canMergeRight() {
					// We can merge the deleted part with the next item
					dest.content.mergeRight(middle)
				}
			}
		}
	}
	return nil
}

// debugPrint prints the content of the document in a human readable format
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
}
