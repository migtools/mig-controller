/*
 *
 * Copyright 2017 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package grpc

import (
	"context"
	"io"
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/codes"
<<<<<<< HEAD
=======
	"google.golang.org/grpc/grpclog"
>>>>>>> cbc9bb05... fixup add vendor back
	"google.golang.org/grpc/internal/channelz"
	"google.golang.org/grpc/internal/transport"
	"google.golang.org/grpc/status"
)

// pickerWrapper is a wrapper of balancer.Picker. It blocks on certain pick
// actions and unblock when there's a picker update.
type pickerWrapper struct {
	mu         sync.Mutex
	done       bool
	blockingCh chan struct{}
	picker     balancer.Picker
<<<<<<< HEAD
}

func newPickerWrapper() *pickerWrapper {
	return &pickerWrapper{blockingCh: make(chan struct{})}
}

// updatePicker is called by UpdateBalancerState. It unblocks all blocked pick.
func (pw *pickerWrapper) updatePicker(p balancer.Picker) {
	pw.mu.Lock()
	if pw.done {
		pw.mu.Unlock()
		return
	}
	pw.picker = p
	// pw.blockingCh should never be nil.
	close(pw.blockingCh)
	pw.blockingCh = make(chan struct{})
	pw.mu.Unlock()
=======

	// The latest connection happened.
	connErrMu sync.Mutex
	connErr   error
}

func newPickerWrapper() *pickerWrapper {
	bp := &pickerWrapper{blockingCh: make(chan struct{})}
	return bp
}

func (bp *pickerWrapper) updateConnectionError(err error) {
	bp.connErrMu.Lock()
	bp.connErr = err
	bp.connErrMu.Unlock()
}

func (bp *pickerWrapper) connectionError() error {
	bp.connErrMu.Lock()
	err := bp.connErr
	bp.connErrMu.Unlock()
	return err
}

// updatePicker is called by UpdateBalancerState. It unblocks all blocked pick.
func (bp *pickerWrapper) updatePicker(p balancer.Picker) {
	bp.mu.Lock()
	if bp.done {
		bp.mu.Unlock()
		return
	}
	bp.picker = p
	// bp.blockingCh should never be nil.
	close(bp.blockingCh)
	bp.blockingCh = make(chan struct{})
	bp.mu.Unlock()
>>>>>>> cbc9bb05... fixup add vendor back
}

func doneChannelzWrapper(acw *acBalancerWrapper, done func(balancer.DoneInfo)) func(balancer.DoneInfo) {
	acw.mu.Lock()
	ac := acw.ac
	acw.mu.Unlock()
	ac.incrCallsStarted()
	return func(b balancer.DoneInfo) {
		if b.Err != nil && b.Err != io.EOF {
			ac.incrCallsFailed()
		} else {
			ac.incrCallsSucceeded()
		}
		if done != nil {
			done(b)
		}
	}
}

// pick returns the transport that will be used for the RPC.
// It may block in the following cases:
// - there's no picker
// - the current picker returns ErrNoSubConnAvailable
// - the current picker returns other errors and failfast is false.
// - the subConn returned by the current picker is not READY
// When one of these situations happens, pick blocks until the picker gets updated.
<<<<<<< HEAD
func (pw *pickerWrapper) pick(ctx context.Context, failfast bool, info balancer.PickInfo) (transport.ClientTransport, func(balancer.DoneInfo), error) {
	var ch chan struct{}

	var lastPickErr error
	for {
		pw.mu.Lock()
		if pw.done {
			pw.mu.Unlock()
			return nil, nil, ErrClientConnClosing
		}

		if pw.picker == nil {
			ch = pw.blockingCh
		}
		if ch == pw.blockingCh {
			// This could happen when either:
			// - pw.picker is nil (the previous if condition), or
			// - has called pick on the current picker.
			pw.mu.Unlock()
			select {
			case <-ctx.Done():
				var errStr string
				if lastPickErr != nil {
					errStr = "latest balancer error: " + lastPickErr.Error()
				} else {
					errStr = ctx.Err().Error()
				}
				switch ctx.Err() {
				case context.DeadlineExceeded:
					return nil, nil, status.Error(codes.DeadlineExceeded, errStr)
				case context.Canceled:
					return nil, nil, status.Error(codes.Canceled, errStr)
				}
=======
func (bp *pickerWrapper) pick(ctx context.Context, failfast bool, opts balancer.PickOptions) (transport.ClientTransport, func(balancer.DoneInfo), error) {
	var ch chan struct{}

	for {
		bp.mu.Lock()
		if bp.done {
			bp.mu.Unlock()
			return nil, nil, ErrClientConnClosing
		}

		if bp.picker == nil {
			ch = bp.blockingCh
		}
		if ch == bp.blockingCh {
			// This could happen when either:
			// - bp.picker is nil (the previous if condition), or
			// - has called pick on the current picker.
			bp.mu.Unlock()
			select {
			case <-ctx.Done():
				if connectionErr := bp.connectionError(); connectionErr != nil {
					switch ctx.Err() {
					case context.DeadlineExceeded:
						return nil, nil, status.Errorf(codes.DeadlineExceeded, "latest connection error: %v", connectionErr)
					case context.Canceled:
						return nil, nil, status.Errorf(codes.Canceled, "latest connection error: %v", connectionErr)
					}
				}
				return nil, nil, ctx.Err()
>>>>>>> cbc9bb05... fixup add vendor back
			case <-ch:
			}
			continue
		}

<<<<<<< HEAD
		ch = pw.blockingCh
		p := pw.picker
		pw.mu.Unlock()

		pickResult, err := p.Pick(info)

		if err != nil {
			if err == balancer.ErrNoSubConnAvailable {
				continue
			}
			if _, ok := status.FromError(err); ok {
				// Status error: end the RPC unconditionally with this status.
				return nil, nil, err
			}
			// For all other errors, wait for ready RPCs should block and other
			// RPCs should fail with unavailable.
			if !failfast {
				lastPickErr = err
				continue
			}
			return nil, nil, status.Error(codes.Unavailable, err.Error())
		}

		acw, ok := pickResult.SubConn.(*acBalancerWrapper)
		if !ok {
			logger.Error("subconn returned from pick is not *acBalancerWrapper")
=======
		ch = bp.blockingCh
		p := bp.picker
		bp.mu.Unlock()

		subConn, done, err := p.Pick(ctx, opts)

		if err != nil {
			switch err {
			case balancer.ErrNoSubConnAvailable:
				continue
			case balancer.ErrTransientFailure:
				if !failfast {
					continue
				}
				return nil, nil, status.Errorf(codes.Unavailable, "%v, latest connection error: %v", err, bp.connectionError())
			case context.DeadlineExceeded:
				return nil, nil, status.Error(codes.DeadlineExceeded, err.Error())
			case context.Canceled:
				return nil, nil, status.Error(codes.Canceled, err.Error())
			default:
				if _, ok := status.FromError(err); ok {
					return nil, nil, err
				}
				// err is some other error.
				return nil, nil, status.Error(codes.Unknown, err.Error())
			}
		}

		acw, ok := subConn.(*acBalancerWrapper)
		if !ok {
			grpclog.Error("subconn returned from pick is not *acBalancerWrapper")
>>>>>>> cbc9bb05... fixup add vendor back
			continue
		}
		if t, ok := acw.getAddrConn().getReadyTransport(); ok {
			if channelz.IsOn() {
<<<<<<< HEAD
				return t, doneChannelzWrapper(acw, pickResult.Done), nil
			}
			return t, pickResult.Done, nil
		}
		if pickResult.Done != nil {
			// Calling done with nil error, no bytes sent and no bytes received.
			// DoneInfo with default value works.
			pickResult.Done(balancer.DoneInfo{})
		}
		logger.Infof("blockingPicker: the picked transport is not ready, loop back to repick")
=======
				return t, doneChannelzWrapper(acw, done), nil
			}
			return t, done, nil
		}
		if done != nil {
			// Calling done with nil error, no bytes sent and no bytes received.
			// DoneInfo with default value works.
			done(balancer.DoneInfo{})
		}
		grpclog.Infof("blockingPicker: the picked transport is not ready, loop back to repick")
>>>>>>> cbc9bb05... fixup add vendor back
		// If ok == false, ac.state is not READY.
		// A valid picker always returns READY subConn. This means the state of ac
		// just changed, and picker will be updated shortly.
		// continue back to the beginning of the for loop to repick.
	}
}

<<<<<<< HEAD
func (pw *pickerWrapper) close() {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	if pw.done {
		return
	}
	pw.done = true
	close(pw.blockingCh)
=======
func (bp *pickerWrapper) close() {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	if bp.done {
		return
	}
	bp.done = true
	close(bp.blockingCh)
>>>>>>> cbc9bb05... fixup add vendor back
}
