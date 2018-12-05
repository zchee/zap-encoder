// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stackdriver

import (
	"bytes"
	"io"
	"net/http"
	"strconv"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	keyHTTPRequest = "httpRequest"
)

// HttpRequest represents a common proto for logging HTTP requests.
//
// Only contains semantics defined by the HTTP specification.
// Product-specific logging information MUST be defined in a separate message.
//
//  https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#httprequest
type HttpRequest struct {
	// The request method. Examples: "GET", "HEAD", "PUT", "POST".
	RequestMethod string `json:"requestMethod"`

	// The scheme (http, https), the host name, the path and the query portion of
	// the URL that was requested.
	//
	// Example: "http://example.com/some/info?color=red".
	RequestURL string `json:"requestUrl"`

	// The size of the HTTP request message in bytes, including the request
	// headers and the request body.
	RequestSize string `json:"requestSize"`

	// The response code indicating the status of response.
	//
	// Examples: 200, 404.
	Status int `json:"status"`

	// The size of the HTTP response message sent back to the client, in bytes,
	// including the response headers and the response body.
	ResponseSize string `json:"responseSize"`

	// The user agent sent by the client.
	//
	// Example: "Mozilla/4.0 (compatible; MSIE 6.0; Windows 98; Q312461; .NET CLR 1.0.3705)".
	UserAgent string `json:"userAgent"`

	// The IP address (IPv4 or IPv6) of the client that issued the HTTP request.
	//
	// Examples: "192.168.1.1", "FE80::0202:B3FF:FE1E:8329".
	RemoteIP string `json:"remoteIp"`

	// The IP address (IPv4 or IPv6) of the origin server that the request was
	// sent to.
	ServerIP string `json:"serverIp"`

	// The referrer URL of the request, as defined in HTTP/1.1 Header Field
	// Definitions.
	Referer string `json:"referer"`

	// The request processing latency on the server, from the time the request was
	// received until the response was sent.
	//
	// A duration in seconds with up to nine fractional digits, terminated by 's'.
	//
	// Example: "3.5s".
	Latency string `json:"latency"`

	// Whether or not a cache lookup was attempted.
	CacheLookup bool `json:"cacheLookup"`

	// Whether or not an entity was served from cache (with or without
	// validation).
	CacheHit bool `json:"cacheHit"`

	// Whether or not the response was validated with the origin server before
	// being served from cache. This field is only meaningful if cacheHit is True.
	CacheValidatedWithOriginServer bool `json:"cacheValidatedWithOriginServer"`

	// The number of HTTP response bytes inserted into cache. Set only when a
	// cache fill was attempted.
	CacheFillBytes string `json:"cacheFillBytes"`

	// Protocol used for the request.
	//
	// Examples: "HTTP/1.1", "HTTP/2", "websocket"
	Protocol string `json:"protocol"`
}

// Clone implements zapcore.Encoder.
func (req *HttpRequest) Clone() *HttpRequest {
	return &HttpRequest{
		RequestMethod:                  req.RequestMethod,
		RequestURL:                     req.RequestURL,
		RequestSize:                    req.RequestSize,
		Status:                         req.Status,
		ResponseSize:                   req.ResponseSize,
		UserAgent:                      req.UserAgent,
		RemoteIP:                       req.RemoteIP,
		ServerIP:                       req.ServerIP,
		Referer:                        req.Referer,
		Latency:                        req.Latency,
		CacheLookup:                    req.CacheLookup,
		CacheHit:                       req.CacheHit,
		CacheValidatedWithOriginServer: req.CacheValidatedWithOriginServer,
		CacheFillBytes:                 req.CacheFillBytes,
		Protocol:                       req.Protocol,
	}
}

// MarshalLogObject implements zapcore.ObjectMarshaler.
func (req *HttpRequest) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddReflected("requestMethod", req.RequestMethod)
	enc.AddString("requestURL", req.RequestURL)
	enc.AddString("requestSize", req.RequestSize)
	enc.AddInt("status", req.Status)
	enc.AddString("responseSize", req.ResponseSize)
	enc.AddString("userAgent", req.UserAgent)
	enc.AddString("remoteIP", req.RemoteIP)
	enc.AddString("serverIP", req.ServerIP)
	enc.AddString("referer", req.Referer)
	enc.AddString("latency", req.Latency)
	enc.AddBool("cacheLookup", req.CacheLookup)
	enc.AddBool("cacheHit", req.CacheHit)
	enc.AddBool("cacheValidatedWithOriginServer", req.CacheValidatedWithOriginServer)
	enc.AddString("cacheFillBytes", req.CacheFillBytes)
	enc.AddString("protocol", req.Protocol)

	return nil
}

// LogHttpRequest adds the correct Stackdriver "HttpRequest" field.
func LogHttpRequest(req *HttpRequest) zap.Field {
	return zap.Object(keyHTTPRequest, req)
}

// NewHttpRequest returns a new HttpRequest struct, based on the passed
// in http.Request and http.Response objects.
func NewHHttpRequest(req *http.Request, resp *http.Response) *HttpRequest {
	if req == nil {
		req = &http.Request{}
	}
	if resp == nil {
		resp = &http.Response{}
	}

	r := &HttpRequest{
		RequestMethod: req.Method,
		Status:        resp.StatusCode,
		UserAgent:     req.UserAgent(),
		RemoteIP:      req.RemoteAddr,
		Referer:       req.Referer(),
		Protocol:      req.Proto,
	}

	if req.URL != nil {
		r.RequestURL = req.URL.String()
	}

	buf := new(bytes.Buffer)
	if req.Body != nil {
		n, _ := io.Copy(buf, req.Body)
		r.RequestSize = strconv.FormatInt(n, 10)
	}
	buf.Reset()

	if resp.Body != nil {
		n, _ := io.Copy(buf, resp.Body)
		r.ResponseSize = strconv.FormatInt(n, 10)
	}

	return r
}
