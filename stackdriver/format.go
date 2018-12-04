// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stackdriver

import (
	"go.uber.org/zap/zapcore"
	"golang.org/x/exp/errors"
)

type ServiceContext struct {
	Service string `json:"service"`
	Version string `json:"version"`
}

func (sc *ServiceContext) Clone() *ServiceContext {
	return &ServiceContext{
		Service: sc.Service,
		Version: sc.Version,
	}
}

// MarshalLogObject implements zapcore ObjectMarshaler.
func (sc *ServiceContext) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	if sc.Service == "" {
		return errors.New("service name is mandatory")
	}
	enc.AddString("service", sc.Service)
	enc.AddString("version", sc.Version)

	return nil
}

type Context struct {
	User           string          `json:"user"`
	HTTPRequest    *HTTPRequest    `json:"httpRequest"`
	ReportLocation *ReportLocation `json:"reportLocation"`
}

func (c *Context) IsEmpty() bool {
	return c.User == "" && c.HTTPRequest == nil && c.ReportLocation == nil
}

func (c *Context) Clone() *Context {
	output := &Context{
		User: c.User,
	}

	if c.HTTPRequest != nil {
		output.HTTPRequest = c.HTTPRequest.Clone()
	}

	if c.ReportLocation != nil {
		output.ReportLocation = c.ReportLocation.Clone()
	}

	return output
}

func (c *Context) MarshalLogObject(enc zapcore.ObjectEncoder) (err error) {
	if c.User != "" {
		enc.AddString("user", c.User)
	}

	if c.HTTPRequest != nil {
		if err = enc.AddObject("httpRequest", c.HTTPRequest); err != nil {
			return
		}
	}

	if c.ReportLocation != nil {
		if err = enc.AddObject("reportLocation", c.ReportLocation); err != nil {
			return
		}
	}

	return
}

type HTTPRequest struct {
	Method             string `json:"method"`
	URL                string `json:"url"`
	UserAgent          string `json:"userAgent"`
	Referrer           string `json:"referrer"`
	ResponseStatusCode int    `json:"responseStatusCode"`
	RemoteIP           string `json:"remoteIp"`
}

func (req *HTTPRequest) Clone() *HTTPRequest {
	return &HTTPRequest{
		Method:             req.Method,
		URL:                req.URL,
		UserAgent:          req.UserAgent,
		Referrer:           req.Referrer,
		ResponseStatusCode: req.ResponseStatusCode,
		RemoteIP:           req.RemoteIP,
	}
}

func (req *HTTPRequest) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("method", req.Method)
	enc.AddString("url", req.URL)
	enc.AddString("userAgent", req.UserAgent)
	enc.AddString("referrer", req.Referrer)
	enc.AddInt("responseStatusCode", req.ResponseStatusCode)
	enc.AddString("remoteIp", req.RemoteIP)
	return nil
}

type ReportLocation struct {
	File     string
	Line     int
	Function string
}

func (r *ReportLocation) Clone() *ReportLocation {
	return &ReportLocation{
		File:     r.File,
		Line:     r.Line,
		Function: r.Function,
	}
}

func (r *ReportLocation) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("filePath", r.File)
	enc.AddInt("lineNumber", r.Line)
	enc.AddString("functionName", r.Function)
	return nil
}
