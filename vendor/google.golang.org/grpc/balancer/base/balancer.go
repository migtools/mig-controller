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

package base

import (
<<<<<<< HEAD
	"errors"
	"fmt"
=======
	"context"
>>>>>>> cbc9bb05... fixup add vendor back

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/resolver"
)

<<<<<<< HEAD
var logger = grpclog.Component("balancer")

=======
>>>>>>> cbc9bb05... fixup add vendor back
type baseBuilder struct {
	name          string
	pickerBuilder PickerBuilder
	config        Config
}

func (bb *baseBuilder) Build(cc balancer.ClientConn, opt balancer.BuildOptions) balancer.Balancer {
<<<<<<< HEAD
	bal := &baseBalancer{
=======
	return &baseBalancer{
>>>>>>> cbc9bb05... fixup add vendor back
		cc:            cc,
		pickerBuilder: bb.pickerBuilder,

		subConns: make(map[resolver.Address]balancer.SubConn),
		scStates: make(map[balancer.SubConn]connectivity.State),
		csEvltr:  &balancer.ConnectivityStateEvaluator{},
<<<<<<< HEAD
		config:   bb.config,
	}
	// Initialize picker to a picker that always returns
	// ErrNoSubConnAvailable, because when state of a SubConn changes, we
	// may call UpdateState with this picker.
	bal.picker = NewErrPicker(balancer.ErrNoSubConnAvailable)
	return bal
=======
		// Initialize picker to a picker that always return
		// ErrNoSubConnAvailable, because when state of a SubConn changes, we
		// may call UpdateBalancerState with this picker.
		picker: NewErrPicker(balancer.ErrNoSubConnAvailable),
		config: bb.config,
	}
>>>>>>> cbc9bb05... fixup add vendor back
}

func (bb *baseBuilder) Name() string {
	return bb.name
}

type baseBalancer struct {
	cc            balancer.ClientConn
	pickerBuilder PickerBuilder

	csEvltr *balancer.ConnectivityStateEvaluator
	state   connectivity.State

	subConns map[resolver.Address]balancer.SubConn
	scStates map[balancer.SubConn]connectivity.State
	picker   balancer.Picker
	config   Config
<<<<<<< HEAD

	resolverErr error // the last error reported by the resolver; cleared on successful resolution
	connErr     error // the last connection error; cleared upon leaving TransientFailure
}

func (b *baseBalancer) ResolverError(err error) {
	b.resolverErr = err
	if len(b.subConns) == 0 {
		b.state = connectivity.TransientFailure
	}

	if b.state != connectivity.TransientFailure {
		// The picker will not change since the balancer does not currently
		// report an error.
		return
	}
	b.regeneratePicker()
	b.cc.UpdateState(balancer.State{
		ConnectivityState: b.state,
		Picker:            b.picker,
	})
}

func (b *baseBalancer) UpdateClientConnState(s balancer.ClientConnState) error {
	// TODO: handle s.ResolverState.ServiceConfig?
	if logger.V(2) {
		logger.Info("base.baseBalancer: got new ClientConn state: ", s)
	}
	// Successful resolution; clear resolver error and ensure we return nil.
	b.resolverErr = nil
=======
}

func (b *baseBalancer) HandleResolvedAddrs(addrs []resolver.Address, err error) {
	panic("not implemented")
}

func (b *baseBalancer) UpdateClientConnState(s balancer.ClientConnState) {
	// TODO: handle s.ResolverState.Err (log if not nil) once implemented.
	// TODO: handle s.ResolverState.ServiceConfig?
	if grpclog.V(2) {
		grpclog.Infoln("base.baseBalancer: got new ClientConn state: ", s)
	}
>>>>>>> cbc9bb05... fixup add vendor back
	// addrsSet is the set converted from addrs, it's used for quick lookup of an address.
	addrsSet := make(map[resolver.Address]struct{})
	for _, a := range s.ResolverState.Addresses {
		addrsSet[a] = struct{}{}
		if _, ok := b.subConns[a]; !ok {
			// a is a new address (not existing in b.subConns).
			sc, err := b.cc.NewSubConn([]resolver.Address{a}, balancer.NewSubConnOptions{HealthCheckEnabled: b.config.HealthCheck})
			if err != nil {
<<<<<<< HEAD
				logger.Warningf("base.baseBalancer: failed to create new SubConn: %v", err)
=======
				grpclog.Warningf("base.baseBalancer: failed to create new SubConn: %v", err)
>>>>>>> cbc9bb05... fixup add vendor back
				continue
			}
			b.subConns[a] = sc
			b.scStates[sc] = connectivity.Idle
			sc.Connect()
		}
	}
	for a, sc := range b.subConns {
		// a was removed by resolver.
		if _, ok := addrsSet[a]; !ok {
			b.cc.RemoveSubConn(sc)
			delete(b.subConns, a)
			// Keep the state of this sc in b.scStates until sc's state becomes Shutdown.
<<<<<<< HEAD
			// The entry will be deleted in UpdateSubConnState.
		}
	}
	// If resolver state contains no addresses, return an error so ClientConn
	// will trigger re-resolve. Also records this as an resolver error, so when
	// the overall state turns transient failure, the error message will have
	// the zero address information.
	if len(s.ResolverState.Addresses) == 0 {
		b.ResolverError(errors.New("produced zero addresses"))
		return balancer.ErrBadResolverState
	}
	return nil
}

// mergeErrors builds an error from the last connection error and the last
// resolver error.  Must only be called if b.state is TransientFailure.
func (b *baseBalancer) mergeErrors() error {
	// connErr must always be non-nil unless there are no SubConns, in which
	// case resolverErr must be non-nil.
	if b.connErr == nil {
		return fmt.Errorf("last resolver error: %v", b.resolverErr)
	}
	if b.resolverErr == nil {
		return fmt.Errorf("last connection error: %v", b.connErr)
	}
	return fmt.Errorf("last connection error: %v; last resolver error: %v", b.connErr, b.resolverErr)
=======
			// The entry will be deleted in HandleSubConnStateChange.
		}
	}
>>>>>>> cbc9bb05... fixup add vendor back
}

// regeneratePicker takes a snapshot of the balancer, and generates a picker
// from it. The picker is
<<<<<<< HEAD
//  - errPicker if the balancer is in TransientFailure,
//  - built by the pickerBuilder with all READY SubConns otherwise.
func (b *baseBalancer) regeneratePicker() {
	if b.state == connectivity.TransientFailure {
		b.picker = NewErrPicker(b.mergeErrors())
		return
	}
	readySCs := make(map[balancer.SubConn]SubConnInfo)
=======
//  - errPicker with ErrTransientFailure if the balancer is in TransientFailure,
//  - built by the pickerBuilder with all READY SubConns otherwise.
func (b *baseBalancer) regeneratePicker() {
	if b.state == connectivity.TransientFailure {
		b.picker = NewErrPicker(balancer.ErrTransientFailure)
		return
	}
	readySCs := make(map[resolver.Address]balancer.SubConn)
>>>>>>> cbc9bb05... fixup add vendor back

	// Filter out all ready SCs from full subConn map.
	for addr, sc := range b.subConns {
		if st, ok := b.scStates[sc]; ok && st == connectivity.Ready {
<<<<<<< HEAD
			readySCs[sc] = SubConnInfo{Address: addr}
		}
	}
	b.picker = b.pickerBuilder.Build(PickerBuildInfo{ReadySCs: readySCs})
=======
			readySCs[addr] = sc
		}
	}
	b.picker = b.pickerBuilder.Build(readySCs)
}

func (b *baseBalancer) HandleSubConnStateChange(sc balancer.SubConn, s connectivity.State) {
	panic("not implemented")
>>>>>>> cbc9bb05... fixup add vendor back
}

func (b *baseBalancer) UpdateSubConnState(sc balancer.SubConn, state balancer.SubConnState) {
	s := state.ConnectivityState
<<<<<<< HEAD
	if logger.V(2) {
		logger.Infof("base.baseBalancer: handle SubConn state change: %p, %v", sc, s)
	}
	oldS, ok := b.scStates[sc]
	if !ok {
		if logger.V(2) {
			logger.Infof("base.baseBalancer: got state changes for an unknown SubConn: %p, %v", sc, s)
		}
		return
	}
	if oldS == connectivity.TransientFailure && s == connectivity.Connecting {
		// Once a subconn enters TRANSIENT_FAILURE, ignore subsequent
		// CONNECTING transitions to prevent the aggregated state from being
		// always CONNECTING when many backends exist but are all down.
		return
	}
=======
	if grpclog.V(2) {
		grpclog.Infof("base.baseBalancer: handle SubConn state change: %p, %v", sc, s)
	}
	oldS, ok := b.scStates[sc]
	if !ok {
		if grpclog.V(2) {
			grpclog.Infof("base.baseBalancer: got state changes for an unknown SubConn: %p, %v", sc, s)
		}
		return
	}
>>>>>>> cbc9bb05... fixup add vendor back
	b.scStates[sc] = s
	switch s {
	case connectivity.Idle:
		sc.Connect()
	case connectivity.Shutdown:
		// When an address was removed by resolver, b called RemoveSubConn but
		// kept the sc's state in scStates. Remove state for this sc here.
		delete(b.scStates, sc)
<<<<<<< HEAD
	case connectivity.TransientFailure:
		// Save error to be reported via picker.
		b.connErr = state.ConnectionError
	}

	b.state = b.csEvltr.RecordTransition(oldS, s)

	// Regenerate picker when one of the following happens:
	//  - this sc entered or left ready
	//  - the aggregated state of balancer is TransientFailure
	//    (may need to update error message)
	if (s == connectivity.Ready) != (oldS == connectivity.Ready) ||
		b.state == connectivity.TransientFailure {
		b.regeneratePicker()
	}

	b.cc.UpdateState(balancer.State{ConnectivityState: b.state, Picker: b.picker})
=======
	}

	oldAggrState := b.state
	b.state = b.csEvltr.RecordTransition(oldS, s)

	// Regenerate picker when one of the following happens:
	//  - this sc became ready from not-ready
	//  - this sc became not-ready from ready
	//  - the aggregated state of balancer became TransientFailure from non-TransientFailure
	//  - the aggregated state of balancer became non-TransientFailure from TransientFailure
	if (s == connectivity.Ready) != (oldS == connectivity.Ready) ||
		(b.state == connectivity.TransientFailure) != (oldAggrState == connectivity.TransientFailure) {
		b.regeneratePicker()
	}

	b.cc.UpdateBalancerState(b.state, b.picker)
>>>>>>> cbc9bb05... fixup add vendor back
}

// Close is a nop because base balancer doesn't have internal state to clean up,
// and it doesn't need to call RemoveSubConn for the SubConns.
func (b *baseBalancer) Close() {
}

<<<<<<< HEAD
// NewErrPicker returns a Picker that always returns err on Pick().
=======
// NewErrPicker returns a picker that always returns err on Pick().
>>>>>>> cbc9bb05... fixup add vendor back
func NewErrPicker(err error) balancer.Picker {
	return &errPicker{err: err}
}

<<<<<<< HEAD
// NewErrPickerV2 is temporarily defined for backward compatibility reasons.
//
// Deprecated: use NewErrPicker instead.
var NewErrPickerV2 = NewErrPicker

=======
>>>>>>> cbc9bb05... fixup add vendor back
type errPicker struct {
	err error // Pick() always returns this err.
}

<<<<<<< HEAD
func (p *errPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	return balancer.PickResult{}, p.err
=======
func (p *errPicker) Pick(ctx context.Context, opts balancer.PickOptions) (balancer.SubConn, func(balancer.DoneInfo), error) {
	return nil, nil, p.err
>>>>>>> cbc9bb05... fixup add vendor back
}
