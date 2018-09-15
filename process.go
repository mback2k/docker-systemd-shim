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
	"log"
	"os"
	"syscall"
)

func checkProcess(pid int) bool {
	var checkResult = false

	process, err := os.FindProcess(pid)
	if err != nil {
		log.Println("[i]", "Failed to find process:", err)
	} else {
		err := process.Signal(syscall.Signal(0))
		log.Println("[i]", "Signal on PID", pid, "returned error:", err)

		if err == nil {
			checkResult = true
		}
	}

	return checkResult
}

func waitProcess(ctx context.Context, pid int) <-chan bool {
	stopped := make(chan bool)

	go func() {
		process, err := os.FindProcess(pid)
		if err != nil {
			log.Println("[i]", "Failed to find process:", err)

			stopped <- true
		} else {
			state, err := process.Wait()
			log.Println("[i]", "Waiting on PID", pid, "returned error:", err)

			if err == nil {
				log.Println("[*]", "Waiting on PID", pid, "returned state:", state)
			}

			stopped <- true
		}

		close(stopped)
	}()

	return stopped
}
