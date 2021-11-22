package goesl

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
)

const (
	ContentType_AuthRequest = `auth/request`
	ContentType_Reply       = `command/reply`
	ContentType_APIResponse = `api/response`
	ContentType_Disconnect  = `text/disconnect-notice`
	// for event
	ContentType_EventPlain = `text/event-plain`
	ContentType_EventJSON  = `text/event-json`
	ContentType_EventXML   = `text/event-xml`
)

var (
	ReadBufferSize      = 1024 << 6
	AllowedContentTypes = []string{
		ContentType_AuthRequest,
		ContentType_Reply,
		ContentType_APIResponse,
		ContentType_Disconnect,
		ContentType_EventPlain,
		ContentType_EventJSON,
		ContentType_EventXML,
	}
)

type ESLResponse struct {
	Headers map[string]string
	Body    []byte
}

// GetReply - Check value in header
func (r *ESLResponse) HasHeader(header string) bool {
	_, ok := r.Headers[textproto.CanonicalMIMEHeaderKey(header)]
	return ok
}

// GetHeader - Get header value
func (r *ESLResponse) GetHeader(header string) string {
	value, _ := url.PathUnescape(r.Headers[header])
	return value
}

// IsOk - Has prefix +OK
func (r *ESLResponse) IsOk() bool {
	return strings.HasPrefix(r.GetReply(), "+OK")
}

// GetReply - Get reply value in header
func (r *ESLResponse) GetReply() string {
	if r.HasHeader("Reply-Text") {
		return r.GetHeader("Reply-Text")
	}
	return string(r.Body)
}

func (c *ESLConnection) ParseResponse() (*ESLResponse, error) {
	header, err := c.header.ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	response := &ESLResponse{
		Headers: make(map[string]string),
	}
	if err != nil && err.Error() != "EOF" {
		return nil, err
	}

	if header.Get("Content-Type") == "" {
		return nil, fmt.Errorf("Parse EOF")
	}

	if contentLength := header.Get("Content-Length"); len(contentLength) > 0 {
		length, err := strconv.Atoi(contentLength)
		if err != nil {
			return nil, err
		}
		response.Body = make([]byte, length)

		if _, err = io.ReadFull(c.reader, response.Body); err != nil {
			return response, err
		}
	}
	contentType := header.Get("Content-Type")

	if !IsExistInSlice(contentType, AllowedContentTypes) {
		return nil, errors.New(fmt.Sprintf("%s is not allowed", contentType))
	}

	if contentType != ContentType_EventJSON {
		for k, v := range header {
			response.Headers[k] = v[0]
			if strings.Contains(v[0], "%") {
				response.Headers[k], err = url.QueryUnescape(v[0])
				if err != nil {
					c.logger.Error("fail to decode : %v", err)
					continue
				}
			}
		}
	}
	switch contentType {
	case ContentType_Reply:
		reply := header.Get("Reply-Text")

		if strings.Contains(reply, "-ERR") {
			return nil, errors.New("unsuccessful reply : " + reply[5:])
		}
	case ContentType_APIResponse:
		if strings.Contains(string(response.Body), "-ERR") {
			return nil, errors.New("unsuccessful reply : " + string(response.Body)[5:])
		}
	case ContentType_EventJSON:
		var decoded map[string]interface{}
		if err := json.Unmarshal(response.Body, &decoded); err != nil {
			return nil, err
		}

		for k, v := range decoded {
			switch v.(type) {
			case string:
				response.Headers[k] = v.(string)
			default:
				c.logger.Warn("non-string property (%s)", k)
			}
		}
		if v, _ := response.Headers["_body"]; v != "" {
			response.Body = []byte(v)
			delete(response.Headers, "_body")
		} else {
			response.Body = []byte("")
		}
	case "text/event-plain":
		r := bufio.NewReader(bytes.NewReader(response.Body))

		tr := textproto.NewReader(r)

		emh, err := tr.ReadMIMEHeader()

		if err != nil {
			return nil, errors.New("could not read headers : " + string(response.Body)[5:])
		}

		if contentLength := emh.Get("Content-Length"); len(contentLength) > 0 {
			length, err := strconv.Atoi(contentLength)
			if err != nil {
				return nil, errors.New("invalid content-length : " + string(response.Body)[5:])
			}
			response.Body = make([]byte, length)
			if _, err = io.ReadFull(r, response.Body); err != nil {
				return nil, errors.New("could not read body : " + string(response.Body)[5:])
			}
		}
	}
	return response, nil
}
