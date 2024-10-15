package main

import (
	"fmt"
	"testing"
)

func TestHandshakeSupportsExtensions(t *testing.T) {
	h := handshake{
		infoHash:          []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		peerID:            []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		supportExtensions: true,
	}

	message := h.makeMessage()

	oh := handshake{}
	err := oh.parseMessage(message)
	if err != nil {
		t.Fatalf("failed to parse handshake message... %s", err.Error())
	}

	if !oh.supportExtensions {
		t.Fatalf("failed to parse that handshake indicates extension support")
	}

	h = handshake{
		infoHash:          []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		peerID:            []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		supportExtensions: false,
	}

	message = h.makeMessage()

	oh = handshake{}
	err = oh.parseMessage(message)
	if err != nil {
		t.Fatalf("failed to parse handshake message... %s", err.Error())
	}

	if oh.supportExtensions {
		t.Fatalf("failed to parse that handshake does not indicates extension support")
	}
}

func TestFoo(t *testing.T) {
	bs := []byte{byte(19), byte(66), byte(105), byte(116), byte(84), byte(111), byte(114), byte(114), byte(101), byte(110), byte(116), byte(32), byte(112), byte(114), byte(111), byte(116), byte(111), byte(99), byte(111), byte(108), byte(0), byte(0), byte(0), byte(0), byte(0), byte(0), byte(0), byte(4), byte(214), byte(159), byte(145), byte(230), byte(178), byte(174), byte(76), byte(84), byte(36), byte(104), byte(209), byte(7), byte(58), byte(113), byte(212), byte(234), byte(19), byte(135), byte(154), byte(127), byte(45), byte(82), byte(78), byte(48), byte(46), byte(48), byte(46), byte(48), byte(45), byte(94), byte(7), byte(181), byte(45), byte(188), byte(124), byte(69), byte(248), byte(249), byte(177), byte(242)}
	h := handshake{}
	if err := h.parseMessage(bs); err != nil {
		t.Fatalf("failed to parse handshake message... %s", err.Error())
	}
}
