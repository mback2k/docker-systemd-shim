/*
	docker-systemd-shim - Shim to allow easy container control via systemd
	Copyright (C) 2018  Marc Hoersken <info@marc-hoersken.de>

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
	"context"
	"os"
	"syscall"
	"time"
)

func checkProcess(pid int) bool {
	process, err := os.FindProcess(pid)
	if err == nil {
		err := process.Signal(syscall.Signal(0))
		if err == nil {
			return true
		}
	}
	return false
}

func watchProcess(ctx context.Context, pid int) <-chan bool {
	stopped := make(chan bool)

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

	loop:
		for {
			select {
			case <-ctx.Done():
				// returning not to leak the goroutine
				break loop
			case <-ticker.C:
				if !checkProcess(pid) {
					stopped <- true
					break loop
				}
			}
		}

		close(stopped)
	}()

	return stopped
}
