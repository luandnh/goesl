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

func (c *ESLConnection) Api(cmd string) (*ESLResponse, error) {
	return c.Send("api " + cmd)
}

func (c *ESLConnection) BgApi(cmd string) error {
	return c.SendAsync("api " + cmd)
}

func (c *ESLConnection) Exit(cmd string) error {
	return c.SendAsync("exit")
}
