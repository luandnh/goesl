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

package test

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/luandnh/goesl"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var (
	FS_HOST    = os.Getenv("FS_HOST")
	FS_PORT    int
	FS_ESLPASS = os.Getenv("FS_ESLPASS")
)

func init() {
	FS_PORT, _ = strconv.Atoi(os.Getenv("FS_PORT"))
	logrus.Info("FS_HOST", FS_HOST)
}

func TestClient_Dial(t *testing.T) {
	client, err := goesl.NewClient(FS_HOST, FS_PORT, FS_ESLPASS, 10)
	assert.Nil(t, err)
	defer client.Close()
}

func TestClient_Send(t *testing.T) {
	client, err := goesl.NewClient(FS_HOST, FS_PORT, FS_ESLPASS, 10)
	assert.Equal(t, err, nil)
	defer client.Close()
	message := "Hello goesl"
	rawResponse, err := client.Send(fmt.Sprintf("api eval %s", message))
	assert.Nil(t, err)
	assert.Equal(t, string(rawResponse.Body), message)
	rawResponse, err = client.Send("api fsctl ready_check")
	assert.Nil(t, err)
	assert.Equal(t, string(rawResponse.Body), "true")
}

func TestClient_Api(t *testing.T) {
	client, err := goesl.NewClient(FS_HOST, FS_PORT, FS_ESLPASS, 10)
	assert.Equal(t, err, nil)
	defer client.Close()
	message := "Hello goesl"
	rawResponse, err := client.Api(fmt.Sprintf("eval %s", message))
	assert.Nil(t, err)
	assert.Equal(t, string(rawResponse.Body), message)
	rawResponse, err = client.Api("fsctl ready_check")
	assert.Nil(t, err)
	assert.Equal(t, string(rawResponse.Body), "true")
}
