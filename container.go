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
	"time"

	"github.com/coreos/go-systemd/daemon"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func createClient() *client.Client {
	cli, err := client.NewEnvClient()
	if err != nil {
		log.Panicln("[!]", err)
	}
	return cli
}

func startContainer(ctx context.Context, cli *client.Client, containerID string,
	startTries int, checkTries int, usePID bool, notifySD bool) int {

	var containerPID = 0

started:
	for {
		log.Println("[*]", "Inspecting container ...")
		response, err := cli.ContainerInspect(ctx, containerID)
		if err != nil {
			log.Panicln("[!]", err)
		} else {
			log.Println("[i]", "Container ID:", response.ID)
			log.Println("[i]", "Container Name:", response.Name)
			log.Println("[i]", "Container Status:", response.State.Status)
			log.Println("[i]", "Container PID:", response.State.Pid)
		}

		if notifySD {
			daemon.SdNotify(false, "STATUS="+response.State.Status)
		}

		if response.State.Status == "running" {
			log.Println("[*]", "Container is running.")
			if usePID {
				log.Println("[*]", "Checking for PID existence ...")
				checkTries = checkTries - 1
				if checkProcess(response.State.Pid) {
					containerPID = response.State.Pid
					log.Println("[*]", "Successfully checked for PID existence.")
					break started
				} else {
					log.Println("[*]", "Failed to check for PID existence.")
					if checkTries == 0 {
						break started
					} else {
						continue started
					}
				}
			} else {
				containerPID = response.State.Pid
				log.Println("[i]", "Skipped check for PID existence.")
				break started
			}
		} else if startTries == 0 {
			log.Panicln("[!]", "Could not start container!")
			break started
		} else {
			log.Println("[*]", "Starting container ...")
			startOptions := types.ContainerStartOptions{
				CheckpointID:  "",
				CheckpointDir: "",
			}
			startTries = startTries - 1
			err := cli.ContainerStart(ctx, response.ID, startOptions)
			if err != nil {
				log.Panicln("[!]", err)
			} else {
				log.Println("[*]", "Successfully started container.")
			}
		}
	}

	return containerPID
}

func watchContainer(ctx context.Context, cli *client.Client, containerID string) <-chan bool {
	stopped := make(chan bool)

	go func() {
		waits, errs := cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	waited:
		for {
			select {
			case <-ctx.Done():
				// returning not to leak the goroutine
				break waited
			case err := <-errs:
				if err != nil {
					log.Panicln("[!]", err)
					stopped <- false
				}
				break waited
			case w := <-waits:
				if w.Error != nil {
					log.Panicln("[!]", w.Error)
					stopped <- false
				} else {
					log.Println("[i]", "Container exit code:", w.StatusCode)
					log.Println("[*]", "Container has stopped.")
					stopped <- true
				}
				break waited
			}
		}

		close(stopped)
	}()

	return stopped
}

func stopContainer(ctx context.Context, cli *client.Client, containerID string,
	stopTimeout string) {

	var timeout time.Duration

	if len(stopTimeout) > 0 {
		parsedTimeout, err := time.ParseDuration(stopTimeout)
		if err != nil {
			log.Panicln("[!]", err)
		}
		timeout = parsedTimeout
	}

	log.Println("[*]", "Stopping container ...")
	err := cli.ContainerStop(ctx, containerID, &timeout)
	if err != nil {
		log.Panicln("[!]", err)
	} else {
		log.Println("[*]", "Successfully stopped container.")
	}
}
