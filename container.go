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
	"fmt"
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
		log.Panicln(logError, err)
	}
	return cli
}

func checkContainer(ctx context.Context, cli *client.Client, response types.ContainerJSON,
	usePID bool, useCGroup bool) bool {

	log.Println(logNotice, "Checking container ...")
	if usePID {
		log.Println(logNotice, "Checking for PID existence ...")
		if checkProcess(response.State.Pid) {
			if useCGroup {
				log.Println(logNotice, "Checking for PID existence in CGroup ...")
				cgroup := fmt.Sprintf(dockerCGroupFormat, response.ID)
				if checkCGroup(response.State.Pid, cgroup) {
					log.Println(logNotice, "Successfully checked for PID existence in CGroup.")
					return true
				} else {
					log.Println(logNotice, "Failed to check for PID existence in CGroup.")
					return false
				}
			} else {
				log.Println(logNotice, "Successfully checked for PID existence, but skipped CGroup.")
				return true
			}
		} else {
			log.Println(logNotice, "Failed to check for PID existence.")
			return false
		}
	} else {
		log.Println(logInfo, "Skipped check for PID existence.")
		return true
	}
}

func startContainer(ctx context.Context, cli *client.Client, containerName string) bool {
	startOptions := types.ContainerStartOptions{
		CheckpointID:  "",
		CheckpointDir: "",
	}
	log.Println(logNotice, "Starting container ...")
	err := cli.ContainerStart(ctx, containerName, startOptions)
	if err != nil {
		log.Println(logError, err)
		return false
	} else {
		log.Println(logNotice, "Successfully started container.")
		return true
	}
}

func runContainer(ctx context.Context, cli *client.Client, containerName string,
	startTries int, checkTries int, usePID bool, useCGroup bool, notifySD bool) (string, int) {

	var containerID = ""
	var containerPID = 0

started:
	for {
		log.Println(logNotice, "Inspecting container ...")
		response, err := cli.ContainerInspect(ctx, containerName)
		if err != nil {
			log.Panicln(logError, err)
		} else {
			log.Println(logInfo, "Container ID:", response.ID)
			log.Println(logInfo, "Container Name:", response.Name)
			log.Println(logInfo, "Container Status:", response.State.Status)
			log.Println(logInfo, "Container PID:", response.State.Pid)
		}

		if notifySD {
			var containerStatus = response.State.Status
			if response.State.Health != nil {
				containerStatus += " [" + response.State.Health.Status + "]"
			}
			log.Println(logNotice, "Reporting status to systemd ...")
			res, err := daemon.SdNotify(false, "STATUS="+containerStatus)
			if err != nil {
				log.Println(logError, err)
			} else if res {
				log.Println(logNotice, "Reported status to systemd:", containerStatus)
			} else {
				log.Println(logError, "Reporting status to systemd is not supported.")
			}
		}

		if response.State.Status == "running" {
			log.Println(logNotice, "Container is running.")
			checkTries = checkTries - 1
			if checkContainer(ctx, cli, response, usePID, useCGroup) {
				containerID = response.ID
				containerPID = response.State.Pid
				break started
			} else if checkTries == 0 {
				break started
			} else {
				continue started
			}
		} else if startTries == 0 {
			log.Println(logError, "Could not start container!")
			break started
		} else {
			startTries = startTries - 1
			startContainer(ctx, cli, response.ID)
		}
	}

	return containerID, containerPID
}

func watchContainer(ctx context.Context, cli *client.Client, containerName string) <-chan bool {
	stopped := make(chan bool)

	go func() {
		waits, errs := cli.ContainerWait(ctx, containerName, container.WaitConditionNotRunning)

	waited:
		for {
			select {
			case <-ctx.Done():
				// returning not to leak the goroutine
				break waited
			case err := <-errs:
				if err != nil {
					log.Println(logError, err)
					stopped <- false
				}
				break waited
			case w := <-waits:
				if w.Error != nil {
					log.Println(logError, w.Error)
					stopped <- false
				} else {
					log.Println(logInfo, "Container exit code:", w.StatusCode)
					log.Println(logNotice, "Container has stopped.")
					stopped <- true
				}
				break waited
			}
		}

		close(stopped)
	}()

	return stopped
}

func stopContainer(ctx context.Context, cli *client.Client, containerName string,
	stopTimeout *time.Duration) {

	log.Println(logNotice, "Stopping container ...")
	err := cli.ContainerStop(ctx, containerName, stopTimeout)
	if err != nil {
		log.Println(logError, err)
	} else {
		log.Println(logNotice, "Successfully stopped container.")
	}
}
