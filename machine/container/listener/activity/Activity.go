package activity

import (
	"github.com/thanhpk/randstr"
	"os/exec"
	"regexp"
	"strconv"
	"supervisor/machine/container/listener"
	"supervisor/machine/container/listener/event"
)

type Activity struct {
	Command             *exec.Cmd
	Description         string
	DescriptionRegex    *regexp.Regexp
	DescriptionIndex    int
	ProgressRegex       *regexp.Regexp
	ProgressIndex       int
	HeadSha             *string
	Type                string
	progress            int
	originalDescription string
	id                  string
}

func (a *Activity) Exec(handler *listener.Handler) (err error) {
	a.id = randstr.Hex(8)
	a.progress = 0
	a.originalDescription = a.Description
	a.Forward(handler, false, false, true)

	stdout, err := a.Command.StdoutPipe()
	if err != nil {
		return err
	}
	a.Command.Stderr = a.Command.Stdout

	// Start the command
	if err := a.Command.Start(); err != nil {
		return err
	}

	go func() {
		for {
			buffer := make([]byte, 1024)
			n, err := stdout.Read(buffer)
			if err != nil {
				break
			} else if n > 0 {
				match := a.ProgressRegex.FindStringSubmatch(string(buffer[:n]))
				if len(match) > a.ProgressIndex {
					newProgress := match[1]
					newProgressNumeric, err := strconv.Atoi(newProgress)
					if err == nil && a.progress != newProgressNumeric {
						if a.DescriptionRegex != nil {
							descriptionMatch := a.DescriptionRegex.FindStringSubmatch(string(buffer[:n]))
							if len(descriptionMatch) > a.DescriptionIndex {
								a.Description = a.originalDescription + " (" + descriptionMatch[a.DescriptionIndex] + ")"
							}
						}
						a.progress = newProgressNumeric
						a.Forward(handler, false, false, false)
					}
				}

			}
		}
	}()

	if err := a.Command.Wait(); err != nil {
		a.Forward(handler, true, true, false)
		return err
	} else {
		a.progress = 100
		a.Forward(handler, true, false, false)
	}

	return nil

}

func (a *Activity) Forward(handler *listener.Handler, finished bool, errored bool, started bool) {
	progress := event.ProgressUpdate{
		Id:          a.id,
		Description: a.Description,
		Started:     started,
		Finished:    finished,
		Errored:     errored,
		Progress:    a.progress,
		HeadSha:     a.HeadSha,
		Type:        a.Type,
	}
	handler.ProgressCache[a.id] = progress
	enc, err := progress.Encode()
	if err != nil {
		return
	}
	_ = handler.HandleEvent(event.Progress, enc, false)
	if started || finished {
		_ = handler.HandleEvent(event.Progress, enc, true)
	}
	if finished {
		delete(handler.ProgressCache, a.id)
	}
}
