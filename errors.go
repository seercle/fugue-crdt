package main

import (
	"errors"
	"fmt"
)

type OutOfBoundErr struct {
	overflow int // overflow of the list, starting from 0 (overflow = 0 means the search is out of bound by 1)
}

func (err *OutOfBoundErr) Error() string {
	return fmt.Sprint("object out of bound by %d", err.overflow)
}

var (
	ErrNotFound = errors.New("object not found")
)
