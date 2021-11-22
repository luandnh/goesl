package goesl

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/textproto"
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
	if am.Get("Reply-Text") != "+OK accepted" {
		return errors.New("invalid password")
	}
	go c.HandleMessage()
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

// SendAsync - Send command and get response message
func (c *ESLConnection) Send(cmd string) (*ESLResponse, error) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	_, err := c.conn.Write([]byte(cmd + EndOfMessage))
	if err != nil {
		return nil, err
	}

	// Get response
	c.responseChanMutex.RLock()
	defer c.responseChanMutex.RUnlock()
	select {
	case err := <-c.err:
		return nil, err
	case response := <-c.responseMessage:
		if response == nil {
			// Nil here if the channel is closed
			return nil, errors.New("connection closed")
		}
		return response, nil
	}
}

// SendAsync - Send command but don't get response message
func (c *ESLConnection) SendAsync(cmd string) error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	_, err := c.conn.Write([]byte(cmd + EndOfMessage))
	return err
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

// HandleMessage - Handle message from channel
func (c *ESLConnection) HandleMessage() {
	done := make(chan bool)
	go func() {
		for {
			msg, err := c.ParseResponse()
			if err != nil {
				c.err <- err
				done <- true
				break
			}
			c.responseMessage <- msg
		}
	}()
	<-done
	c.Close()
}

// Close - Close connection
func (c *ESLConnection) Close() error {
	if err := c.conn.Close(); err != nil {
		return err
	}

	return nil
}

// ExitAndClose - Send exit command before close connection
func (c *ESLConnection) ExitAndClose() {
	_, _ = c.Send("exit")
	c.Close()
}
