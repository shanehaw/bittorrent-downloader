package main

import (
	"testing"
)

func TestCopyTo(t *testing.T) {
	destination := make([]byte, 10)
	for i := 0; i < 5; i++ {
		destination[i] = byte(i)
	}

	src := []byte{
		5, 6, 7, 8, 9,
	}

	copyTo(&destination, src, 5)

	for i := 0; i < 10; i++ {
		if destination[i] != byte(i) {
			t.Fatalf("unexpected value at %d = %d. Expected %d", i, destination[i], i)
		}
	}
}
