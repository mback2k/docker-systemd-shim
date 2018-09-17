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
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/coreos/go-systemd/daemon"
)

type flags struct {
	containerName string
	startTries    int
	checkTries    int
	usePID        bool
	useCGroup     bool
	notifySD      bool
	stopOnSIGINT  bool
	stopOnSIGTERM bool
	stopTimeout   string
}

const dockerCGroupFormat = "/docker/%s/"

func parseFlags(flags *flags) {
	log.SetFlags(log.Ldate | log.Ltime)

	flag.StringVar(&((*flags).containerName), "container", "", "Name or ID of container")
	flag.IntVar(&((*flags).startTries), "startTries", 3, "Number of tries to start the container if it is stopped")
	flag.IntVar(&((*flags).checkTries), "checkTries", 3, "Number of tries to check the container if it is running")
	flag.BoolVar(&((*flags).usePID), "usePID", true, "Check existence of process via container PID")
	flag.BoolVar(&((*flags).useCGroup), "useCGroup", true, "Check existence of process via container CGroup")
	flag.BoolVar(&((*flags).notifySD), "notifySD", true, "Notify systemd about service state changes")
	flag.BoolVar(&((*flags).stopOnSIGINT), "stopOnSIGINT", false, "Stop the container on receiving signal SIGINT")
	flag.BoolVar(&((*flags).stopOnSIGTERM), "stopOnSIGTERM", true, "Stop the container on receiving signal SIGTERM")
	flag.StringVar(&((*flags).stopTimeout), "stopTimeout", "", "Timeout before the container is gracefully killed")
	flag.Parse()

	if len((*flags).containerName) == 0 {
		log.Panicln("[!]", "Name or ID of container is missing!")
	}

	if !(*flags).usePID && (*flags).useCGroup {
		log.Panicln("[!]", "Flag useCGroup depends upon flag usePID!")
	}

	log.Println("[i]", "Provided container name or ID:", (*flags).containerName)
}

func handleSignals(ctx context.Context, stop chan<- bool, flags flags) {
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGTERM)

	go func() {
	loop:
		for {
			select {
			case <-ctx.Done():
				// returning not to leak the goroutine
				break loop
			case sig := <-sigs:
				switch sig {
				case syscall.SIGINT:
					stop <- flags.stopOnSIGINT
				case syscall.SIGTERM:
					stop <- flags.stopOnSIGTERM
				}
			}
		}

		signal.Reset(syscall.SIGTERM)
		close(sigs)
	}()
}

func workerLoop(ctx context.Context, stop <-chan bool, flags flags) {
loop:
	for {
		cli := createClient()
		defer cli.Close()

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		if id, pid := runContainer(ctx, cli,
			flags.containerName, flags.startTries, flags.checkTries,
			flags.usePID, flags.useCGroup, flags.notifySD); len(id) > 0 && pid > 0 {

			var container <-chan bool
			var process <-chan bool

			container = watchContainer(ctx, cli, flags.containerName)
			if flags.usePID {
				if flags.useCGroup {
					cgroup := fmt.Sprintf(dockerCGroupFormat, id)
					process = watchProcess(ctx, pid, cgroup)
				} else {
					process = watchProcess(ctx, pid, "")
				}
			} else {
				process = make(chan bool)
			}

			if flags.notifySD {
				daemon.SdNotify(false, daemon.SdNotifyReady)
			}

			select {
			case <-ctx.Done():
				// returning not to leak the goroutine
				break loop
			case container := <-container:
				if container {
					log.Println("[*]", "Container has stopped (notified via docker) and will be restarted.")
					if flags.notifySD {
						daemon.SdNotify(false, daemon.SdNotifyReloading)
					}
					continue loop
				}
			case process := <-process:
				if process {
					log.Println("[*]", "Container has stopped (notified via ticker) and will be restarted.")
					if flags.notifySD {
						daemon.SdNotify(false, daemon.SdNotifyReloading)
					}
					continue loop
				}
			case stop := <-stop:
				if stop {
					log.Println("[*]", "Container will be stopped due to system signal.")
					if flags.notifySD {
						daemon.SdNotify(false, daemon.SdNotifyStopping)
					}
					stopContainer(ctx, cli, flags.containerName, flags.stopTimeout)
					break loop
				}
			}
		} else {
			break loop
		}
	}
}

func main() {
	var flags flags
	parseFlags(&flags)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan bool)

	handleSignals(ctx, stop, flags)
	workerLoop(ctx, stop, flags)

	close(stop)
}
