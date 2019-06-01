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
	"context"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/cgroups"
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

func checkCGroup(pid int, cgroup string) bool {
	control, err := cgroups.Load(cgroups.V1, cgroups.StaticPath(cgroup))
	if err == nil {
		subsystems := control.Subsystems()
		for si := 0; si < len(subsystems); si++ {
			subsystem := subsystems[si]
			processes, err := control.Processes(subsystem.Name(), false)
			if err == nil {
				for pi := 0; pi < len(processes); pi++ {
					process := processes[pi]
					if process.Pid == pid && !strings.HasSuffix(process.Path, cgroup) {
						return false
					}
				}
			}
		}
		if len(subsystems) > 0 {
			return true
		}
	}
	return false
}

func watchProcess(ctx context.Context, pid int, cgroup string, interval time.Duration) <-chan bool {
	stopped := make(chan bool)

	go func() {
		ticker := time.NewTicker(interval)
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
				if len(cgroup) > 0 && !checkCGroup(pid, cgroup) {
					stopped <- true
					break loop
				}
			}
		}

		close(stopped)
	}()

	return stopped
}
