// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

<<<<<<< HEAD
// This file provides the generic implementation of Sum and MAC. Other files
// might provide optimized assembly implementations of some of this code.

=======
>>>>>>> cbc9bb05... fixup add vendor back
package poly1305

import "encoding/binary"

<<<<<<< HEAD
// Poly1305 [RFC 7539] is a relatively simple algorithm: the authentication tag
// for a 64 bytes message is approximately
//
//     s + m[0:16] * r⁴ + m[16:32] * r³ + m[32:48] * r² + m[48:64] * r  mod  2¹³⁰ - 5
//
// for some secret r and s. It can be computed sequentially like
//
//     for len(msg) > 0:
//         h += read(msg, 16)
//         h *= r
//         h %= 2¹³⁰ - 5
//     return h + s
//
// All the complexity is about doing performant constant-time math on numbers
// larger than any available numeric type.

=======
const (
	msgBlock   = uint32(1 << 24)
	finalBlock = uint32(0)
)

// sumGeneric generates an authenticator for msg using a one-time key and
// puts the 16-byte result into out. This is the generic implementation of
// Sum and should be called if no assembly implementation is available.
>>>>>>> cbc9bb05... fixup add vendor back
func sumGeneric(out *[TagSize]byte, msg []byte, key *[32]byte) {
	h := newMACGeneric(key)
	h.Write(msg)
	h.Sum(out)
}

<<<<<<< HEAD
func newMACGeneric(key *[32]byte) macGeneric {
	m := macGeneric{}
	initialize(key, &m.macState)
	return m
}

// macState holds numbers in saturated 64-bit little-endian limbs. That is,
// the value of [x0, x1, x2] is x[0] + x[1] * 2⁶⁴ + x[2] * 2¹²⁸.
type macState struct {
	// h is the main accumulator. It is to be interpreted modulo 2¹³⁰ - 5, but
	// can grow larger during and after rounds. It must, however, remain below
	// 2 * (2¹³⁰ - 5).
	h [3]uint64
	// r and s are the private key components.
	r [2]uint64
	s [2]uint64
}

type macGeneric struct {
	macState
=======
func newMACGeneric(key *[32]byte) (h macGeneric) {
	h.r[0] = binary.LittleEndian.Uint32(key[0:]) & 0x3ffffff
	h.r[1] = (binary.LittleEndian.Uint32(key[3:]) >> 2) & 0x3ffff03
	h.r[2] = (binary.LittleEndian.Uint32(key[6:]) >> 4) & 0x3ffc0ff
	h.r[3] = (binary.LittleEndian.Uint32(key[9:]) >> 6) & 0x3f03fff
	h.r[4] = (binary.LittleEndian.Uint32(key[12:]) >> 8) & 0x00fffff

	h.s[0] = binary.LittleEndian.Uint32(key[16:])
	h.s[1] = binary.LittleEndian.Uint32(key[20:])
	h.s[2] = binary.LittleEndian.Uint32(key[24:])
	h.s[3] = binary.LittleEndian.Uint32(key[28:])
	return
}

type macGeneric struct {
	h, r [5]uint32
	s    [4]uint32
>>>>>>> cbc9bb05... fixup add vendor back

	buffer [TagSize]byte
	offset int
}

<<<<<<< HEAD
// Write splits the incoming message into TagSize chunks, and passes them to
// update. It buffers incomplete chunks.
func (h *macGeneric) Write(p []byte) (int, error) {
	nn := len(p)
	if h.offset > 0 {
		n := copy(h.buffer[h.offset:], p)
		if h.offset+n < TagSize {
			h.offset += n
			return nn, nil
		}
		p = p[n:]
		h.offset = 0
		updateGeneric(&h.macState, h.buffer[:])
	}
	if n := len(p) - (len(p) % TagSize); n > 0 {
		updateGeneric(&h.macState, p[:n])
		p = p[n:]
=======
func (h *macGeneric) Write(p []byte) (n int, err error) {
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
		updateGeneric(h.buffer[:], msgBlock, &(h.h), &(h.r))
	}
	if nn := len(p) - (len(p) % TagSize); nn > 0 {
		updateGeneric(p, msgBlock, &(h.h), &(h.r))
		p = p[nn:]
>>>>>>> cbc9bb05... fixup add vendor back
	}
	if len(p) > 0 {
		h.offset += copy(h.buffer[h.offset:], p)
	}
<<<<<<< HEAD
	return nn, nil
}

// Sum flushes the last incomplete chunk from the buffer, if any, and generates
// the MAC output. It does not modify its state, in order to allow for multiple
// calls to Sum, even if no Write is allowed after Sum.
func (h *macGeneric) Sum(out *[TagSize]byte) {
	state := h.macState
	if h.offset > 0 {
		updateGeneric(&state, h.buffer[:h.offset])
	}
	finalize(out, &state.h, &state.s)
}

// [rMask0, rMask1] is the specified Poly1305 clamping mask in little-endian. It
// clears some bits of the secret coefficient to make it possible to implement
// multiplication more efficiently.
const (
	rMask0 = 0x0FFFFFFC0FFFFFFF
	rMask1 = 0x0FFFFFFC0FFFFFFC
)

// initialize loads the 256-bit key into the two 128-bit secret values r and s.
func initialize(key *[32]byte, m *macState) {
	m.r[0] = binary.LittleEndian.Uint64(key[0:8]) & rMask0
	m.r[1] = binary.LittleEndian.Uint64(key[8:16]) & rMask1
	m.s[0] = binary.LittleEndian.Uint64(key[16:24])
	m.s[1] = binary.LittleEndian.Uint64(key[24:32])
}

// uint128 holds a 128-bit number as two 64-bit limbs, for use with the
// bits.Mul64 and bits.Add64 intrinsics.
type uint128 struct {
	lo, hi uint64
}

func mul64(a, b uint64) uint128 {
	hi, lo := bitsMul64(a, b)
	return uint128{lo, hi}
}

func add128(a, b uint128) uint128 {
	lo, c := bitsAdd64(a.lo, b.lo, 0)
	hi, c := bitsAdd64(a.hi, b.hi, c)
	if c != 0 {
		panic("poly1305: unexpected overflow")
	}
	return uint128{lo, hi}
}

func shiftRightBy2(a uint128) uint128 {
	a.lo = a.lo>>2 | (a.hi&3)<<62
	a.hi = a.hi >> 2
	return a
}

// updateGeneric absorbs msg into the state.h accumulator. For each chunk m of
// 128 bits of message, it computes
//
//     h₊ = (h + m) * r  mod  2¹³⁰ - 5
//
// If the msg length is not a multiple of TagSize, it assumes the last
// incomplete chunk is the final one.
func updateGeneric(state *macState, msg []byte) {
	h0, h1, h2 := state.h[0], state.h[1], state.h[2]
	r0, r1 := state.r[0], state.r[1]

	for len(msg) > 0 {
		var c uint64

		// For the first step, h + m, we use a chain of bits.Add64 intrinsics.
		// The resulting value of h might exceed 2¹³⁰ - 5, but will be partially
		// reduced at the end of the multiplication below.
		//
		// The spec requires us to set a bit just above the message size, not to
		// hide leading zeroes. For full chunks, that's 1 << 128, so we can just
		// add 1 to the most significant (2¹²⁸) limb, h2.
		if len(msg) >= TagSize {
			h0, c = bitsAdd64(h0, binary.LittleEndian.Uint64(msg[0:8]), 0)
			h1, c = bitsAdd64(h1, binary.LittleEndian.Uint64(msg[8:16]), c)
			h2 += c + 1

			msg = msg[TagSize:]
		} else {
			var buf [TagSize]byte
			copy(buf[:], msg)
			buf[len(msg)] = 1

			h0, c = bitsAdd64(h0, binary.LittleEndian.Uint64(buf[0:8]), 0)
			h1, c = bitsAdd64(h1, binary.LittleEndian.Uint64(buf[8:16]), c)
			h2 += c

			msg = nil
		}

		// Multiplication of big number limbs is similar to elementary school
		// columnar multiplication. Instead of digits, there are 64-bit limbs.
		//
		// We are multiplying a 3 limbs number, h, by a 2 limbs number, r.
		//
		//                        h2    h1    h0  x
		//                              r1    r0  =
		//                       ----------------
		//                      h2r0  h1r0  h0r0     <-- individual 128-bit products
		//            +   h2r1  h1r1  h0r1
		//               ------------------------
		//                 m3    m2    m1    m0      <-- result in 128-bit overlapping limbs
		//               ------------------------
		//         m3.hi m2.hi m1.hi m0.hi           <-- carry propagation
		//     +         m3.lo m2.lo m1.lo m0.lo
		//        -------------------------------
		//           t4    t3    t2    t1    t0      <-- final result in 64-bit limbs
		//
		// The main difference from pen-and-paper multiplication is that we do
		// carry propagation in a separate step, as if we wrote two digit sums
		// at first (the 128-bit limbs), and then carried the tens all at once.

		h0r0 := mul64(h0, r0)
		h1r0 := mul64(h1, r0)
		h2r0 := mul64(h2, r0)
		h0r1 := mul64(h0, r1)
		h1r1 := mul64(h1, r1)
		h2r1 := mul64(h2, r1)

		// Since h2 is known to be at most 7 (5 + 1 + 1), and r0 and r1 have their
		// top 4 bits cleared by rMask{0,1}, we know that their product is not going
		// to overflow 64 bits, so we can ignore the high part of the products.
		//
		// This also means that the product doesn't have a fifth limb (t4).
		if h2r0.hi != 0 {
			panic("poly1305: unexpected overflow")
		}
		if h2r1.hi != 0 {
			panic("poly1305: unexpected overflow")
		}

		m0 := h0r0
		m1 := add128(h1r0, h0r1) // These two additions don't overflow thanks again
		m2 := add128(h2r0, h1r1) // to the 4 masked bits at the top of r0 and r1.
		m3 := h2r1

		t0 := m0.lo
		t1, c := bitsAdd64(m1.lo, m0.hi, 0)
		t2, c := bitsAdd64(m2.lo, m1.hi, c)
		t3, _ := bitsAdd64(m3.lo, m2.hi, c)

		// Now we have the result as 4 64-bit limbs, and we need to reduce it
		// modulo 2¹³⁰ - 5. The special shape of this Crandall prime lets us do
		// a cheap partial reduction according to the reduction identity
		//
		//     c * 2¹³⁰ + n  =  c * 5 + n  mod  2¹³⁰ - 5
		//
		// because 2¹³⁰ = 5 mod 2¹³⁰ - 5. Partial reduction since the result is
		// likely to be larger than 2¹³⁰ - 5, but still small enough to fit the
		// assumptions we make about h in the rest of the code.
		//
		// See also https://speakerdeck.com/gtank/engineering-prime-numbers?slide=23

		// We split the final result at the 2¹³⁰ mark into h and cc, the carry.
		// Note that the carry bits are effectively shifted left by 2, in other
		// words, cc = c * 4 for the c in the reduction identity.
		h0, h1, h2 = t0, t1, t2&maskLow2Bits
		cc := uint128{t2 & maskNotLow2Bits, t3}

		// To add c * 5 to h, we first add cc = c * 4, and then add (cc >> 2) = c.

		h0, c = bitsAdd64(h0, cc.lo, 0)
		h1, c = bitsAdd64(h1, cc.hi, c)
		h2 += c

		cc = shiftRightBy2(cc)

		h0, c = bitsAdd64(h0, cc.lo, 0)
		h1, c = bitsAdd64(h1, cc.hi, c)
		h2 += c

		// h2 is at most 3 + 1 + 1 = 5, making the whole of h at most
		//
		//     5 * 2¹²⁸ + (2¹²⁸ - 1) = 6 * 2¹²⁸ - 1
	}

	state.h[0], state.h[1], state.h[2] = h0, h1, h2
}

const (
	maskLow2Bits    uint64 = 0x0000000000000003
	maskNotLow2Bits uint64 = ^maskLow2Bits
)

// select64 returns x if v == 1 and y if v == 0, in constant time.
func select64(v, x, y uint64) uint64 { return ^(v-1)&x | (v-1)&y }

// [p0, p1, p2] is 2¹³⁰ - 5 in little endian order.
const (
	p0 = 0xFFFFFFFFFFFFFFFB
	p1 = 0xFFFFFFFFFFFFFFFF
	p2 = 0x0000000000000003
)

// finalize completes the modular reduction of h and computes
//
//     out = h + s  mod  2¹²⁸
//
func finalize(out *[TagSize]byte, h *[3]uint64, s *[2]uint64) {
	h0, h1, h2 := h[0], h[1], h[2]

	// After the partial reduction in updateGeneric, h might be more than
	// 2¹³⁰ - 5, but will be less than 2 * (2¹³⁰ - 5). To complete the reduction
	// in constant time, we compute t = h - (2¹³⁰ - 5), and select h as the
	// result if the subtraction underflows, and t otherwise.

	hMinusP0, b := bitsSub64(h0, p0, 0)
	hMinusP1, b := bitsSub64(h1, p1, b)
	_, b = bitsSub64(h2, p2, b)

	// h = h if h < p else h - p
	h0 = select64(b, h0, hMinusP0)
	h1 = select64(b, h1, hMinusP1)

	// Finally, we compute the last Poly1305 step
	//
	//     tag = h + s  mod  2¹²⁸
	//
	// by just doing a wide addition with the 128 low bits of h and discarding
	// the overflow.
	h0, c := bitsAdd64(h0, s[0], 0)
	h1, _ = bitsAdd64(h1, s[1], c)

	binary.LittleEndian.PutUint64(out[0:8], h0)
	binary.LittleEndian.PutUint64(out[8:16], h1)
=======
	return n, nil
}

func (h *macGeneric) Sum(out *[16]byte) {
	H, R := h.h, h.r
	if h.offset > 0 {
		var buffer [TagSize]byte
		copy(buffer[:], h.buffer[:h.offset])
		buffer[h.offset] = 1 // invariant: h.offset < TagSize
		updateGeneric(buffer[:], finalBlock, &H, &R)
	}
	finalizeGeneric(out, &H, &(h.s))
}

func updateGeneric(msg []byte, flag uint32, h, r *[5]uint32) {
	h0, h1, h2, h3, h4 := h[0], h[1], h[2], h[3], h[4]
	r0, r1, r2, r3, r4 := uint64(r[0]), uint64(r[1]), uint64(r[2]), uint64(r[3]), uint64(r[4])
	R1, R2, R3, R4 := r1*5, r2*5, r3*5, r4*5

	for len(msg) >= TagSize {
		// h += msg
		h0 += binary.LittleEndian.Uint32(msg[0:]) & 0x3ffffff
		h1 += (binary.LittleEndian.Uint32(msg[3:]) >> 2) & 0x3ffffff
		h2 += (binary.LittleEndian.Uint32(msg[6:]) >> 4) & 0x3ffffff
		h3 += (binary.LittleEndian.Uint32(msg[9:]) >> 6) & 0x3ffffff
		h4 += (binary.LittleEndian.Uint32(msg[12:]) >> 8) | flag

		// h *= r
		d0 := (uint64(h0) * r0) + (uint64(h1) * R4) + (uint64(h2) * R3) + (uint64(h3) * R2) + (uint64(h4) * R1)
		d1 := (d0 >> 26) + (uint64(h0) * r1) + (uint64(h1) * r0) + (uint64(h2) * R4) + (uint64(h3) * R3) + (uint64(h4) * R2)
		d2 := (d1 >> 26) + (uint64(h0) * r2) + (uint64(h1) * r1) + (uint64(h2) * r0) + (uint64(h3) * R4) + (uint64(h4) * R3)
		d3 := (d2 >> 26) + (uint64(h0) * r3) + (uint64(h1) * r2) + (uint64(h2) * r1) + (uint64(h3) * r0) + (uint64(h4) * R4)
		d4 := (d3 >> 26) + (uint64(h0) * r4) + (uint64(h1) * r3) + (uint64(h2) * r2) + (uint64(h3) * r1) + (uint64(h4) * r0)

		// h %= p
		h0 = uint32(d0) & 0x3ffffff
		h1 = uint32(d1) & 0x3ffffff
		h2 = uint32(d2) & 0x3ffffff
		h3 = uint32(d3) & 0x3ffffff
		h4 = uint32(d4) & 0x3ffffff

		h0 += uint32(d4>>26) * 5
		h1 += h0 >> 26
		h0 = h0 & 0x3ffffff

		msg = msg[TagSize:]
	}

	h[0], h[1], h[2], h[3], h[4] = h0, h1, h2, h3, h4
}

func finalizeGeneric(out *[TagSize]byte, h *[5]uint32, s *[4]uint32) {
	h0, h1, h2, h3, h4 := h[0], h[1], h[2], h[3], h[4]

	// h %= p reduction
	h2 += h1 >> 26
	h1 &= 0x3ffffff
	h3 += h2 >> 26
	h2 &= 0x3ffffff
	h4 += h3 >> 26
	h3 &= 0x3ffffff
	h0 += 5 * (h4 >> 26)
	h4 &= 0x3ffffff
	h1 += h0 >> 26
	h0 &= 0x3ffffff

	// h - p
	t0 := h0 + 5
	t1 := h1 + (t0 >> 26)
	t2 := h2 + (t1 >> 26)
	t3 := h3 + (t2 >> 26)
	t4 := h4 + (t3 >> 26) - (1 << 26)
	t0 &= 0x3ffffff
	t1 &= 0x3ffffff
	t2 &= 0x3ffffff
	t3 &= 0x3ffffff

	// select h if h < p else h - p
	t_mask := (t4 >> 31) - 1
	h_mask := ^t_mask
	h0 = (h0 & h_mask) | (t0 & t_mask)
	h1 = (h1 & h_mask) | (t1 & t_mask)
	h2 = (h2 & h_mask) | (t2 & t_mask)
	h3 = (h3 & h_mask) | (t3 & t_mask)
	h4 = (h4 & h_mask) | (t4 & t_mask)

	// h %= 2^128
	h0 |= h1 << 26
	h1 = ((h1 >> 6) | (h2 << 20))
	h2 = ((h2 >> 12) | (h3 << 14))
	h3 = ((h3 >> 18) | (h4 << 8))

	// s: the s part of the key
	// tag = (h + s) % (2^128)
	t := uint64(h0) + uint64(s[0])
	h0 = uint32(t)
	t = uint64(h1) + uint64(s[1]) + (t >> 32)
	h1 = uint32(t)
	t = uint64(h2) + uint64(s[2]) + (t >> 32)
	h2 = uint32(t)
	t = uint64(h3) + uint64(s[3]) + (t >> 32)
	h3 = uint32(t)

	binary.LittleEndian.PutUint32(out[0:], h0)
	binary.LittleEndian.PutUint32(out[4:], h1)
	binary.LittleEndian.PutUint32(out[8:], h2)
	binary.LittleEndian.PutUint32(out[12:], h3)
>>>>>>> cbc9bb05... fixup add vendor back
}
