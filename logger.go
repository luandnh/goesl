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

import log "github.com/sirupsen/logrus"

type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

type NilLogger struct{}
type NormalLogger struct{}

func (l NormalLogger) Debug(format string, args ...interface{}) {
	log.Debugf(format, args...)
}
func (l NormalLogger) Info(format string, args ...interface{}) {
	log.Infof(format, args...)
}
func (l NormalLogger) Warn(format string, args ...interface{}) {
	log.Warnf(format, args...)
}
func (l NormalLogger) Error(format string, args ...interface{}) {
	log.Errorf(format, args...)
}

func (l NilLogger) Debug(string, ...interface{}) {}
func (l NilLogger) Info(string, ...interface{})  {}
func (l NilLogger) Warn(string, ...interface{})  {}
func (l NilLogger) Error(string, ...interface{}) {}
