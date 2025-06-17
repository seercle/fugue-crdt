package main

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

type Client uint8
type Seq int
type Content string

type Id struct {
	client Client // up to 255 clients
	seq    Seq    // up to 2^32 - 1 operations per client
}

type Item struct {
	id           Id
	origin_left  *Id
	origin_right *Id
	deleted      bool
	content      Content // Supports UTF-8 encoded content of any length
	length       int     // length of the content
}

type LinkedItem struct {
	item Item
	prev *LinkedItem
	next *LinkedItem
}

type LinkedList struct {
	length int // length of the list, not used
	count  int // sum of the lengths of all items in the list
	head   *LinkedItem
	tail   *LinkedItem
}

var (
	sb = &strings.Builder{}
)

// length returns the length of the content
func (content *Content) length() int {
	// The content is encoded in UTF-8, so we need to count the number of runes
	// instead of the number of bytes
	return utf8.RuneCountInString(string(*content))
}

// delete removes the item from the list and updates both length and count
func (doc *Doc) delete(item *LinkedItem) error {
	if item == nil {
		return errors.New("item is nil")
	}
	if item.prev != nil {
		item.prev.next = item.next
	} else {
		doc.content.head = item.next
	}
	if item.next != nil {
		item.next.prev = item.prev
	} else {
		doc.content.tail = item.prev
	}
	doc.content.length--
	doc.content.count -= item.item.length
	index, err := doc.findInCache(item.item.id)
	if err != nil {
		return fmt.Errorf("error finding item in cache: %w", err)
	}
	doc.cache[item.item.id.client] = slices.Delete(doc.cache[item.item.id.client], index, index+1)
	return nil
}

// mergeLeft merges the content of the item at 'at' with the content of the item to its left
// and deletes the item at 'at'.
//
// Warning: make sure to verify that the merging is valid before calling this function
func (doc *Doc) mergeLeft(at *LinkedItem) error {
	if at == nil {
		return errors.New("item is nil")
	}
	if at.prev == nil {
		return errors.New("no left item to merge with")
	}

	at.prev.item.length += at.item.length
	sb.WriteString(string(at.prev.item.content))
	sb.WriteString(string(at.item.content))
	at.prev.item.content = Content(sb.String())
	sb.Reset()

	// Update the count of the list, this change will be counterbalanced by the deletion
	doc.content.count += at.item.length
	if err := doc.delete(at); err != nil {
		return fmt.Errorf("error deleting item: %w", err)
	}
	return nil
}

// mergeRight merges the content of the item at 'at' with the content of the item to its right
// and deletes the item at 'at'.
//
// Warning: make sure to verify that the merging is valid before calling this function
func (doc *Doc) mergeRight(at *LinkedItem) error {
	if at == nil {
		return errors.New("item is nil")
	}
	return doc.mergeLeft(at.next)
}

// splitTwo splits the item at 'at' into two items at the given position.
// The left part contains the first 'position' characters of the original item,
// and the right part contains the rest.
//
// Returns
// - left: the left part of the split
// - right: the right part of the split
// - err: an error if the split is not possible
func (doc *Doc) splitTwo(at *LinkedItem, position int) (left *LinkedItem, right *LinkedItem, err error) {
	if at == nil {
		return nil, nil, errors.New("item is nil")
	}
	if position == 0 {
		return nil, at, nil
	}
	if position < 0 || position > at.item.length {
		return nil, nil, errors.New("position out of bound")
	}
	if position == at.item.length {
		return at, nil, nil
	}

	// Find the byte index corresponding to the rune position
	content := string(at.item.content)
	byte_index := 0
	for range position {
		_, size := utf8.DecodeRuneInString(content[byte_index:])
		byte_index += size
	}

	// Create the right part of the split
	right_item := Item{
		id: Id{
			client: at.item.id.client,
			seq:    at.item.id.seq + Seq(position),
		},
		origin_left: &Id{
			client: at.item.id.client,
			seq:    at.item.id.seq + Seq(position) - 1,
		},
		origin_right: at.item.origin_right,
		deleted:      at.item.deleted,
		content:      Content(content[byte_index:]),
		length:       at.item.length - position,
	}

	// Modify the original item to be the left part of the split
	at.item.content = Content(content[:byte_index])
	at.item.length = position

	// Update the count of the list, this change will be counterbalanced by the insertion
	doc.content.count -= right_item.length
	doc.insertAfter(at, right_item)

	return at, at.next, nil
}

func (doc *Doc) insertAt(at *LinkedItem, position int, item Item) (left *LinkedItem, middle *LinkedItem, right *LinkedItem, err error) {
	left_split, right_split, err := doc.splitTwo(at, position)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error splitting item: %w", err)
	}

	if right_split == nil {
		doc.insertAfter(left_split, item)
		return left_split, left_split.next, nil, nil
	}

	doc.insertBefore(right_split, item)
	return nil, right_split.prev, right_split, nil
}

// insertAfter inserts an item after the given item in the list.
// If 'at' is nil, the item is inserted at the beginning of the list.
func (doc *Doc) insertAfter(at *LinkedItem, item Item) {
	doc.content.length++
	doc.content.count += item.length
	linked_item := &LinkedItem{item: item}
	if at == nil { // insert at the beginning
		if doc.content.head == nil { // insert in an empty list
			doc.content.head = linked_item
			doc.content.tail = linked_item
		} else { // insert at the beginning
			doc.content.head.prev = linked_item
			linked_item.next = doc.content.head
			doc.content.head = linked_item
		}
	} else {
		if at.next == nil { // insert at the end
			doc.content.tail = linked_item
			linked_item.prev = at
			at.next = linked_item
		} else { // insert in the middle
			at.next.prev = linked_item
			linked_item.prev = at
			linked_item.next = at.next
			at.next = linked_item
		}
	}
	doc.addToCache(linked_item)
}

// insertBefore inserts an item before the given item in the list.
// If 'at' is nil, the item is inserted at the end of the list.
func (doc *Doc) insertBefore(at *LinkedItem, item Item) {
	doc.content.length++
	doc.content.count += item.length
	linked_item := &LinkedItem{item: item}
	if at == nil { // insert at the end
		if doc.content.tail == nil { // insert in an empty list
			doc.content.head = linked_item
			doc.content.tail = linked_item
		} else { // insert at the end
			doc.content.tail.next = linked_item
			linked_item.prev = doc.content.tail
			doc.content.tail = linked_item
		}
	} else {
		if at.prev == nil { // insert at the beginning
			doc.content.head = linked_item
			linked_item.next = at
			at.prev = linked_item
		} else { // insert in the middle
			at.prev.next = linked_item
			linked_item.prev = at.prev
			linked_item.next = at
			at.prev = linked_item
		}
	}
	doc.addToCache(linked_item)
}
