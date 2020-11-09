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
<<<<<<< HEAD
	"errors"
	"fmt"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
=======
	"context"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/resolver"
>>>>>>> cbc9bb05... fixup add vendor back
)

// PickFirstBalancerName is the name of the pick_first balancer.
const PickFirstBalancerName = "pick_first"

func newPickfirstBuilder() balancer.Builder {
	return &pickfirstBuilder{}
}

type pickfirstBuilder struct{}

func (*pickfirstBuilder) Build(cc balancer.ClientConn, opt balancer.BuildOptions) balancer.Balancer {
	return &pickfirstBalancer{cc: cc}
}

func (*pickfirstBuilder) Name() string {
	return PickFirstBalancerName
}

type pickfirstBalancer struct {
<<<<<<< HEAD
	state connectivity.State
	cc    balancer.ClientConn
	sc    balancer.SubConn
}

func (b *pickfirstBalancer) ResolverError(err error) {
	switch b.state {
	case connectivity.TransientFailure, connectivity.Idle, connectivity.Connecting:
		// Set a failing picker if we don't have a good picker.
		b.cc.UpdateState(balancer.State{ConnectivityState: connectivity.TransientFailure,
			Picker: &picker{err: fmt.Errorf("name resolver error: %v", err)},
		})
	}
	if logger.V(2) {
		logger.Infof("pickfirstBalancer: ResolverError called with error %v", err)
	}
}

func (b *pickfirstBalancer) UpdateClientConnState(cs balancer.ClientConnState) error {
	if len(cs.ResolverState.Addresses) == 0 {
		b.ResolverError(errors.New("produced zero addresses"))
		return balancer.ErrBadResolverState
	}
	if b.sc == nil {
		var err error
		b.sc, err = b.cc.NewSubConn(cs.ResolverState.Addresses, balancer.NewSubConnOptions{})
		if err != nil {
			if logger.V(2) {
				logger.Errorf("pickfirstBalancer: failed to NewSubConn: %v", err)
			}
			b.state = connectivity.TransientFailure
			b.cc.UpdateState(balancer.State{ConnectivityState: connectivity.TransientFailure,
				Picker: &picker{err: fmt.Errorf("error creating connection: %v", err)},
			})
			return balancer.ErrBadResolverState
		}
		b.state = connectivity.Idle
		b.cc.UpdateState(balancer.State{ConnectivityState: connectivity.Idle, Picker: &picker{result: balancer.PickResult{SubConn: b.sc}}})
		b.sc.Connect()
	} else {
		b.sc.UpdateAddresses(cs.ResolverState.Addresses)
		b.sc.Connect()
	}
	return nil
}

func (b *pickfirstBalancer) UpdateSubConnState(sc balancer.SubConn, s balancer.SubConnState) {
	if logger.V(2) {
		logger.Infof("pickfirstBalancer: UpdateSubConnState: %p, %v", sc, s)
	}
	if b.sc != sc {
		if logger.V(2) {
			logger.Infof("pickfirstBalancer: ignored state change because sc is not recognized")
		}
		return
	}
	b.state = s.ConnectivityState
	if s.ConnectivityState == connectivity.Shutdown {
=======
	cc balancer.ClientConn
	sc balancer.SubConn
}

func (b *pickfirstBalancer) HandleResolvedAddrs(addrs []resolver.Address, err error) {
	if err != nil {
		if grpclog.V(2) {
			grpclog.Infof("pickfirstBalancer: HandleResolvedAddrs called with error %v", err)
		}
		return
	}
	if b.sc == nil {
		b.sc, err = b.cc.NewSubConn(addrs, balancer.NewSubConnOptions{})
		if err != nil {
			//TODO(yuxuanli): why not change the cc state to Idle?
			if grpclog.V(2) {
				grpclog.Errorf("pickfirstBalancer: failed to NewSubConn: %v", err)
			}
			return
		}
		b.cc.UpdateBalancerState(connectivity.Idle, &picker{sc: b.sc})
		b.sc.Connect()
	} else {
		b.sc.UpdateAddresses(addrs)
		b.sc.Connect()
	}
}

func (b *pickfirstBalancer) HandleSubConnStateChange(sc balancer.SubConn, s connectivity.State) {
	if grpclog.V(2) {
		grpclog.Infof("pickfirstBalancer: HandleSubConnStateChange: %p, %v", sc, s)
	}
	if b.sc != sc {
		if grpclog.V(2) {
			grpclog.Infof("pickfirstBalancer: ignored state change because sc is not recognized")
		}
		return
	}
	if s == connectivity.Shutdown {
>>>>>>> cbc9bb05... fixup add vendor back
		b.sc = nil
		return
	}

<<<<<<< HEAD
	switch s.ConnectivityState {
	case connectivity.Ready, connectivity.Idle:
		b.cc.UpdateState(balancer.State{ConnectivityState: s.ConnectivityState, Picker: &picker{result: balancer.PickResult{SubConn: sc}}})
	case connectivity.Connecting:
		b.cc.UpdateState(balancer.State{ConnectivityState: s.ConnectivityState, Picker: &picker{err: balancer.ErrNoSubConnAvailable}})
	case connectivity.TransientFailure:
		b.cc.UpdateState(balancer.State{
			ConnectivityState: s.ConnectivityState,
			Picker:            &picker{err: s.ConnectionError},
		})
=======
	switch s {
	case connectivity.Ready, connectivity.Idle:
		b.cc.UpdateBalancerState(s, &picker{sc: sc})
	case connectivity.Connecting:
		b.cc.UpdateBalancerState(s, &picker{err: balancer.ErrNoSubConnAvailable})
	case connectivity.TransientFailure:
		b.cc.UpdateBalancerState(s, &picker{err: balancer.ErrTransientFailure})
>>>>>>> cbc9bb05... fixup add vendor back
	}
}

func (b *pickfirstBalancer) Close() {
}

type picker struct {
<<<<<<< HEAD
	result balancer.PickResult
	err    error
}

func (p *picker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	return p.result, p.err
=======
	err error
	sc  balancer.SubConn
}

func (p *picker) Pick(ctx context.Context, opts balancer.PickOptions) (balancer.SubConn, func(balancer.DoneInfo), error) {
	if p.err != nil {
		return nil, nil, p.err
	}
	return p.sc, nil, nil
>>>>>>> cbc9bb05... fixup add vendor back
}

func init() {
	balancer.Register(newPickfirstBuilder())
}
