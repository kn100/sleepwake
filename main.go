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

	conn, err := dbus.SystemBus()
	if err != nil {
		log.Fatal().Err(err).Msg("error connecting to dbus")
	}
	defer conn.Close()
	for {
		fd, err := InhibitSleep(conn)
		if err != nil {
			log.Fatal().Err(err).Msg("error inhibiting sleep")
		}
		log.Info().Msg("inhibited sleep, waiting for system to sleep")

		err = WaitForSleep(conn)
		if err != nil {
			log.Fatal().Err(err).Msg("error waiting for sleep")
			break
		}
		log.Info().Msg("system is sleeping shortly")

		err = onSleep()
		if err != nil {
			log.Error().Err(err).Msg("error doing sleep tasks")
		}
		log.Info().Msg("done with sleep tasks")

		sleepTime := time.Now()
		err = UninhibitSleep(fd)
		if err != nil {
			log.Error().Err(err).Msg("error uninhibiting sleep")
			break
		}
		log.Info().Msg("giving up our sleep lock to allow system to sleep")
		// At some point, the system is going to go to sleep. We don't know exactly
		// when that will be, but program execution will stop at some point. After,
		// it continues as if nothing happened. We need to detect that. We do this
		// by seeing if 15 wall clock seconds have passed since the time we slept
		// If it has, we assume the system has woken up and we can continue going.
		for time.Since(sleepTime) < 15*time.Second {
			time.Sleep(100 * time.Millisecond)
		}
		log.Info().Msg("15 wall clock seconds have passed since last sleep event, assuming we're awake")

		err = OnWake()
		if err != nil {
			log.Error().Err(err).Msg("error switching to pc")
		}
		log.Info().Msg("done with wake tasks")
	}
	log.Info().Msg("exiting")
}

// Blocks until a sleep is detected
func WaitForSleep(dbusConn *dbus.Conn) error {
	err := dbusConn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.login1.Manager"),
		dbus.WithMatchObjectPath("/org/freedesktop/login1"),
		dbus.WithMatchMember("PrepareForSleep"),
	)
	if err != nil {
		return err
	}
	dbusChan := make(chan *dbus.Signal, 10)
	dbusConn.Signal(dbusChan)
	for sig := range dbusChan {
		if sig.Name == "org.freedesktop.login1.Manager.PrepareForSleep" {
			prepareForSleep, ok := sig.Body[0].(bool)
			if ok && prepareForSleep {
				return nil
			}
		}
	}
	return fmt.Errorf("error waiting for sleep")
}

// Returns a fd which represents a lock on the system's ability to sleep
func InhibitSleep(conn *dbus.Conn) (uint32, error) {
	var fd uint32
	err := conn.Object(
		"org.freedesktop.login1",
		dbus.ObjectPath("/org/freedesktop/login1"),
	).Call(
		"org.freedesktop.login1.Manager.Inhibit",
		0,
		"sleep",
		"sleepwake",
		"sleepwake needs time to do stuff",
		"delay",
	).Store(&fd)
	if err != nil {
		return uint32(0), err
	}
	return fd, nil
}

// Uninhibits sleep by closing the file descriptor
func UninhibitSleep(fd uint32) error {
	err := syscall.Close(int(fd))
	if err != nil {
		return fmt.Errorf("error closing file descriptor: %w", err)
	}
	return nil
}

// Called when the system is going to sleep
// This is where you put your sleep tasks
func onSleep() error {
	return switchTo("mac")
}

// Called when the system has woken up
// This is where you put your wake tasks
func OnWake() error {
	return switchTo("pc")
}

func switchTo(system string) error {
	resp, err := makeRequestWithRetry(fmt.Sprintf("http://192.168.18.15/%s", system), 10)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.Status == "409" {
		log.Info().Str("switched_to", system).Msg("already in requested mode")
	} else {
		log.Info().Str("switched_to", system).Msg("switched over")
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
	return resp, fmt.Errorf("failed to make request after %d retries. Last error: %s", retries, err.Error())
}
