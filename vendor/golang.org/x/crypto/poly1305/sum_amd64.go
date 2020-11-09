// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

<<<<<<< HEAD
// +build !gccgo,!purego
=======
// +build amd64,!gccgo,!appengine
>>>>>>> cbc9bb05... fixup add vendor back

package poly1305

//go:noescape
<<<<<<< HEAD
func update(state *macState, msg []byte)

// mac is a wrapper for macGeneric that redirects calls that would have gone to
// updateGeneric to update.
//
// Its Write and Sum methods are otherwise identical to the macGeneric ones, but
// using function pointers would carry a major performance cost.
type mac struct{ macGeneric }

func (h *mac) Write(p []byte) (int, error) {
	nn := len(p)
	if h.offset > 0 {
		n := copy(h.buffer[h.offset:], p)
		if h.offset+n < TagSize {
			h.offset += n
			return nn, nil
		}
		p = p[n:]
		h.offset = 0
		update(&h.macState, h.buffer[:])
	}
	if n := len(p) - (len(p) % TagSize); n > 0 {
		update(&h.macState, p[:n])
		p = p[n:]
=======
func initialize(state *[7]uint64, key *[32]byte)

//go:noescape
func update(state *[7]uint64, msg []byte)

//go:noescape
func finalize(tag *[TagSize]byte, state *[7]uint64)

// Sum generates an authenticator for m using a one-time key and puts the
// 16-byte result into out. Authenticating two different messages with the same
// key allows an attacker to forge messages at will.
func Sum(out *[16]byte, m []byte, key *[32]byte) {
	h := newMAC(key)
	h.Write(m)
	h.Sum(out)
}

func newMAC(key *[32]byte) (h mac) {
	initialize(&h.state, key)
	return
}

type mac struct {
	state [7]uint64 // := uint64{ h0, h1, h2, r0, r1, pad0, pad1 }

	buffer [TagSize]byte
	offset int
}

func (h *mac) Write(p []byte) (n int, err error) {
	n = len(p)
	if h.offset > 0 {
		remaining := TagSize - h.offset
		if n < remaining {
			h.offset += copy(h.buffer[h.offset:], p)
			return n, nil
		}
		copy(h.buffer[h.offset:], p[:remaining])
		p = p[remaining:]
		h.offset = 0
		update(&h.state, h.buffer[:])
	}
	if nn := len(p) - (len(p) % TagSize); nn > 0 {
		update(&h.state, p[:nn])
		p = p[nn:]
>>>>>>> cbc9bb05... fixup add vendor back
	}
	if len(p) > 0 {
		h.offset += copy(h.buffer[h.offset:], p)
	}
<<<<<<< HEAD
	return nn, nil
}

func (h *mac) Sum(out *[16]byte) {
	state := h.macState
	if h.offset > 0 {
		update(&state, h.buffer[:h.offset])
	}
	finalize(out, &state.h, &state.s)
=======
	return n, nil
}

func (h *mac) Sum(out *[16]byte) {
	state := h.state
	if h.offset > 0 {
		update(&state, h.buffer[:h.offset])
	}
	finalize(out, &state)
>>>>>>> cbc9bb05... fixup add vendor back
}
