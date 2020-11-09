<<<<<<< HEAD
// Copyright 2015 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
=======
// Copyright 2015 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
>>>>>>> cbc9bb05... fixup add vendor back

// Package http supports network connections to HTTP servers.
// This package is not intended for use by end developers. Use the
// google.golang.org/api/option package to configure API clients.
package http

import (
	"context"
<<<<<<< HEAD
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"time"
=======
	"errors"
	"net/http"
>>>>>>> cbc9bb05... fixup add vendor back

	"go.opencensus.io/plugin/ochttp"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi/transport"
	"google.golang.org/api/internal"
	"google.golang.org/api/option"
<<<<<<< HEAD
	"google.golang.org/api/transport/cert"
	"google.golang.org/api/transport/http/internal/propagation"
	"google.golang.org/api/transport/internal/dca"
=======
	"google.golang.org/api/transport/http/internal/propagation"
>>>>>>> cbc9bb05... fixup add vendor back
)

// NewClient returns an HTTP client for use communicating with a Google cloud
// service, configured with the given ClientOptions. It also returns the endpoint
// for the service as specified in the options.
func NewClient(ctx context.Context, opts ...option.ClientOption) (*http.Client, string, error) {
	settings, err := newSettings(opts)
	if err != nil {
		return nil, "", err
	}
<<<<<<< HEAD
	clientCertSource, endpoint, err := dca.GetClientCertificateSourceAndEndpoint(settings)
	if err != nil {
		return nil, "", err
	}
	// TODO(cbro): consider injecting the User-Agent even if an explicit HTTP client is provided?
	if settings.HTTPClient != nil {
		return settings.HTTPClient, endpoint, nil
	}
	trans, err := newTransport(ctx, defaultBaseTransport(ctx, clientCertSource), settings)
	if err != nil {
		return nil, "", err
	}
	return &http.Client{Transport: trans}, endpoint, nil
=======
	// TODO(cbro): consider injecting the User-Agent even if an explicit HTTP client is provided?
	if settings.HTTPClient != nil {
		return settings.HTTPClient, settings.Endpoint, nil
	}
	trans, err := newTransport(ctx, defaultBaseTransport(ctx), settings)
	if err != nil {
		return nil, "", err
	}
	return &http.Client{Transport: trans}, settings.Endpoint, nil
>>>>>>> cbc9bb05... fixup add vendor back
}

// NewTransport creates an http.RoundTripper for use communicating with a Google
// cloud service, configured with the given ClientOptions. Its RoundTrip method delegates to base.
func NewTransport(ctx context.Context, base http.RoundTripper, opts ...option.ClientOption) (http.RoundTripper, error) {
	settings, err := newSettings(opts)
	if err != nil {
		return nil, err
	}
	if settings.HTTPClient != nil {
		return nil, errors.New("transport/http: WithHTTPClient passed to NewTransport")
	}
	return newTransport(ctx, base, settings)
}

func newTransport(ctx context.Context, base http.RoundTripper, settings *internal.DialSettings) (http.RoundTripper, error) {
<<<<<<< HEAD
	paramTransport := &parameterTransport{
		base:          base,
=======
	trans := base
	trans = parameterTransport{
		base:          trans,
>>>>>>> cbc9bb05... fixup add vendor back
		userAgent:     settings.UserAgent,
		quotaProject:  settings.QuotaProject,
		requestReason: settings.RequestReason,
	}
<<<<<<< HEAD
	var trans http.RoundTripper = paramTransport
	trans = addOCTransport(trans, settings)
=======
	trans = addOCTransport(trans)
>>>>>>> cbc9bb05... fixup add vendor back
	switch {
	case settings.NoAuth:
		// Do nothing.
	case settings.APIKey != "":
		trans = &transport.APIKey{
			Transport: trans,
			Key:       settings.APIKey,
		}
	default:
		creds, err := internal.Creds(ctx, settings)
		if err != nil {
			return nil, err
		}
<<<<<<< HEAD
		if paramTransport.quotaProject == "" {
			paramTransport.quotaProject = internal.QuotaProjectFromCreds(creds)
		}

		ts := creds.TokenSource
		if settings.TokenSource != nil {
			ts = settings.TokenSource
		}
		trans = &oauth2.Transport{
			Base:   trans,
			Source: ts,
=======
		trans = &oauth2.Transport{
			Base:   trans,
			Source: creds.TokenSource,
>>>>>>> cbc9bb05... fixup add vendor back
		}
	}
	return trans, nil
}

func newSettings(opts []option.ClientOption) (*internal.DialSettings, error) {
	var o internal.DialSettings
	for _, opt := range opts {
		opt.Apply(&o)
	}
	if err := o.Validate(); err != nil {
		return nil, err
	}
	if o.GRPCConn != nil {
		return nil, errors.New("unsupported gRPC connection specified")
	}
	return &o, nil
}

type parameterTransport struct {
	userAgent     string
	quotaProject  string
	requestReason string

	base http.RoundTripper
}

<<<<<<< HEAD
func (t *parameterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
=======
func (t parameterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
>>>>>>> cbc9bb05... fixup add vendor back
	rt := t.base
	if rt == nil {
		return nil, errors.New("transport: no Transport specified")
	}
<<<<<<< HEAD
=======
	if t.userAgent == "" {
		return rt.RoundTrip(req)
	}
>>>>>>> cbc9bb05... fixup add vendor back
	newReq := *req
	newReq.Header = make(http.Header)
	for k, vv := range req.Header {
		newReq.Header[k] = vv
	}
<<<<<<< HEAD
	if t.userAgent != "" {
		// TODO(cbro): append to existing User-Agent header?
		newReq.Header.Set("User-Agent", t.userAgent)
	}
=======
	// TODO(cbro): append to existing User-Agent header?
	newReq.Header.Set("User-Agent", t.userAgent)
>>>>>>> cbc9bb05... fixup add vendor back

	// Attach system parameters into the header
	if t.quotaProject != "" {
		newReq.Header.Set("X-Goog-User-Project", t.quotaProject)
	}
	if t.requestReason != "" {
		newReq.Header.Set("X-Goog-Request-Reason", t.requestReason)
	}

	return rt.RoundTrip(&newReq)
}

// Set at init time by dial_appengine.go. If nil, we're not on App Engine.
var appengineUrlfetchHook func(context.Context) http.RoundTripper

// defaultBaseTransport returns the base HTTP transport.
<<<<<<< HEAD
// On App Engine, this is urlfetch.Transport.
// Otherwise, use a default transport, taking most defaults from
// http.DefaultTransport.
// If TLSCertificate is available, set TLSClientConfig as well.
func defaultBaseTransport(ctx context.Context, clientCertSource cert.Source) http.RoundTripper {
	if appengineUrlfetchHook != nil {
		return appengineUrlfetchHook(ctx)
	}

	// Copy http.DefaultTransport except for MaxIdleConnsPerHost setting,
	// which is increased due to reported performance issues under load in the GCS
	// client. Transport.Clone is only available in Go 1.13 and up.
	trans := clonedTransport(http.DefaultTransport)
	if trans == nil {
		trans = fallbackBaseTransport()
	}
	trans.MaxIdleConnsPerHost = 100

	if clientCertSource != nil {
		trans.TLSClientConfig = &tls.Config{
			GetClientCertificate: clientCertSource,
		}
	}

	return trans
}

// fallbackBaseTransport is used in <go1.13 as well as in the rare case if
// http.DefaultTransport has been reassigned something that's not a
// *http.Transport.
func fallbackBaseTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func addOCTransport(trans http.RoundTripper, settings *internal.DialSettings) http.RoundTripper {
	if settings.TelemetryDisabled {
		return trans
	}
=======
// On App Engine, this is urlfetch.Transport, otherwise it's http.DefaultTransport.
func defaultBaseTransport(ctx context.Context) http.RoundTripper {
	if appengineUrlfetchHook != nil {
		return appengineUrlfetchHook(ctx)
	}
	return http.DefaultTransport
}

func addOCTransport(trans http.RoundTripper) http.RoundTripper {
>>>>>>> cbc9bb05... fixup add vendor back
	return &ochttp.Transport{
		Base:        trans,
		Propagation: &propagation.HTTPFormat{},
	}
}
