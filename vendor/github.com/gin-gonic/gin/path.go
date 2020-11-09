// Copyright 2013 Julien Schmidt. All rights reserved.
// Based on the path package, Copyright 2009 The Go Authors.
// Use of this source code is governed by a BSD-style license that can be found
// at https://github.com/julienschmidt/httprouter/blob/master/LICENSE.

package gin

// cleanPath is the URL version of path.Clean, it returns a canonical URL path
// for p, eliminating . and .. elements.
//
// The following rules are applied iteratively until no further processing can
// be done:
//	1. Replace multiple slashes with a single slash.
//	2. Eliminate each . path name element (the current directory).
//	3. Eliminate each inner .. path name element (the parent directory)
//	   along with the non-.. element that precedes it.
//	4. Eliminate .. elements that begin a rooted path:
//	   that is, replace "/.." by "/" at the beginning of a path.
//
// If the result of this process is an empty string, "/" is returned.
func cleanPath(p string) string {
<<<<<<< HEAD
	const stackBufSize = 128
=======
>>>>>>> cbc9bb05... fixup add vendor back
	// Turn empty string into "/"
	if p == "" {
		return "/"
	}

<<<<<<< HEAD
	// Reasonably sized buffer on stack to avoid allocations in the common case.
	// If a larger buffer is required, it gets allocated dynamically.
	buf := make([]byte, 0, stackBufSize)

	n := len(p)
=======
	n := len(p)
	var buf []byte
>>>>>>> cbc9bb05... fixup add vendor back

	// Invariants:
	//      reading from path; r is index of next byte to process.
	//      writing to buf; w is index of next byte to write.

	// path must start with '/'
	r := 1
	w := 1

	if p[0] != '/' {
		r = 0
<<<<<<< HEAD

		if n+1 > stackBufSize {
			buf = make([]byte, n+1)
		} else {
			buf = buf[:n+1]
		}
=======
		buf = make([]byte, n+1)
>>>>>>> cbc9bb05... fixup add vendor back
		buf[0] = '/'
	}

	trailing := n > 1 && p[n-1] == '/'

	// A bit more clunky without a 'lazybuf' like the path package, but the loop
<<<<<<< HEAD
	// gets completely inlined (bufApp calls).
	// loop has no expensive function calls (except 1x make)		// So in contrast to the path package this loop has no expensive function
	// calls (except make, if needed).
=======
	// gets completely inlined (bufApp). So in contrast to the path package this
	// loop has no expensive function calls (except 1x make)
>>>>>>> cbc9bb05... fixup add vendor back

	for r < n {
		switch {
		case p[r] == '/':
			// empty path element, trailing slash is added after the end
			r++

		case p[r] == '.' && r+1 == n:
			trailing = true
			r++

		case p[r] == '.' && p[r+1] == '/':
			// . element
			r += 2

		case p[r] == '.' && p[r+1] == '.' && (r+2 == n || p[r+2] == '/'):
			// .. element: remove to last /
			r += 3

			if w > 1 {
				// can backtrack
				w--

<<<<<<< HEAD
				if len(buf) == 0 {
=======
				if buf == nil {
>>>>>>> cbc9bb05... fixup add vendor back
					for w > 1 && p[w] != '/' {
						w--
					}
				} else {
					for w > 1 && buf[w] != '/' {
						w--
					}
				}
			}

		default:
<<<<<<< HEAD
			// Real path element.
			// Add slash if needed
=======
			// real path element.
			// add slash if needed
>>>>>>> cbc9bb05... fixup add vendor back
			if w > 1 {
				bufApp(&buf, p, w, '/')
				w++
			}

<<<<<<< HEAD
			// Copy element
=======
			// copy element
>>>>>>> cbc9bb05... fixup add vendor back
			for r < n && p[r] != '/' {
				bufApp(&buf, p, w, p[r])
				w++
				r++
			}
		}
	}

<<<<<<< HEAD
	// Re-append trailing slash
=======
	// re-append trailing slash
>>>>>>> cbc9bb05... fixup add vendor back
	if trailing && w > 1 {
		bufApp(&buf, p, w, '/')
		w++
	}

<<<<<<< HEAD
	// If the original string was not modified (or only shortened at the end),
	// return the respective substring of the original string.
	// Otherwise return a new string from the buffer.
	if len(buf) == 0 {
=======
	if buf == nil {
>>>>>>> cbc9bb05... fixup add vendor back
		return p[:w]
	}
	return string(buf[:w])
}

<<<<<<< HEAD
// Internal helper to lazily create a buffer if necessary.
// Calls to this function get inlined.
func bufApp(buf *[]byte, s string, w int, c byte) {
	b := *buf
	if len(b) == 0 {
		// No modification of the original string so far.
		// If the next character is the same as in the original string, we do
		// not yet have to allocate a buffer.
=======
// internal helper to lazily create a buffer if necessary.
func bufApp(buf *[]byte, s string, w int, c byte) {
	if *buf == nil {
>>>>>>> cbc9bb05... fixup add vendor back
		if s[w] == c {
			return
		}

<<<<<<< HEAD
		// Otherwise use either the stack buffer, if it is large enough, or
		// allocate a new buffer on the heap, and copy all previous characters.
		if l := len(s); l > cap(b) {
			*buf = make([]byte, len(s))
		} else {
			*buf = (*buf)[:l]
		}
		b = *buf

		copy(b, s[:w])
	}
	b[w] = c
=======
		*buf = make([]byte, len(s))
		copy(*buf, s[:w])
	}
	(*buf)[w] = c
>>>>>>> cbc9bb05... fixup add vendor back
}
