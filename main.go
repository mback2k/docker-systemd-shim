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
	"log"
	"os"
	"os/signal"
	"syscall"
)

type flags struct {
	containerID   string
	startTries    int
	checkTries    int
	usePID        bool
	stopOnSIGTERM bool
	stopTimeout   string
}

func parseFlags(flags *flags) {
	log.SetFlags(log.Ldate | log.Ltime)

	flag.StringVar(&((*flags).containerID), "container", "", "Name or ID of container")
	flag.IntVar(&((*flags).startTries), "startTries", 3, "Number of tries to start the container if it is stopped")
	flag.IntVar(&((*flags).checkTries), "checkTries", 3, "Number of tries to check the container if it is running")
	flag.BoolVar(&((*flags).usePID), "usePID", true, "Check existence of process via container PID")
	flag.BoolVar(&((*flags).stopOnSIGTERM), "stopOnSIGTERM", true, "Stop the container on system signal SIGTERM")
	flag.StringVar(&((*flags).stopTimeout), "stopTimeout", "", "Timeout before the container is gracefully killed")
	flag.Parse()

	if len((*flags).containerID) == 0 {
		log.Panicln("[!]", "Name or containerID is missing!")
	}

	log.Println("[i]", "Provided container name or ID:", (*flags).containerID)
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
				if sig == syscall.SIGTERM {
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

		if pid := startContainer(ctx, cli, flags.containerID,
			flags.startTries, flags.checkTries, flags.usePID); pid > 0 {

			var watched <-chan bool
			var waited <-chan bool

			watched = watchContainer(ctx, cli, flags.containerID)
			if flags.usePID {
				waited = waitProcess(ctx, pid)
			} else {
				waited = make(chan bool)
			}

			select {
			case <-ctx.Done():
				// returning not to leak the goroutine
				break loop
			case watched := <-watched:
				if watched {
					log.Println("[*]", "Container has stopped (notified via docker) and will be restarted.")
					continue loop
				}
			case waited := <-waited:
				if waited {
					log.Println("[*]", "Container has stopped (notified via system) and will be restarted.")
					continue loop
				}
			case stop := <-stop:
				if stop {
					log.Println("[*]", "Container will be stopped due to system signal.")
					stopContainer(ctx, cli, flags.containerID, flags.stopTimeout)
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
