// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

<<<<<<< HEAD
// +build !gccgo,!purego
=======
// +build s390x,go1.11,!gccgo,!appengine
>>>>>>> cbc9bb05... fixup add vendor back

package poly1305

import (
	"golang.org/x/sys/cpu"
)

<<<<<<< HEAD
// updateVX is an assembly implementation of Poly1305 that uses vector
// instructions. It must only be called if the vector facility (vx) is
// available.
//go:noescape
func updateVX(state *macState, msg []byte)

// mac is a replacement for macGeneric that uses a larger buffer and redirects
// calls that would have gone to updateGeneric to updateVX if the vector
// facility is installed.
//
// A larger buffer is required for good performance because the vector
// implementation has a higher fixed cost per call than the generic
// implementation.
type mac struct {
	macState

	buffer [16 * TagSize]byte // size must be a multiple of block size (16)
	offset int
}

func (h *mac) Write(p []byte) (int, error) {
	nn := len(p)
	if h.offset > 0 {
		n := copy(h.buffer[h.offset:], p)
		if h.offset+n < len(h.buffer) {
			h.offset += n
			return nn, nil
		}
		p = p[n:]
		h.offset = 0
		if cpu.S390X.HasVX {
			updateVX(&h.macState, h.buffer[:])
		} else {
			updateGeneric(&h.macState, h.buffer[:])
		}
	}

	tail := len(p) % len(h.buffer) // number of bytes to copy into buffer
	body := len(p) - tail          // number of bytes to process now
	if body > 0 {
		if cpu.S390X.HasVX {
			updateVX(&h.macState, p[:body])
		} else {
			updateGeneric(&h.macState, p[:body])
		}
	}
	h.offset = copy(h.buffer[:], p[body:]) // copy tail bytes - can be 0
	return nn, nil
}

func (h *mac) Sum(out *[TagSize]byte) {
	state := h.macState
	remainder := h.buffer[:h.offset]

	// Use the generic implementation if we have 2 or fewer blocks left
	// to sum. The vector implementation has a higher startup time.
	if cpu.S390X.HasVX && len(remainder) > 2*TagSize {
		updateVX(&state, remainder)
	} else if len(remainder) > 0 {
		updateGeneric(&state, remainder)
	}
	finalize(out, &state.h, &state.s)
=======
// poly1305vx is an assembly implementation of Poly1305 that uses vector
// instructions. It must only be called if the vector facility (vx) is
// available.
//go:noescape
func poly1305vx(out *[16]byte, m *byte, mlen uint64, key *[32]byte)

// poly1305vmsl is an assembly implementation of Poly1305 that uses vector
// instructions, including VMSL. It must only be called if the vector facility (vx) is
// available and if VMSL is supported.
//go:noescape
func poly1305vmsl(out *[16]byte, m *byte, mlen uint64, key *[32]byte)

// Sum generates an authenticator for m using a one-time key and puts the
// 16-byte result into out. Authenticating two different messages with the same
// key allows an attacker to forge messages at will.
func Sum(out *[16]byte, m []byte, key *[32]byte) {
	if cpu.S390X.HasVX {
		var mPtr *byte
		if len(m) > 0 {
			mPtr = &m[0]
		}
		if cpu.S390X.HasVXE && len(m) > 256 {
			poly1305vmsl(out, mPtr, uint64(len(m)), key)
		} else {
			poly1305vx(out, mPtr, uint64(len(m)), key)
		}
	} else {
		sumGeneric(out, m, key)
	}
>>>>>>> cbc9bb05... fixup add vendor back
}
