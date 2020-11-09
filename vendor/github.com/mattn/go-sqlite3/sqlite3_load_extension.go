// Copyright (C) 2019 Yasuhiro Matsumoto <mattn.jp@gmail.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// +build !sqlite_omit_load_extension

package sqlite3

/*
#ifndef USE_LIBSQLITE3
#include <sqlite3-binding.h>
#else
#include <sqlite3.h>
#endif
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"unsafe"
)

func (c *SQLiteConn) loadExtensions(extensions []string) error {
	rv := C.sqlite3_enable_load_extension(c.db, 1)
	if rv != C.SQLITE_OK {
		return errors.New(C.GoString(C.sqlite3_errmsg(c.db)))
	}

	for _, extension := range extensions {
<<<<<<< HEAD
		if err := c.loadExtension(extension, nil); err != nil {
			C.sqlite3_enable_load_extension(c.db, 0)
			return err
=======
		cext := C.CString(extension)
		defer C.free(unsafe.Pointer(cext))
		rv = C.sqlite3_load_extension(c.db, cext, nil, nil)
		if rv != C.SQLITE_OK {
			C.sqlite3_enable_load_extension(c.db, 0)
			return errors.New(C.GoString(C.sqlite3_errmsg(c.db)))
>>>>>>> cbc9bb05... fixup add vendor back
		}
	}

	rv = C.sqlite3_enable_load_extension(c.db, 0)
	if rv != C.SQLITE_OK {
		return errors.New(C.GoString(C.sqlite3_errmsg(c.db)))
	}
<<<<<<< HEAD

=======
>>>>>>> cbc9bb05... fixup add vendor back
	return nil
}

// LoadExtension load the sqlite3 extension.
func (c *SQLiteConn) LoadExtension(lib string, entry string) error {
	rv := C.sqlite3_enable_load_extension(c.db, 1)
	if rv != C.SQLITE_OK {
		return errors.New(C.GoString(C.sqlite3_errmsg(c.db)))
	}

<<<<<<< HEAD
	if err := c.loadExtension(lib, &entry); err != nil {
		C.sqlite3_enable_load_extension(c.db, 0)
		return err
	}

	rv = C.sqlite3_enable_load_extension(c.db, 0)
	if rv != C.SQLITE_OK {
		return errors.New(C.GoString(C.sqlite3_errmsg(c.db)))
	}

	return nil
}

func (c *SQLiteConn) loadExtension(lib string, entry *string) error {
	clib := C.CString(lib)
	defer C.free(unsafe.Pointer(clib))

	var centry *C.char
	if entry != nil {
		centry = C.CString(*entry)
		defer C.free(unsafe.Pointer(centry))
	}

	var errMsg *C.char
	defer C.sqlite3_free(unsafe.Pointer(errMsg))

	rv := C.sqlite3_load_extension(c.db, clib, centry, &errMsg)
	if rv != C.SQLITE_OK {
		return errors.New(C.GoString(errMsg))
=======
	clib := C.CString(lib)
	defer C.free(unsafe.Pointer(clib))
	centry := C.CString(entry)
	defer C.free(unsafe.Pointer(centry))

	rv = C.sqlite3_load_extension(c.db, clib, centry, nil)
	if rv != C.SQLITE_OK {
		return errors.New(C.GoString(C.sqlite3_errmsg(c.db)))
	}

	rv = C.sqlite3_enable_load_extension(c.db, 0)
	if rv != C.SQLITE_OK {
		return errors.New(C.GoString(C.sqlite3_errmsg(c.db)))
>>>>>>> cbc9bb05... fixup add vendor back
	}

	return nil
}
