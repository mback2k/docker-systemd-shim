/*
	docker-systemd-shim - Shim to allow easy container control via systemd
	Copyright (C) 2018 - 2019, Marc Hoersken <info@marc-hoersken.de>

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package main

import (
	"github.com/coreos/go-systemd/v22/daemon"
)

func notifyReady() (bool, error) {
	return daemon.SdNotify(false, daemon.SdNotifyReady)
}

func notifyReloading() (bool, error) {
	return daemon.SdNotify(false, daemon.SdNotifyReloading)
}

func notifyStopping() (bool, error) {
	return daemon.SdNotify(false, daemon.SdNotifyStopping)
}

func notifyStatus(status string) (bool, error) {
	return daemon.SdNotify(false, "STATUS="+status)
}
