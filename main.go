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

const (
	logInfo   = "[i]"
	logNotice = "[*]"
	logError  = "[!]"
)

const (
	dockerCGroupFormat  = "/docker/%s/"
	dockerHostEnv       = "DOCKER_HOST"
	dockerAPIVersionEnv = "DOCKER_API_VERSION"
	dockerCertPathEnv   = "DOCKER_CERT_PATH"
	dockerTLSVerifyEnv  = "DOCKER_TLS_VERIFY"
)

type dockerFlags struct {
	host       string
	apiVersion string
	certPath   string
	tlsVerify  bool
}

const (
	containerNameEnv = "CONTAINER"
	startTriesEnv    = "START_TRIES"
	checkTriesEnv    = "CHECK_TRIES"
	checkIntervalEnv = "CHECK_INTERVAL"
	usePIDEnv        = "USE_PID"
	useCGroupEnv     = "USE_CGROUP"
	notifySDEnv      = "NOTIFY_SD"
	stopOnSIGINTEnv  = "STOP_ON_SIGINT"
	stopOnSIGTERMEnv = "STOP_ON_SIGTERM"
	stopTimeoutEnv   = "STOP_TIMEOUT"
)

type flags struct {
	containerName string
	startTries    int
	checkTries    int
	checkInterval int
	usePID        bool
	useCGroup     bool
	notifySD      bool
	stopOnSIGINT  bool
	stopOnSIGTERM bool
	stopTimeout   string
	dockerFlags   dockerFlags
}

func parseFlags(flags *flags) {
	log.SetFlags(log.Ldate | log.Ltime)

	flag.StringVar(&((*flags).containerName), "container", "", "Name or ID of container")
	flag.IntVar(&((*flags).startTries), "startTries", 3, "Number of tries to start the container if it is stopped")
	flag.IntVar(&((*flags).checkTries), "checkTries", 3, "Number of tries to check the container if it is running")
	flag.IntVar(&((*flags).checkInterval), "checkInterval", 500, "Number of milliseconds between each container check")
	flag.BoolVar(&((*flags).usePID), "usePID", true, "Check existence of process via container PID")
	flag.BoolVar(&((*flags).useCGroup), "useCGroup", true, "Check existence of process via container CGroup")
	flag.BoolVar(&((*flags).notifySD), "notifySD", true, "Notify systemd about service state changes")
	flag.BoolVar(&((*flags).stopOnSIGINT), "stopOnSIGINT", false, "Stop the container on receiving signal SIGINT")
	flag.BoolVar(&((*flags).stopOnSIGTERM), "stopOnSIGTERM", true, "Stop the container on receiving signal SIGTERM")
	flag.StringVar(&((*flags).stopTimeout), "stopTimeout", "", "Timeout before the container is gracefully killed")

	if value, ok := os.LookupEnv(containerNameEnv); ok {
		flag.Set("container", value)
	}
	if value, ok := os.LookupEnv(startTriesEnv); ok {
		flag.Set("startTries", value)
	}
	if value, ok := os.LookupEnv(checkTriesEnv); ok {
		flag.Set("checkTries", value)
	}
	if value, ok := os.LookupEnv(checkIntervalEnv); ok {
		flag.Set("checkInterval", value)
	}
	if value, ok := os.LookupEnv(usePIDEnv); ok {
		flag.Set("usePID", value)
	}
	if value, ok := os.LookupEnv(useCGroupEnv); ok {
		flag.Set("useCGroup", value)
	}
	if value, ok := os.LookupEnv(notifySDEnv); ok {
		flag.Set("notifySD", value)
	}
	if value, ok := os.LookupEnv(stopOnSIGINTEnv); ok {
		flag.Set("stopOnSIGINT", value)
	}
	if value, ok := os.LookupEnv(stopOnSIGTERMEnv); ok {
		flag.Set("stopOnSIGTERM", value)
	}
	if value, ok := os.LookupEnv(stopTimeoutEnv); ok {
		flag.Set("stopTimeout", value)
	}

	flag.StringVar(&((*flags).dockerFlags.host), "dockerHost", os.Getenv(dockerHostEnv),
		"Set the URL to the docker server, leave empty for default")
	flag.StringVar(&((*flags).dockerFlags.apiVersion), "dockerAPIVersion", os.Getenv(dockerAPIVersionEnv),
		"Set the version of the API to reach, leave empty for default")
	flag.StringVar(&((*flags).dockerFlags.certPath), "dockerCertPath", os.Getenv(dockerCertPathEnv),
		"Set the path to load the TLS certificates from, leave empty for default")
	flag.BoolVar(&((*flags).dockerFlags.tlsVerify), "dockerTLSVerify", os.Getenv(dockerTLSVerifyEnv) != "",
		"Enable or disable TLS verification, off by default")

	flag.Parse()

	if len((*flags).containerName) == 0 {
		log.Panicln(logError, "Name or ID of container is missing!")
	}
	if !(*flags).usePID && (*flags).useCGroup {
		log.Panicln(logError, "Flag useCGroup depends upon flag usePID!")
	}

	if os.Getenv(dockerHostEnv) != (*flags).dockerFlags.host {
		os.Setenv(dockerHostEnv, (*flags).dockerFlags.host)
	}
	if os.Getenv(dockerAPIVersionEnv) != (*flags).dockerFlags.apiVersion {
		os.Setenv(dockerAPIVersionEnv, (*flags).dockerFlags.apiVersion)
	}
	if os.Getenv(dockerCertPathEnv) != (*flags).dockerFlags.certPath {
		os.Setenv(dockerCertPathEnv, (*flags).dockerFlags.certPath)
	}
	if (*flags).dockerFlags.tlsVerify {
		os.Setenv(dockerTLSVerifyEnv, "true")
	} else {
		os.Unsetenv(dockerTLSVerifyEnv)
	}

	log.Println(logInfo, "Provided container name or ID:", (*flags).containerName)
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
					process = watchProcess(ctx, pid, cgroup, flags.checkInterval)
				} else {
					process = watchProcess(ctx, pid, "", flags.checkInterval)
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
					log.Println(logNotice, "Container has stopped (notified via docker) and will be restarted.")
					if flags.notifySD {
						daemon.SdNotify(false, daemon.SdNotifyReloading)
					}
					continue loop
				}
			case process := <-process:
				if process {
					log.Println(logNotice, "Container has stopped (notified via ticker) and will be restarted.")
					if flags.notifySD {
						daemon.SdNotify(false, daemon.SdNotifyReloading)
					}
					continue loop
				}
			case stop := <-stop:
				if stop {
					log.Println(logNotice, "Container will be stopped due to system signal.")
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
