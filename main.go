package main

import (
	"syscall"

	"github.com/godbus/dbus/v5"
	"github.com/rs/zerolog/log"
)

func main() {
	var fd int
	var conn *dbus.Conn
	defer conn.Close()
	for {
		// Firstly, check if connection is open
		if conn == nil || !conn.Connected() {
			var err error
			conn, err = dbus.SystemBus()
			if err != nil {
				log.Error().Err(err).Msg("error connecting to dbus")
				break
			}
		}
		log.Info().Msg("Connected to dbus")

		var err error
		if fd == -1 {
			// If fd is -1, it means that the sleep listener was unregistered, and therefore machine is waking up.
			log.Info().Msg("machine just woke up")
			fd, err = RegisterSleepListener(conn)
		} else if fd == 0 || syscall.FcntlFlock(uintptr(fd), syscall.F_SETLKW, &syscall.Flock_t{Type: syscall.F_WRLCK}) != nil {
			log.Info().Msg("sleep listener not registered, registering now for the first time")
			fd, err = RegisterSleepListener(conn)
		}

		if err != nil {
			log.Error().Err(err).Msg("error registering sleep listener")
			break
		}

		log.Info().Msg("sleep listener registered")
		sleepSignal := make(chan *dbus.Signal, 1)
		conn.Signal(sleepSignal)
		for range sleepSignal {
			if err := syscall.Close(fd); err != nil {
				log.Error().Err(err).Msg("error closing fd") // Not rly sure why this errors. Whatever.
			}
			fd = -1
			log.Info().Msg("sleep signal received")
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
