package main

import (
	"fmt"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	var fd int
	var conn *dbus.Conn
	defer conn.Close()
	for {
		if conn == nil || !conn.Connected() {
			var err error
			conn, err = dbus.SystemBus()
			if err != nil {
				log.Error().Err(err).Msg("error connecting to dbus")
				break
			}
		}
		var err error
		if fd == 0 || syscall.FcntlFlock(uintptr(fd), syscall.F_SETLKW, &syscall.Flock_t{Type: syscall.F_WRLCK}) != nil {
			fd, err = RegisterSleepListener(conn)
		}

		if err != nil {
			log.Error().Err(err).Msg("error registering sleep listener")
			break
		}

		dbusChan := make(chan *dbus.Signal, 10)
		conn.Signal(dbusChan)
		log.Info().Int("fd", fd).Msg("waiting for sleep")
		for sig := range dbusChan {
			if sig.Name == "org.freedesktop.login1.Manager.PrepareForSleep" {
				prepareForSleep, ok := sig.Body[0].(bool)
				if ok && !prepareForSleep {
					err := switchTo("pc")
					if err != nil {
						log.Error().Err(err).Msg("error waking up")
					}
				} else if ok && prepareForSleep {
					err := switchTo("mac")
					if err != nil {
						log.Error().Err(err).Msg("error suspending")
					}
				} else {
					log.Error().Msg("error parsing signal")
				}
			}
			_, err := syscall.Seek(fd, 0, 0)
			if err != nil {
				// Inability to seek means descriptor is closed, so
				// we break so the whole thing can restart.
				break
			}
			// Just in case it isn't closed, we close it
			err = syscall.Close(fd)
			if err != nil {
				log.Error().Err(err).Msg("error closing file descriptor")
			}
		}
	}
	log.Info().Msg("exiting")
}

func RegisterSleepListener(conn *dbus.Conn) (int, error) {
	var fd int
	err := conn.Object(
		"org.freedesktop.login1",
		dbus.ObjectPath("/org/freedesktop/login1"),
	).Call(
		"org.freedesktop.login1.Manager.Inhibit",
		0,
		"sleep",
		"sleepwake",
		"Do stuff on suspend",
		"delay",
	).Store(&fd)
	if err != nil {
		return -1, err
	}

	err = conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.login1.Manager"),
		dbus.WithMatchObjectPath("/org/freedesktop/login1"),
		dbus.WithMatchMember("PrepareForSleep"),
	)
	if err != nil {
		return -1, err
	}

	return fd, nil
}

func switchTo(system string) error {
	resp, err := makeRequestWithRetry(fmt.Sprintf("http://192.168.18.15/%s", system), 10)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.Status == "409" {
		log.Info().Str("requested", system).Msg("already in requested mode")
	} else {
		log.Info().Str("requested", system).Msg("switched over")
	}
	return nil
}

func makeRequestWithRetry(url string, retries int) (*http.Response, error) {
	var resp *http.Response
	var err error
	for i := 0; i < retries; i++ {
		var resp *http.Response
		resp, err = http.Get(url)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		return resp, nil
	}
	log.Error().Err(err).Msg("request failed")
	return resp, fmt.Errorf("failed to make request after %d retries", retries)
}
