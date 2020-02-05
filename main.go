package main

import (
	"bufio"
	notification "github.com/TheCreeper/go-notify"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
	"time"
)

const (
	pipe   = ".dunst-pause"
	Pause  = "DUNST_COMMAND_PAUSE"
	Resume = "DUNST_COMMAND_RESUME"
)

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

	f, err := os.Open(p)
	if err != nil {
		log.Fatal("could not open pipe for reading: ", err)
	}
	defer f.Close()

	var e notify.EventInfo
	var tChan <-chan time.Time
	for {
		select {
		case e = <-c:
			tChan = handle(e.Event(), f)
		case <-tChan:
			log.Debug("Shit expired")
			resumeNotifications()
		}
	}
}

func handle(e notify.Event, r io.Reader) <-chan time.Time {
	switch e {
	case notify.Write:
		log.Debug("Write event. Read full pipe")
		s := bufio.NewScanner(r)
		for s.Scan() {
			switch s.Text() {
			case "pause":
				go pauseNotifications()
				return time.NewTimer(time.Second * 10).C
			case "resume":
				go resumeNotifications()
			}
		}
	case notify.Remove:
		log.Fatalf("Pipe was removed. Exiting.")
	}

	return nil
}

func resumeNotifications() {
	notification.NewNotification(Resume, "").Show()
	notification.NewNotification("Notifications resumed", "").Show()
}

func pauseNotifications() {
	id, err := notification.NewNotification("Pause notifications...", "Notifications will be paused for an hour...").Show()
	if err == nil {
		<-time.After(3 * time.Second)
		notification.CloseNotification(id)
	}
	notification.NewNotification(Pause, "").Show()
}
