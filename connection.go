/*
 * Copyright (c) 2021 LuanDNH
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * Contributor(s):
 * LuanDNH <luandnh98@gmail.com>
 */

package goesl

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"time"
)

// ESLConnection
type ESLConnection struct {
	conn net.Conn
	err  chan error

	reader            *bufio.Reader
	header            *textproto.Reader
	writeLock         sync.Mutex
	responseMessage   chan *ESLResponse
	responseChanMutex sync.RWMutex

	runningContext context.Context
	logger         Logger
	stopFunc       func()

	isClosed  bool
	closeOnce sync.Once
}

const EndOfMessage = "\r\n\r\n"

// Options - Generic options for an ESL connection, either inbound or outbound
type Options struct {
	Context context.Context
	Logger  Logger
}

// DefaultOptions - The default options used for creating the connection
var DefaultOptions = Options{
	Context: context.Background(),
	Logger:  NormalLogger{},
}

func newConnection(c net.Conn, outbound bool, opts Options) *ESLConnection {
	reader := bufio.NewReader(c)
	header := textproto.NewReader(reader)

	if opts.Logger == nil {
		opts.Logger = NilLogger{}
	}

	runningContext, stop := context.WithCancel(opts.Context)

	instance := &ESLConnection{
		conn:            c,
		reader:          reader,
		header:          header,
		responseMessage: make(chan *ESLResponse),
		runningContext:  runningContext,
		stopFunc:        stop,
		logger:          opts.Logger,
		err:             make(chan error),
	}
	return instance
}

func (c *ESLConnection) Dial(protocol string, address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout(protocol, address, timeout)
}

// Authenticate - Method used to authenticate client against freeswitch.
func (c *ESLConnection) Authenticate(ctx context.Context, password string) error {
	header, err := c.header.ReadMIMEHeader()
	if err != nil && err.Error() != "EOF" {
		return err
	}
	if header.Get("Content-Type") != "auth/request" {
		return errors.New("auth request is invalid")
	}
	cmd := "auth " + password + EndOfMessage
	_, err = io.WriteString(c.conn, cmd)
	if err != nil {
		return err
	}
	am, err := c.header.ReadMIMEHeader()
	if err != nil && err.Error() != "EOF" {
		return err
	}
	c.logger.Debug("%s", am.Get("Reply-Text"))
	if am.Get("Reply-Text") != "+OK accepted" {
		return errors.New("invalid password")
	}
	go c.receiveLoop()
	return nil
}

// SendWithContext - Send command and get response message with deadline
func (c *ESLConnection) SendWithContext(ctx context.Context, cmd string) (*ESLResponse, error) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetWriteDeadline(deadline)
	}
	_, err := c.conn.Write([]byte(cmd + EndOfMessage))
	if err != nil {
		return nil, err
	}

	// Get response
	c.responseChanMutex.RLock()
	defer c.responseChanMutex.RUnlock()
	select {
	case response := <-c.responseMessage:
		if response == nil {
			// Nil here if the channel is closed
			return nil, errors.New("connection closed")
		}
		return response, nil
	case err := <-c.err:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

const DEFAULT_TIMEOUT = time.Second * 60

// Send - Send command and get response message
func (c *ESLConnection) Send(cmd string) (*ESLResponse, error) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), DEFAULT_TIMEOUT)
	defer cancel()

	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetWriteDeadline(deadline)
	}
	_, err := c.conn.Write([]byte(cmd + EndOfMessage))
	if err != nil {
		return nil, err
	}

	// Get response
	c.responseChanMutex.RLock()
	defer c.responseChanMutex.RUnlock()
	select {
	case response := <-c.responseMessage:
		if response == nil {
			// Nil here if the channel is closed
			return nil, errors.New("connection closed")
		}
		return response, nil
	case err := <-c.err:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SendAsync - Send command but don't get response message
func (c *ESLConnection) SendAsync(cmd string) error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	_, err := c.conn.Write([]byte(cmd + EndOfMessage))
	return err
}

// SendEvent - Loop to passed event headers
func (c *ESLConnection) SendEvent(eventHeaders []string) (*ESLResponse, error) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), DEFAULT_TIMEOUT)
	defer cancel()

	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetWriteDeadline(deadline)
	}
	_, err := c.conn.Write([]byte("sendevent "))
	if err != nil {
		return nil, err
	}
	for _, eventHeader := range eventHeaders {
		_, err := c.conn.Write([]byte(eventHeader))
		if err != nil {
			return nil, err
		}
		_, err = c.conn.Write([]byte("\r\n"))
		if err != nil {
			return nil, err
		}

	}
	_, err = c.conn.Write([]byte("\r\n"))
	if err != nil {
		return nil, err
	}
	// Get response
	c.responseChanMutex.RLock()
	defer c.responseChanMutex.RUnlock()
	select {
	case response := <-c.responseMessage:
		if response == nil {
			// Nil here if the channel is closed
			return nil, errors.New("connection closed")
		}
		return response, nil
	case err := <-c.err:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ReadMessage - Read message from channel and return ESLResponse
func (c *ESLConnection) ReadMessage() (*ESLResponse, error) {
	select {
	case response := <-c.responseMessage:
		if response == nil {
			// Nil here if the channel is closed
			return nil, errors.New("connection closed")
		}
		return response, nil
	case err := <-c.err:
		return nil, err
	}
}

// Close - Close our connection to FreeSWITCH without sending "exit". Protected by a sync.Once
func (c *ESLConnection) Close() {
	c.closeOnce.Do(c.close)
}

// Close - Close connection
func (c *ESLConnection) close() {
	// c.responseChanMutex.Lock()
	// defer c.responseChanMutex.Unlock()
	close(c.responseMessage)
	c.isClosed = true
	if err := c.conn.Close(); err != nil {
		c.logger.Error("close connection error: %v", err)
	}
	return
}

// ExitAndClose - Send exit command before close connection
func (c *ESLConnection) ExitAndClose() {
	_, _ = c.Send("exit")
	c.Close()
}

// SendEvent - Loop to passed event headers
func (c *ESLConnection) SendMsg(msg map[string]string, uuid, data string) (*ESLResponse, error) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), DEFAULT_TIMEOUT)
	defer cancel()

	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetWriteDeadline(deadline)
	}

	b := bytes.NewBufferString("sendmsg")
	if len(uuid) > 0 {
		if strings.Contains(uuid, "\r\n") {
			return nil, fmt.Errorf("%v", "invalid uuid")
		}

		b.WriteString(" " + uuid)
	}
	b.WriteString("\n")
	for k, v := range msg {
		if strings.Contains(k, "\r\n") {
			return nil, fmt.Errorf("%s: %s", k, "invalid")
		}

		if v != "" {
			if strings.Contains(v, "\r\n") {
				return nil, fmt.Errorf("%s: %s", v, "invalid")
			}

			b.WriteString(fmt.Sprintf("%s: %s\n", k, v))
		}
	}
	b.WriteString("\n")
	if len(msg["content-length"]) > 0 && len(data) > 0 {
		b.WriteString(data)
	}
	b.WriteString(EndOfMessage)
	_, err := c.conn.Write(b.Bytes())
	if err != nil {
		return nil, err
	}
	// Get response
	c.responseChanMutex.RLock()
	defer c.responseChanMutex.RUnlock()
	select {
	case response := <-c.responseMessage:
		if response == nil {
			// Nil here if the channel is closed
			return nil, errors.New("connection closed")
		}
		return response, nil
	case err := <-c.err:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *ESLConnection) receiveLoop() {
	done := make(chan bool)
	go func() {
		for c.runningContext.Err() == nil {
			err := c.doMessage()
			if err != nil {
				c.logger.Warn("err receiving message: %v", err)
				c.err <- err
				done <- true
				break
			}
		}
	}()
	<-done
	c.Close()
}

func (c *ESLConnection) doMessage() error {
	msg, err := c.ParseResponse()
	if err != nil {
		return err
	}

	c.responseChanMutex.RLock()
	defer c.responseChanMutex.RUnlock()
	if c.isClosed {
		return errors.New("connection closed, no response channel")
	}

	select {
	case c.responseMessage <- msg:
	case <-c.runningContext.Done():
		return c.runningContext.Err()
	}
	return nil
}

// // HandleMessage - Handle message from channel
// func (c *ESLConnection) HandleMessage() {
// 	done := make(chan bool)
// 	go func() {
// 		for {
// 			msg, err := c.ParseResponse()
// 			if err != nil {
// 				c.err <- err
// 				done <- true
// 				break
// 			}
// 			c.responseMessage <- msg
// 		}
// 	}()
// 	<-done
// 	c.Close()
// }
