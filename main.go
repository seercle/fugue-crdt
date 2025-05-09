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

/**
 * Return the position in the document of the item with the given id.
 * The LinkedItem returned contains the item with the given id
 * Return -1 if the item is nil
 */
func (doc *Doc) findItemFromId(id *Id) (*LinkedItem, int, error) {
	if id == nil {
		return nil, -1, nil
	}
	i := 0
	for linked_item := doc.content.head; linked_item != nil; linked_item = linked_item.next {
		if linked_item.item.id.client != id.client { // different client
			i += len(linked_item.item.content)
		} else {
			if linked_item.item.id.seq <= id.seq &&
				id.seq <= linked_item.item.id.seq+Seq(len(linked_item.item.content)-1) { // the item is contained in the linked_item
				return linked_item, i + int(id.seq-linked_item.item.id.seq), nil
			}
			i += len(linked_item.item.content) // skip to the next item
		}
	}
	return nil, -1, ErrNotFound
}

func (doc *Doc) findItemAt(position int, stick_end bool) (*LinkedItem, int, *OutOfBoundErr) {
	if position < 0 {
		return nil, -1, &OutOfBoundErr{position}
	}
	for item := doc.content.head; item != nil; item = item.next {
		if stick_end && position == 0 { // the item is contained in the linked_item
			return item, position, nil
		} else if item.item.deleted { // skip deleted items without counting them
			continue
		} else if len(item.item.content) > position { // the item is contained in the linked_item
			return item, position, nil
		}
		position -= len(item.item.content) // skip to the next item
	}
	return nil, -1, &OutOfBoundErr{position}
}

func (doc *Doc) localInsertOne(client Client, position int, content Content) error {
	if position < 0 {
		return fmt.Errorf("position must be greater than 0")
	}
	var origin_left *Id = nil
	var origin_right *Id = nil
	item, item_position, err := doc.findItemAt(position, true)

	if err != nil && err.overflow > 0 { // we allow an overflow of 1
		return fmt.Errorf("item not found: %w", err)
	}
	var seq Seq = 0
	if val, ok := doc.version[client]; ok {
		seq = val + 1
	}

	// Find the left and right origins
	if item == nil {
		if doc.content.tail != nil { // if we insert at the end
			origin_left = &Id{
				client: doc.content.tail.item.id.client,
				seq:    doc.content.tail.item.id.seq,
			}
			origin_left.seq += Seq(len(doc.content.tail.item.content) - 1)
		}
	} else {
		// if we insert in the middle
		// id of the item we found at the position
		// it will be the right origin
		origin_right = &Id{
			client: item.item.id.client,
			seq:    item.item.id.seq + Seq(item_position),
		}
		if item_position == 0 { // if we insert at the beginning of the item
			if item.prev != nil { // if we insert after something
				prev_id := item.prev.item.id
				origin_left = &Id{
					client: prev_id.client,
					seq:    prev_id.seq + Seq(len(item.prev.item.content)-1), // the right most item of the previous item
				}
			}
		} else { // if we insert in the middle of the item
			origin_left = &Id{
				client: item.item.id.client,
				seq:    item.item.id.seq + Seq(item_position-1),
			}
		}
	}
	fmt.Println(origin_left, origin_right)
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

func (doc *Doc) localDelete(position int, length int) error {
	if length <= 0 {
		return fmt.Errorf("length must be greater than 0")
	}
	item, item_position, err := doc.findItemAt(position, false)
	if err != nil {
		return fmt.Errorf("item not found: %w", err)
	}
	if !item.item.deleted && item_position > 0 { // if we are in the middle of an item, we need to split it

		new_item_item := Item{ // create a new item that will be the left part of the split
			id: Id{
				client: item.item.id.client,
				seq:    item.item.id.seq,
			},
			origin_left:  item.item.origin_left,
			origin_right: item.item.origin_right,
			deleted:      false,
			content:      item.item.content[:item_position],
		}

		// create a new linked item
		new_item := &LinkedItem{
			item: new_item_item,
			prev: item.prev,
			next: item,
		}

		// edit the content of the item to be the right part of the split
		item.item.id = Id{
			client: item.item.id.client,
			seq:    item.item.id.seq + Seq(item_position),
		}
		item.item.content = item.item.content[item_position:]
		item.item.origin_left = &new_item.item.id

		// insert the new item in the list
		if item.prev != nil {
			item.prev.next = new_item
		} else {
			doc.content.head = new_item
		}
		item.prev = new_item
		doc.content.length++

	}

	// Traverse and mark items as deleted
	for length > 0 && item != nil {
		if !item.item.deleted { // skip deleted items
			if length >= len(item.item.content) { // if we can delete the whole item
				item.item.deleted = true
				length -= len(item.item.content)

				// See if we can merge the item with the previous item
				if item.prev != nil &&
					item.prev.item.deleted && // both items are deleted
					item.prev.item.origin_right.equals(item.item.origin_right) && // in case new item is placed at the left of a merged item
					item.prev.item.id.client == item.item.id.client && // if the item is from the same client
					item.prev.item.id.seq+Seq(len(item.prev.item.content)) == item.item.id.seq {

					item.prev.item.content += item.item.content
					item.prev.next = item.next
					if item.next != nil {
						item.next.prev = item.prev
					} else {
						doc.content.tail = item.prev
					}
					doc.content.length--
					item = item.prev // move to the previous item, so that we can do merging in both directions
				}

				// See if we can merge the item with the next item
				if item.next != nil &&
					item.next.item.deleted && // both items are deleted
					item.next.item.origin_right.equals(item.item.origin_right) && // in case new item is placed at the left of a merged item
					item.next.item.id.client == item.item.id.client && // if the item is from the same client
					item.next.item.id.seq == item.item.id.seq+Seq(len(item.item.content)) {

					item.item.content += item.next.item.content
					item.next = item.next.next
					if item.next != nil {
						item.next.prev = item
					} else {
						doc.content.tail = item
					}
					doc.content.length--
				}
			} else { // if we can only delete part of the item at the end, we need to split it

				//See if we can merge the left part of the split with the previous item
				if item.prev != nil &&
					item.prev.item.deleted && // both items are deleted
					item.prev.item.origin_right.equals(item.item.origin_right) && // in case new item is placed at the left of a merged item
					item.prev.item.id.client == item.item.id.client && // if the item is from the same client
					item.prev.item.id.seq+Seq(len(item.prev.item.content)) == item.item.id.seq {
					item.prev.item.content += item.item.content[:length]
					item.item.id = Id{
						client: item.item.id.client,
						seq:    item.item.id.seq + Seq(length),
					}
					item.item.content = item.item.content[length:]
					item.item.origin_left = &Id{
						client: item.item.id.client,
						seq:    item.item.id.seq - 1,
					}
					return nil
				} else { // if we can't merge, we need to split the item
					new_item_item := Item{ // create a new item that will be the left part of the split
						id: Id{
							client: item.item.id.client,
							seq:    item.item.id.seq,
						},
						origin_left:  item.item.origin_left,
						origin_right: item.item.origin_right,
						deleted:      true,
						content:      item.item.content[:length],
					}
					// create a new linked item
					new_item := &LinkedItem{
						item: new_item_item,
						prev: item.prev,
						next: item,
					}

					// edit the content of the item to be the right part of the split
					item.item.id = Id{
						client: item.item.id.client,
						seq:    item.item.id.seq + Seq(length),
					}
					item.item.content = item.item.content[length:]
					item.item.origin_left = &new_item.item.id

					// insert the new item in the list
					if item.prev != nil {
						item.prev.next = new_item
					} else {
						doc.content.head = new_item
					}
					item.prev = new_item
					doc.content.length++
					return nil // we are done
				}
			}
		}
		item = item.next
	}
	// If length > 0, it means we ran out of items to delete
	if length > 0 {
		return errors.New("not enough items to delete")
	}

	return nil
}

// Return the part of the item that is not in the version, nil if the item is fully in the version
func cropInVersion(item *Item, version *Version) *Item {
	if item == nil {
		return nil
	}
	if seq, ok := (*version)[item.id.client]; ok {
		if seq >= item.id.seq+Seq(len(item.content)-1) { // item is fully in the version
			return nil
		}
		if seq < item.id.seq { // item is fully out of the version
			return item
		}
		// item is partially in the version
		crop := item
		crop.content = item.content[seq-item.id.seq+1:]
		crop.id.seq = seq + 1
		crop.origin_left = &Id{
			client: item.id.client,
			seq:    seq,
		}
		return crop
	}
	return item
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

func (doc *Doc) integrate(item Item) error {
	id := item.id
	var prev_seq Seq = -1
	if val, ok := doc.version[id.client]; ok {
		prev_seq = val
	}
	if id.seq != prev_seq+1 {
		return errors.New("invalid sequence number")
	}
	doc.version[id.client] = id.seq + Seq(len(item.content)-1) // the version also increase with the length of the item

	left_item, left_index, err := doc.findItemFromId(item.origin_left)

	if err != nil {
		return fmt.Errorf("origin_left not found: %w", err)
	}
	var dest_item *LinkedItem = doc.content.head
	var position Seq = 0
	if left_item != nil {
		position = item.origin_left.seq - left_item.item.id.seq
		dest_item = left_item
		position++                                         // we will place the item after the left item
		if position > Seq(len(left_item.item.content)-1) { // go to the next item if we were at the end of the left item
			dest_item = dest_item.next
			position = 0
		}
	}
	var right_item *LinkedItem = nil
	right_index := doc.content.length
	if item.origin_right != nil {
		right_item, right_index, err = doc.findItemFromId(item.origin_right)
		if err != nil {
			return fmt.Errorf("origin_right not found: %w", err)
		}
	}
	scanning := false
	if dest_item != nil {
		//fmt.Println(dest_item.item.id)
	}
	for other := dest_item; ; other = other.next {
		if !scanning {
			dest_item = other
		}
		//fmt.Println("X")
		if other == nil || other == right_item {
			break
		}

		//fmt.Println("Y")
		_, oleft_index, err := doc.findItemFromId(other.item.origin_left)
		oleft_index += int(position)
		if err != nil {
			return fmt.Errorf("origin_left not found: %w", err)
		}
		oright_index := doc.content.length
		if other.item.origin_right != nil {
			_, oright_index, err = doc.findItemFromId(other.item.origin_right)
			if err != nil {
				return fmt.Errorf("origin_right not found: %w", err)
			}
		}
		//fmt.Println("A")
		if oleft_index < left_index || (oleft_index == left_index && oright_index == right_index && item.id.client < other.item.id.client) {
			break
		}
		//fmt.Println("B")
		if oleft_index == left_index {
			scanning = oright_index < right_index
		}

		position = 0 // search at the beginning of every new item
	}
	/*
		fmt.Println("We will insert at")
		fmt.Println(left_item)
		if dest_item != nil {
			fmt.Println(dest_item.item.id, position)
		}
		fmt.Println("-------------")
	*/

	// Put the item in the list before the destination item
	new_item := &LinkedItem{item: item}
	if dest_item != nil {
		fmt.Println(dest_item.item.id, position)
	}
	if doc.content.head == nil { // insert in an empty list
		doc.content.length++
		doc.content.head = new_item
		doc.content.tail = new_item
		return nil
	}
	if dest_item == nil { // insert at the end
		if canMerge(doc.content.tail, new_item) {
			doc.content.tail.item.content += new_item.item.content
			return nil
		}
		doc.content.length++
		doc.content.tail.next = new_item
		new_item.prev = doc.content.tail
		doc.content.tail = new_item
		return nil
	}

	if position == 0 && dest_item.prev == nil { // insert at the beginning
		doc.content.length++
		new_item.next = doc.content.head
		doc.content.head.prev = new_item
		doc.content.head = new_item
		return nil
	}
	// insert in the middle
	if position == 0 { // insert before the destination item

		if canMerge(dest_item.prev, new_item) { // Merge with the previous item if possible
			dest_item.prev.item.content += new_item.item.content
			return nil
		}

		doc.content.length++
		new_item.prev = dest_item.prev
		new_item.next = dest_item
		dest_item.prev.next = new_item
		dest_item.prev = new_item
		return nil
	} else { //insert in the middle of the destination item

		left_split_item := Item{
			id: Id{
				client: dest_item.item.id.client,
				seq:    dest_item.item.id.seq,
			},
			origin_left:  dest_item.item.origin_left,
			origin_right: dest_item.item.origin_right,
			deleted:      dest_item.item.deleted,
			content:      dest_item.item.content[:int(position)],
		}
		fmt.Println(left_split_item.id, left_split_item.content, left_split_item.origin_left, left_split_item.origin_right)
		left_split := &LinkedItem{
			item: left_split_item,
			prev: dest_item.prev,
			next: new_item,
		}
		new_item.prev = left_split
		new_item.next = dest_item
		fmt.Println(new_item.item.id, new_item.item.content, new_item.item.origin_left, new_item.item.origin_right)
		if dest_item.prev != nil {
			dest_item.prev.next = left_split
		} else {
			doc.content.head = left_split
		}

		dest_item.prev = new_item
		dest_item.item.id.seq = dest_item.item.id.seq + Seq(position)
		dest_item.item.content = dest_item.item.content[int(position):]
		dest_item.item.origin_left = &Id{
			client: left_split.item.id.client,
			seq:    left_split.item.id.seq + Seq(len(left_split.item.content)-1),
		}
		fmt.Println(dest_item.item.id, dest_item.item.content, dest_item.item.origin_left, dest_item.item.origin_right)
		return nil
	}
}

func (id1 *Id) equals(id2 *Id) bool {
	return id1 == id2 || (id1 != nil && id2 != nil && id1.client == id2.client && id1.seq == id2.seq)
}

func canMerge(prev *LinkedItem, new *LinkedItem) bool {
	return prev.item.deleted == new.item.deleted && // both items are deleted or not
		prev.item.origin_right.equals(new.item.origin_right) && // in case new item is placed at the left of a merged item
		prev.item.id.client == new.item.id.client && // if the item is from the same client
		prev.item.id.seq+Seq(len(prev.item.content)) == new.item.id.seq
}

func (dest *Doc) mergeFrom(from *Doc) error {
	// Handle insertions
	var missing []Item
	for linked_item := from.content.head; linked_item != nil; linked_item = linked_item.next {
		if cropped := cropInVersion(&linked_item.item, &dest.version); cropped != nil {
			missing = append(missing, *cropped)
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
	/*
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
	*/
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
	doc1 := newDoc()
	/*doc1.localInsertOne(1, 0, "abc")
	doc1.localDelete(0, 1)
	doc1.localDelete(1, 1)
	doc1.localDelete(0, 1)*/
	/*doc1.localInsertOne(1, 0, "abc")
	doc1.localDelete(2, 1)
	doc1.localDelete(0, 2)*/
	/*doc1.localInsertOne(1, 0, "def")
	doc1.localInsertOne(1, 0, "abc")
	doc1.localDelete(2, 2)
	doc1.debugPrint()
	doc1.localInsertOne(1, 3, "ghi")*/
	doc1.localInsertOne(1, 0, "ab")
	doc1.debugPrint()
	doc1.localDelete(1, 1)

	doc1.debugPrint()
	doc1.localInsertOne(1, 1, "c")
	doc1.debugPrint()
	doc1.localDelete(0, 1)
	doc1.debugPrint()

}
