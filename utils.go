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

func IsExistInSlice(s string, list []string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
