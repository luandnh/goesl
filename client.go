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
	"context"
	"net"
	"strconv"
	"time"
)

// Client - Used to create an inbound connection to Freeswitch server
// In order to originate call, transfer, or something amazing ...
type Client struct {
	*ESLConnection
	Protocol     string
	Address      string
	Password     string
	Timeout      int
	OnDisconnect func()
}

// NewClient - Init new client connection, this will establish connection and attempt to authenticate against connected freeswitch server
func NewClient(host string, port int, password string, timeout int) (*Client, error) {
	client := &Client{
		Protocol: "tcp",
		Address:  net.JoinHostPort(host, strconv.Itoa(int(port))),
		Password: password,
		Timeout:  timeout,
	}
	var err error
	client.ESLConnection, err = client.EstablishConnection()
	if err != nil {
		return nil, err
	}
	return client, nil
}

// EstablishConnection - Will attempt to establish connection against freeswitch and create new connection
func (client *Client) EstablishConnection() (*ESLConnection, error) {
	c, err := client.Dial("tcp", client.Address, time.Duration(client.Timeout*int(time.Second)))
	if err != nil {
		return nil, err
	}
	connection := newConnection(c, false, DefaultOptions)
	authCtx, cancel := context.WithTimeout(connection.runningContext, time.Duration(client.Timeout)*time.Second)
	err = connection.Authenticate(authCtx, client.Password)
	cancel()
	if err != nil {
		// Disconnect, we have the wrong password.
		connection.Close()
		return nil, err
	} else {
		connection.logger.Info("Successfully connect to %s\n", connection.conn.RemoteAddr())
	}
	return connection, nil
}
