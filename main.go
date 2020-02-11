package main

import (
	"bufio"
	"fmt"
	notification "github.com/TheCreeper/go-notify"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const (
	pipe   = ".dunst-pause"
	Pause  = "DUNST_COMMAND_PAUSE"
	Resume = "DUNST_COMMAND_RESUME"
)

var PauseResolution = 10 * time.Second
var DefaultExpiry = time.Hour

type Status int

const (
	Paused Status = iota
	Active
	Initial
)

type state struct {
	State  Status
	Until  time.Time
	mu     *sync.RWMutex
	closer chan struct{}
}

func (s *state) Pause(expiry time.Duration) {
	s.mu.Lock()

	defer s.Status()
	if s.State != Paused {
		s.State = Paused
		go pauseNotifications()
	}

	if s.closer != nil {
		// close any old timer/ticker running
		close(s.closer)
	}

	s.closer = make(chan struct{})
	s.Until = time.Now().Add(expiry)
	go s.handleTick(s.closer)
	go s.handleTimeout(s.closer, expiry)

	s.mu.Unlock()
}

func (s *state) Toggle() {
	s.mu.RLock()
	switch s.State {
	case Paused:
		defer s.Activate()
	case Active:
		fallthrough
	case Initial:
		defer s.Pause(DefaultExpiry)
	}
	s.mu.RUnlock()
}

func (s *state) Activate() {
	s.mu.Lock()
	if s.State != Active {
		defer s.Status()
		go resumeNotifications(s.State == Initial)
		s.State = Active
	}

	if s.closer != nil {
		close(s.closer)
		s.closer = nil
	}

	s.mu.Unlock()
}

func (s *state) handleTick(c <-chan struct{}) {
	defer log.Debug("Ticker handler exited.")
	t := time.NewTicker(PauseResolution)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			s.Status()
		case <-c:
			return
		}
	}
}

func (s *state) handleTimeout(c <-chan struct{}, after time.Duration) {
	defer log.Debug("Timeout handler exited.")
	t := time.NewTimer(after)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			s.Activate()
		case <-c:
			return
		}
	}
}

func (s *state) Status() {
	s.mu.RLock()
	switch s.State {
	case Paused:
		until := time.Until(s.Until)
		m := (until / time.Minute) + 1
		fmt.Printf("  (%dm)\n", m)
	case Active:
		fmt.Println("")
	}
	s.mu.RUnlock()
}

func main() {
	log.SetLevel(log.DebugLevel)
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	p := filepath.Join(usr.HomeDir, pipe)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		err := syscall.Mkfifo(p, 0600)
		if err != nil {
			log.Fatal(err)
		}
	}
	defer os.Remove(p)

	// File change notifications for the pipe, so that we know when to read.
	c := make(chan notify.EventInfo, 5)
	notify.Watch(p, c, notify.Write|notify.Remove)

	currentState := state{
		mu:    &sync.RWMutex{},
		State: Initial,
	}
	currentState.Activate()

	f, err := os.Open(p)
	if err != nil {
		log.Fatal("Could not open pipe for reading: ", err)
	}
	defer f.Close()

	var e notify.EventInfo
	for {
		select {
		case e = <-c:
			switch e.Event() {
			case notify.Write:
				log.Debugf("Pipe event: %v", e.Event().String())
				s := bufio.NewScanner(f)
				for s.Scan() {
					switch s.Text() {
					case "pause":
						currentState.Pause(time.Hour)
					case "resume":
						currentState.Activate()
					case "toggle":
						currentState.Toggle()
					default:
						log.Debugf("Unrecognized command: %v", s.Text())
					}
				}
			case notify.Remove:
				log.Fatalf("Pipe was removed. Exiting.")
			}
		}
	}
}

func resumeNotifications(quiet bool) {
	notification.NewNotification(Resume, "").Show()
	if !quiet {
		notification.Notification{
			AppIcon: "appointment-soon",
			Summary: "Notifications resumed",
			Timeout: notification.ExpiresDefault,
		}.Show()
	}
}

func pauseNotifications() {
	id, err := notification.Notification{
		AppIcon: "appointment-missed",
		Summary: "Pause notifications...",
		Body:    "Notifications will be paused for an hour...",
		Timeout: notification.ExpiresDefault,
	}.Show()

	if err == nil {
		<-time.After(5 * time.Second)
		notification.CloseNotification(id)
	}
	notification.NewNotification(Pause, "").Show()
}
