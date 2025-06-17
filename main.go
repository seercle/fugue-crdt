package main

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
)

type Version map[Client]Seq

type Doc struct {
	content LinkedList
	version Version
	cache   map[Client][]*LinkedItem
}

// findInCache finds the item in the cache for the given client
//
// returns the index of the item in the cache
// returns an error if the item is not found
func (doc *Doc) findInCache(id Id) (int, error) {
	cache := doc.cache[id.client]
	if cache == nil {
		return -1, fmt.Errorf("cache not found for client %d", id.client)
	}
	len := len(cache)
	if len == 0 {
		return -1, fmt.Errorf("cache is empty for client %d", id.client)
	}
	left := 0
	right := len - 1
	mid := cache[right]
	mid_seq := mid.item.id.seq
	if mid_seq == id.seq {
		return right, nil
	}
	fmt.Println(id.client, id.seq, mid_seq, mid.item.length, right)
	mid_index := int(math.Floor((float64(id.seq) / (float64(int(mid_seq) + mid.item.length - 1))) * float64(right)))
	for left <= right {
		mid = cache[mid_index]
		mid_seq = mid.item.id.seq
		if mid_seq <= id.seq {
			if id.seq < mid_seq+Seq(mid.item.length) {
				return mid_index, nil
			}
			left = mid_index + 1
		} else {
			right = mid_index - 1
		}
		mid_index = int(math.Floor(float64(left+right)) / 2)
	}
	return -1, fmt.Errorf("item not found in cache for client %d", id.client)
}

func (doc *Doc) findCleanInCache(id Id) (int, error) {
	index, err := doc.findInCache(id)
	if err != nil {
		return -1, err
	}
	item := doc.cache[id.client][index]
	if item.item.id.seq < id.seq {
		doc.cache[id.client] = append(doc.cache[id.client], item)
	}

	//structs.splice(index + 1, 0, splitItem(transaction, struct, clock - struct.id.clock))
	return index + 1, nil

}

// addToCache adds the item to the cache for the given client
//
// returns an error if the item does not follow the previous item
func (doc *Doc) addToCache(item *LinkedItem) error {
	if doc.cache[item.item.id.client] == nil {
		doc.cache[item.item.id.client] = make([]*LinkedItem, 0)
	} else {
		// Check if the item is already in the cache
		len := len(doc.cache[item.item.id.client])
		last := doc.cache[item.item.id.client][len-1]
		if last.item.id.seq+Seq(last.item.length) != item.item.id.seq {
			return fmt.Errorf("items are not in order in cache %d", item.item.id.client)
		}
	}
	doc.cache[item.item.id.client] = append(doc.cache[item.item.id.client], item)
	return nil
}

// getFromCache gets the item from the cache for the given client
//
// returns the item and an error if the item is not found
func (doc *Doc) getFromCache(id Id) (*LinkedItem, error) {
	index, err := doc.findInCache(id)
	if err != nil {
		return nil, err
	}
	if index < 0 {
		return nil, fmt.Errorf("returned index is negative for client %d", id.client)
	}
	if index >= len(doc.cache[id.client]) {
		return nil, fmt.Errorf("returned index is greater than cache length for client %d", id.client)
	}
	return doc.cache[id.client][index], nil
}

func newDoc() *Doc {
	return &Doc{
		content: LinkedList{},
		version: make(Version),
		cache:   make(map[Client][]*LinkedItem),
	}
}

func (doc *Doc) getContent() Content {
	var content Content = ""
	for linked_item := doc.content.head; linked_item != nil; linked_item = linked_item.next {
		if !(linked_item.item.deleted) {
			escaped := strings.ReplaceAll(string(linked_item.item.content), "\n", "\\n")
			content += Content(escaped)
		}
	}
	return content
}

// findItemFromId finds the item in the list that contains the id
//
// returns the item and the position of the id in the item
// returns an error if the item is not found
// returns 0 if the item is nil
func (doc *Doc) findItemFromId(id *Id) (*LinkedItem, int, error) {
	if id == nil {
		return nil, 0, nil
	}
	for linked_item := doc.content.head; linked_item != nil; linked_item = linked_item.next {
		if linked_item.item.id.client != id.client {
			continue
		}
		if linked_item.item.id.seq <= id.seq &&
			id.seq <= linked_item.item.id.seq+Seq(linked_item.item.length-1) {
			// The item's id is in the range of the linked_item
			return linked_item, int(id.seq - linked_item.item.id.seq), nil
		}
	}
	return nil, 0, ErrNotFound
}

// findItemAt finds the item at the given document position, ignoring deleted items
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
	/*if doc.last_item != nil && doc.last_position <= position {
	// Start searching from the last item
	position -= doc.last_position
	for item := doc.last_item; item != nil; item = item.next {
		if item.item.deleted {
			// We skip deleted items without counting them
			continue
		} else if item.item.length > position {
			// The item is contained in the linked_item
			doc.last_item = item
			doc.last_position = position
			return item, position, nil
		}
		position -= item.item.length // skip to the next item
	}
	return nil, -1, &OutOfBoundErr{position}
	}*/
	for item := doc.content.head; item != nil; item = item.next {
		if stick_end && position == 0 {
			return item, 0, nil
		} else if item.item.deleted {
			// We skip deleted items without counting them
			continue
		} else if item.item.length > position {
			// The item is contained in the linked_item
			/*doc.last_item = item
			doc.last_position = position*/
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
		_, right_split, err := doc.splitTwo(item, item_position)
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
				doc.content.count -= item.item.length
				// See if we can merge the item with the previous item
				if item.canMergeLeft() {
					doc.mergeLeft(item)
					// Move to the previous item, so that we can do merging in both directions
					item = item.prev
				}
				// See if we can merge the item with the next item
				if item.canMergeRight() {
					doc.mergeRight(item)
				}
			} else {
				// We need to split the last item to delete a part of it
				left, _, err := doc.splitTwo(item, length)
				if err != nil {
					return fmt.Errorf("delete error: %w", err)
				}
				left.item.deleted = true
				doc.content.count -= left.item.length
				//doc.content.count -= left.item.length
				//See if we can merge the left part of the split with the previous item
				if left.canMergeLeft() {
					doc.mergeLeft(left)
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

// compare compares two integers and returns the order between them
//
// returns 0 if they are equal
// returns -1 if a is less than b
// returns 1 if a is greater than b
func compare(a int, b int) int {
	if a == b {
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}

// order compares two items and returns the order between them in the list
//
// returns 0 if they are equal
// returns -1 if a is before b
// returns 1 if a is after b
func order(a *LinkedItem, a_position int, b *LinkedItem, b_position int) int {
	if a == b {
		return compare(a_position, b_position)
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	after_a := a
	after_b := b
	for {
		after_a = after_a.next
		after_b = after_b.next
		if after_a == b || after_b == nil {
			return -1
		}
		if after_b == a || after_a == nil {
			return 1
		}
	}
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
	left_item, left_position, err := doc.findItemFromId(item.origin_left)
	if err != nil {
		return fmt.Errorf("origin_left not found: %w", err)
	}
	dest_item := doc.content.head
	dest_position := 0
	if left_item != nil {
		// We will place the item after the left item
		dest_item = left_item
		dest_position = left_position + 1
		if dest_position > left_item.item.length-1 {
			// Go to the next item if we were already at the end of the left item
			dest_item = dest_item.next
			dest_position = 0
		}
	}
	right_item, right_position, err := doc.findItemFromId(item.origin_right)
	if err != nil {
		return fmt.Errorf("origin_right not found: %w", err)
	}
	scanning := true
	for other := dest_item; ; other = other.next {
		if !scanning {
			dest_item = other
		}
		if other == nil || other == right_item {
			break
		}

		oleft, oleft_position, err := doc.findItemFromId(other.item.origin_left)
		if err != nil {
			return fmt.Errorf("origin_left not found: %w", err)
		}

		oright, oright_position, err := doc.findItemFromId(other.item.origin_right)
		if err != nil {
			return fmt.Errorf("origin_right not found: %w", err)
		}

		// Fugue logic
		order_left := order(oleft, oleft_position, left_item, left_position)
		if order_left <= 0 {
			order_right := order(oright, oright_position, right_item, right_position)
			if order_left < 0 || (order_left == 0 && order_right == 0 && item.id.client < other.item.id.client) {
				break
			}
			if order_left == 0 {
				scanning = order_right < 0
			}
		}
		dest_position = 0
	}
	if dest_item == nil {
		// We insert at the end of the list
		doc.insertAfter(doc.content.tail, item)
		if doc.content.tail.canMergeLeft() {
			// The new tail can be merged with the previous item
			err := doc.mergeLeft(doc.content.tail)
			if err != nil {
				return fmt.Errorf("error merging left: %w", err)
			}
		}
		return nil
	}
	// We insert in the rest of the list
	_, middle, _, err := doc.insertAt(dest_item, dest_position, item)
	if err != nil {
		return fmt.Errorf("error inserting item: %w", err)
	}
	if middle.canMergeLeft() {
		err := doc.mergeLeft(middle)
		if err != nil {
			return fmt.Errorf("error merging left: %w", err)
		}
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

// TODO: optimize the deletion by not restarting at the beginning of the list for each deletion
//
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
				left, middle_right, err1 := dest.splitTwo(dest_item, left_split_count)
				if err1 != nil {
					return fmt.Errorf("error splitting item: %w", err1)
				}
				if middle_right == nil {
					// No right split means we deleted the whole item
					left.item.deleted = true
					//dest.content.count -= left.item.length
					// Try to merge with the previous item
					if left.canMergeLeft() {
						dest.mergeLeft(left)
					}
					continue
				}
				deleted_item_count := dest_item.item.length - left_split_count + min(int(from_item.item.id.seq)+from_item.item.length-int(dest_item.item.id.seq)-dest_item.item.length, 0)
				middle, _, err2 := dest.splitTwo(middle_right, deleted_item_count)
				if err2 != nil {
					return fmt.Errorf("error splitting item: %w", err2)
				}
				middle.item.deleted = true
				//dest.content.count -= middle.item.length
				if middle.canMergeLeft() {
					// We can merge the deleted part with the previous item
					dest.mergeLeft(middle)
					// Move to the previous item, so that we can do merging in both directions
					middle = middle.prev
				}
				if middle.canMergeRight() {
					// We can merge the deleted part with the next item
					dest.mergeRight(middle)
				}
			}
		}
	}
	return nil
}

// debugPrint prints the content of the document in a human readable format
func (doc *Doc) debugPrint() {
	for item := doc.content.head; item != nil; item = item.next {
		fmt.Printf("Content: '%s' ID: {client: %d, seq: %d} Origins: left=%v, right=%v Deleted=%t Length=%d\n",
			item.item.content,
			item.item.id.client,
			item.item.id.seq,
			item.item.origin_left,
			item.item.origin_right,
			item.item.deleted,
			item.item.length)
	}
	fmt.Printf("Length: %d Count: %d\n", doc.content.length, doc.content.count)
	fmt.Println("---")
	for client, seq := range doc.version {
		fmt.Printf("Client: %d Seq: %d\n", client, seq)
		for _, item := range doc.cache[client] {
			fmt.Printf("%d ", item.item.id.seq)
		}
		fmt.Print("\n")
		fmt.Println("---")
	}
}

func main() {
	/*doc := newDoc()
	doc.localInsert(0, 0, "\\")
	doc.localInsert(0, 1, "d")
	doc.localInsert(0, 2, "o")
	doc.localInsert(0, 3, "c")
	doc.localInsert(0, 4, "u")
	doc.localInsert(0, 5, "m")
	doc.localInsert(0, 6, "e")
	doc.localInsert(0, 7, "n")
	doc.localInsert(0, 8, "t")
	doc.localInsert(0, 9, "c")
	doc.localInsert(0, 10, "l")
	doc.localInsert(0, 11, "a")
	doc.localInsert(0, 12, "s")
	doc.localInsert(0, 13, "s")
	doc.localInsert(0, 14, "[")
	doc.localInsert(0, 15, "a")
	doc.localInsert(0, 16, "4")
	doc.localInsert(0, 17, "p")
	doc.localInsert(0, 18, "a")
	doc.localInsert(0, 19, "p")
	doc.localInsert(0, 20, "e")
	doc.localInsert(0, 21, "r")
	doc.localInsert(0, 22, ",")
	doc.localInsert(0, 23, "t")
	doc.localInsert(0, 24, "w")
	doc.localInsert(0, 25, "o")
	doc.localInsert(0, 26, "c")
	doc.localInsert(0, 27, "o")
	doc.localInsert(0, 28, "l")
	doc.localInsert(0, 29, "u")
	doc.localInsert(0, 30, "m")
	doc.localInsert(0, 31, "n")
	doc.localInsert(0, 32, ",")
	doc.localInsert(0, 33, "1")
	doc.localInsert(0, 34, "0")
	doc.localInsert(0, 35, "p")
	doc.localInsert(0, 36, "t")
	doc.localInsert(0, 37, "]")
	doc.localInsert(0, 38, "{")
	doc.localInsert(0, 39, "a")
	doc.localInsert(0, 40, "r")
	doc.localInsert(0, 41, "t")
	doc.localInsert(0, 42, "i")
	doc.localInsert(0, 43, "c")
	doc.localInsert(0, 44, "l")
	doc.localInsert(0, 45, "e")
	doc.localInsert(0, 46, "}")
	doc.localInsert(0, 47, "\n")
	doc.localInsert(0, 48, "\\")
	doc.localDelete(20, 1)
	doc.debugPrint()*/
	//fmt.Println(doc.getContent())
	doc1 := newDoc()
	fmt.Println(doc1.localInsert(1, 0, "a"))
	fmt.Println(doc1.localInsert(1, 1, "b"))
	fmt.Println(doc1.localDelete(0, 1))
	doc1.debugPrint()
	fmt.Println(doc1.localInsert(1, 0, "c"))
	fmt.Println(doc1.localInsert(1, 1, "d"))
	fmt.Println(doc1.localInsert(1, 1, "e"))
	doc1.debugPrint()

}
