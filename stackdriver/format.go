// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stackdriver

import (
	"go.uber.org/zap"
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

// LogHTTPPayload adds the correct Stackdriver "HttpRequest" field.
//
// ref: https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
func LogHTTPRequest(req *HTTPRequest) zap.Field {
	return zap.Object(keyContextHTTPRequest, req)
}

type ReportLocation struct {
	FilePath     string `json:"filePath"`
	LineNumber   int    `json:"lineNumber"`
	FunctionName string `json:"functionName"`
}

func (r *ReportLocation) Clone() *ReportLocation {
	return &ReportLocation{
		FilePath:     r.FilePath,
		LineNumber:   r.LineNumber,
		FunctionName: r.FunctionName,
	}
}

func (r *ReportLocation) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("filePath", r.FilePath)
	enc.AddInt("lineNumber", r.LineNumber)
	enc.AddString("functionName", r.FunctionName)
	return nil
}

func LogContext(ctx *Context) zapcore.Field {
	return zap.Object(keyContext, ctx)
}

func LogServiceContext(sc *ServiceContext) zapcore.Field {
	return zap.Object(keyServiceContext, sc)
}

func LogUser(user string) zapcore.Field {
	return zap.String(keyContextUser, user)
}

func LogReportLocation(loc *ReportLocation) zapcore.Field {
	return zap.Object(keyContextReportLocation, loc)
}
