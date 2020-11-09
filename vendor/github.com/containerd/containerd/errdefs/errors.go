/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package errdefs defines the common errors used throughout containerd
// packages.
//
// Use with errors.Wrap and error.Wrapf to add context to an error.
//
// To detect an error class, use the IsXXX functions to tell whether an error
// is of a certain type.
//
// The functions ToGRPC and FromGRPC can be used to map server-side and
// client-side errors to the correct types.
package errdefs

import (
	"context"

	"github.com/pkg/errors"
)

// Definitions of common error types used throughout containerd. All containerd
// errors returned by most packages will map into one of these errors classes.
// Packages should return errors of these types when they want to instruct a
// client to take a particular action.
//
// For the most part, we just try to provide local grpc errors. Most conditions
// map very well to those defined by grpc.
var (
	ErrUnknown            = errors.New("unknown") // used internally to represent a missed mapping.
	ErrInvalidArgument    = errors.New("invalid argument")
	ErrNotFound           = errors.New("not found")
	ErrAlreadyExists      = errors.New("already exists")
	ErrFailedPrecondition = errors.New("failed precondition")
	ErrUnavailable        = errors.New("unavailable")
	ErrNotImplemented     = errors.New("not implemented") // represents not supported and unimplemented
)

// IsInvalidArgument returns true if the error is due to an invalid argument
func IsInvalidArgument(err error) bool {
<<<<<<< HEAD
	return errors.Cause(err) == ErrInvalidArgument
=======
	return errors.Is(err, ErrInvalidArgument)
>>>>>>> cbc9bb05... fixup add vendor back
}

// IsNotFound returns true if the error is due to a missing object
func IsNotFound(err error) bool {
<<<<<<< HEAD
	return errors.Cause(err) == ErrNotFound
=======
	return errors.Is(err, ErrNotFound)
>>>>>>> cbc9bb05... fixup add vendor back
}

// IsAlreadyExists returns true if the error is due to an already existing
// metadata item
func IsAlreadyExists(err error) bool {
<<<<<<< HEAD
	return errors.Cause(err) == ErrAlreadyExists
=======
	return errors.Is(err, ErrAlreadyExists)
>>>>>>> cbc9bb05... fixup add vendor back
}

// IsFailedPrecondition returns true if an operation could not proceed to the
// lack of a particular condition
func IsFailedPrecondition(err error) bool {
<<<<<<< HEAD
	return errors.Cause(err) == ErrFailedPrecondition
=======
	return errors.Is(err, ErrFailedPrecondition)
>>>>>>> cbc9bb05... fixup add vendor back
}

// IsUnavailable returns true if the error is due to a resource being unavailable
func IsUnavailable(err error) bool {
<<<<<<< HEAD
	return errors.Cause(err) == ErrUnavailable
=======
	return errors.Is(err, ErrUnavailable)
>>>>>>> cbc9bb05... fixup add vendor back
}

// IsNotImplemented returns true if the error is due to not being implemented
func IsNotImplemented(err error) bool {
<<<<<<< HEAD
	return errors.Cause(err) == ErrNotImplemented
=======
	return errors.Is(err, ErrNotImplemented)
>>>>>>> cbc9bb05... fixup add vendor back
}

// IsCanceled returns true if the error is due to `context.Canceled`.
func IsCanceled(err error) bool {
<<<<<<< HEAD
	return errors.Cause(err) == context.Canceled
=======
	return errors.Is(err, context.Canceled)
>>>>>>> cbc9bb05... fixup add vendor back
}

// IsDeadlineExceeded returns true if the error is due to
// `context.DeadlineExceeded`.
func IsDeadlineExceeded(err error) bool {
<<<<<<< HEAD
	return errors.Cause(err) == context.DeadlineExceeded
=======
	return errors.Is(err, context.DeadlineExceeded)
>>>>>>> cbc9bb05... fixup add vendor back
}
