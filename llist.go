package main

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

type Client uint8
type Seq uint32
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
	content      Content // Now multiple characters
}

type LinkedItem struct {
	item Item
	prev *LinkedItem
	next *LinkedItem
}

type LinkedList struct {
	length int // length of the list, not used
	count  int // number of items in the list, different than length
	head   *LinkedItem
	tail   *LinkedItem
}

// THIS IS VERY SLOW, SHOULD NOT BE USED
// WE WILL STORE THE LENGTH OF THE CONTENT IN THE ITEM LATER
func (content *Content) length() int {
	// The content is encoded in UTF-8, so we need to count the number of runes
	// instead of the number of bytes
	return utf8.RuneCountInString(string(*content))
}

// delete removes the item from the list and updates both length and count
func (list *LinkedList) delete(item *LinkedItem) error {
	if item == nil {
		return errors.New("item is nil")
	}
	if item.prev != nil {
		item.prev.next = item.next
	} else {
		list.head = item.next
	}
	if item.next != nil {
		item.next.prev = item.prev
	} else {
		list.tail = item.prev
	}
	list.length--
	list.count -= len(item.item.content)
	return nil
}

// mergeLeft merges the content of the item at 'at' with the content of the item to its left
// and deletes the item at 'at'.
//
// Warning: make sure to verify that the merging is valid before calling this function
func (list *LinkedList) mergeLeft(at *LinkedItem) error {
	if at == nil {
		return errors.New("item is nil")
	}
	if at.prev == nil {
		return errors.New("no left item to merge with")
	}

	at.prev.item.content += at.item.content
	/* Below should be better but no performance gain was observed
	    sb.WriteString(string(at.prev.item.content))
	   	sb.WriteString(string(at.item.content))
	   	at.prev.item.content = Content(sb.String())
	   	sb.Reset()
	*/

	// Update the count of the list, this change will be counterbalanced by the deletion
	list.count += len(at.item.content)
	if err := list.delete(at); err != nil {
		return fmt.Errorf("error deleting item: %w", err)
	}
	return nil
}

// mergeRight merges the content of the item at 'at' with the content of the item to its right
// and deletes the item at 'at'.
//
// Warning: make sure to verify that the merging is valid before calling this function
func (list *LinkedList) mergeRight(at *LinkedItem) error {
	if at == nil {
		return errors.New("item is nil")
	}
	return list.mergeLeft(at.next)
}

// splitTwo splits the item at 'at' into two items at the given position.
// The left part contains the first 'position' characters of the original item,
// and the right part contains the rest.
//
// Returns
// - left: the left part of the split
// - right: the right part of the split
// - err: an error if the split is not possible
func (list *LinkedList) splitTwo(at *LinkedItem, position int) (left *LinkedItem, right *LinkedItem, err error) {
	if at == nil {
		return nil, nil, errors.New("item is nil")
	}
	if position == 0 {
		return nil, at, nil
	}
	if position < 0 || position > len(at.item.content) {
		return nil, nil, errors.New("position out of bound")
	}
	if position == len(at.item.content) {
		return at, nil, nil
	}
	// Create the left part of the split
	left_item := at.item
	left_item.content = at.item.content[:position]

	// Modify the original item to be the right part of the split
	at.item.id.seq = at.item.id.seq + Seq(position)
	at.item.content = at.item.content[position:]
	at.item.origin_left = &Id{
		client: left_item.id.client,
		seq:    at.item.id.seq - 1,
	}

	// Update the count of the list, this change will be counterbalanced by the insertion
	list.count -= len(left_item.content)
	list.insertBefore(at, left_item)
	return at.prev, at, nil
}

func (list *LinkedList) insertAt(at *LinkedItem, position int, item Item) (left *LinkedItem, middle *LinkedItem, right *LinkedItem, err error) {
	left_split, right_split, err := list.splitTwo(at, position)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error splitting item: %w", err)
	}
	if right_split == nil {
		list.insertAfter(left_split, item)
		return left_split, left_split.next, nil, nil
	}
	list.insertBefore(right_split, item)
	return nil, right_split.prev, right_split, nil
}

// insertAfter inserts an item after the given item in the list.
// If 'at' is nil, the item is inserted at the beginning of the list.
func (list *LinkedList) insertAfter(at *LinkedItem, item Item) {
	list.length++
	list.count += len(item.content)
	linked_item := &LinkedItem{item: item}
	if at == nil { // insert at the beginning
		if list.head == nil { // insert in an empty list
			list.head = linked_item
			list.tail = linked_item
		} else { // insert at the beginning
			list.head.prev = linked_item
			linked_item.next = list.head
			list.head = linked_item
		}
	} else {
		if at.next == nil { // insert at the end
			list.tail = linked_item
			linked_item.prev = at
			at.next = linked_item
		} else { // insert in the middle
			at.next.prev = linked_item
			linked_item.prev = at
			linked_item.next = at.next
			at.next = linked_item
		}
	}
}

// insertBefore inserts an item before the given item in the list.
// If 'at' is nil, the item is inserted at the end of the list.
func (list *LinkedList) insertBefore(at *LinkedItem, item Item) {
	list.length++
	list.count += len(item.content)
	linked_item := &LinkedItem{item: item}
	if at == nil { // insert at the end
		if list.tail == nil { // insert in an empty list
			list.head = linked_item
			list.tail = linked_item
		} else { // insert at the end
			list.tail.next = linked_item
			linked_item.prev = list.tail
			list.tail = linked_item
		}
	} else {
		if at.prev == nil { // insert at the beginning
			list.head = linked_item
			linked_item.next = at
			at.prev = linked_item
		} else { // insert in the middle
			at.prev.next = linked_item
			linked_item.prev = at.prev
			linked_item.next = at
			at.prev = linked_item
		}
	}
}
