# dunst-pause
Small daemon to manage pausing/resuming dunst notifications

## Reasons for existing
I've been missing a way to reliably pause/resume notifications for dunst. Mostly the problem has been that when I pause them, I forget to re-enable them later on, but also that I've not been able to at a glance figure out if notifications are currently paused or not. The intent with this daemon is to:
* Track the current state of notifications
* Re-enable the notifications after a timeout
* Output current state for use in eg. i3bar

## Usage
The daemon will create a named pipe at `~/.dunst-pause` that can be written to to control pausing/resuming dunst notifications. Write `pause` to pause the notifications, and `resume` to resume them. 
