// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

<<<<<<< HEAD
// +build !amd64,!ppc64le,!s390x gccgo purego
=======
// +build !amd64 gccgo appengine
>>>>>>> cbc9bb05... fixup add vendor back

package poly1305

type mac struct{ macGeneric }
<<<<<<< HEAD
=======

func newMAC(key *[32]byte) mac { return mac{newMACGeneric(key)} }
>>>>>>> cbc9bb05... fixup add vendor back
