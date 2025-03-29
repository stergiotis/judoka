package plainTextNotebooks

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
	"github.com/stergiotis/boxer/public/fs"
	"github.com/stergiotis/boxer/public/observability/eh"
	"github.com/stergiotis/boxer/public/observability/eh/eb"
	"github.com/urfave/cli/v2"
)

func cleanupNotebook(nb string, wolframScript string, nbRegex string, newSuffix string) {
	code := fmt.Sprintf("nb=$ScriptCommandLine[[-1]];If[FileExistsQ[nb],ResourceFunction[\"SaveReadableNotebook\"][nb, StringReplace[nb, RegularExpression[\"%s\"] -> \"%s\"]]];", nbRegex, newSuffix)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	p := exec.CommandContext(ctx, wolframScript, "-code", code, nb)
	buf := bytes.NewBuffer(make([]byte, 0, 4096))
	p.Stdin = nil
	p.Stderr = buf
	p.Stdout = buf
	err := p.Run()
	if err != nil {
		log.Warn().Str("code", code).Err(err).Str("stderr", buf.String()).Str("nb", nb).Msg("unable to cleanup notebook")
		err = nil
	}
	return
}

func plainTextNotebooks(maxEvents int, dir string, nbRegexp string, tryRun bool, wolframScript string, newSuffix string) (err error) {
	var watcher *fs.BoundedFsWatcher
	var rgx *regexp.Regexp
	rgx, err = regexp.Compile(nbRegexp)
	if err != nil {
		err = eb.Build().Str("nbRegexp", nbRegexp).Errorf("unable to compile regexp: %w", err)
		return
	}
	watcher, err = fs.NewBoundedFsWatcher(maxEvents, func(event fsnotify.Event) (keep bool) {
		keep = (event.Op&fsnotify.Write != 0) && !strings.HasSuffix(event.Name, newSuffix) && rgx.MatchString(event.Name)
		return
	})
	if err != nil {
		err = eh.Errorf("unable to create file system watcher: %w", err)
		return
	}

	events := make([]fsnotify.Event, 0, maxEvents)
	for {
		err = watcher.AddDirRecursive(os.DirFS(dir), true)
		if err != nil {
			err = eb.Build().Str("dir", dir).Errorf("unable to watch directory recursively")
			return
		}
		for i := 0; i < 100; i++ {
			events = watcher.GetAndClearEvents(events[:0])
			for _, p := range events {
				log.Info().Str("path", p.Name).Msg("detected change")
				if !tryRun {
					cleanupNotebook(p.Name, wolframScript, nbRegexp, newSuffix)
				}
			}
			time.Sleep(time.Millisecond * 500)
		}
		err = watcher.ResetWatches()
		if err != nil {
			log.Warn().Err(err).Msg("unable to reset all watches, ignoring")
		}
		log.Trace().Msg("resetting watches to discover new directories")
	}
	return
}

func NewCommand() *cli.Command {
	return &cli.Command{
		Name: "plainTextNotebooks",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "maxEvents",
				Value: 0xffff,
			},
			&cli.StringFlag{
				Name:  "watchDir",
				Value: ".",
			},
			&cli.StringFlag{
				Name:  "notebookFileNameRegexp",
				Value: "[.]nb$",
			},
			&cli.BoolFlag{
				Name:  "tryRun",
				Value: false,
			},
			&cli.StringFlag{
				Name:  "wolframscript",
				Value: "wolframscript",
			},
			&cli.StringFlag{
				Name:  "plainTextSuffix",
				Value: ".plain.nb",
			},
		},
		Action: func(context *cli.Context) error {
			return plainTextNotebooks(context.Int("maxEvents"),
				context.String("watchDir"),
				context.String("notebookFileNameRegexp"),
				context.Bool("tryRun"),
				context.String("wolframscript"),
				context.String("plainTextSuffix"))
		},
	}
}
