package plainTextNotebooks

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
	fs2 "github.com/stergiotis/boxer/public/fs"
	"github.com/stergiotis/boxer/public/observability/eh"
	"github.com/stergiotis/boxer/public/observability/eh/eb"
	"github.com/urfave/cli/v2"
)

//go:embed format.wl
var formatPlainTextNotebookCode string

type plainTextNotebooks struct {
	maxEvents     int
	dir           string
	nbRegexp      *regexp.Regexp
	wolframScript string
	newSuffix     string
	tryRun        bool
}

func newPlainTextNotebooks(maxEvents int, dir string, nbRegexp string, tryRun bool, wolframScript string, newSuffix string) (inst *plainTextNotebooks, err error) {
	var rgx *regexp.Regexp
	rgx, err = regexp.Compile(nbRegexp)
	if err != nil {
		err = eb.Build().Str("nbRegexp", nbRegexp).Errorf("unable to compile regexp: %w", err)
		return
	}

	inst = &plainTextNotebooks{
		maxEvents:     maxEvents,
		dir:           dir,
		nbRegexp:      rgx,
		wolframScript: wolframScript,
		newSuffix:     newSuffix,
		tryRun:        tryRun,
	}
	return
}
func (inst *plainTextNotebooks) formatPlainTextNotebook(nb string) {
	if inst.tryRun {
		log.Info().Str("path", nb).Msg("skipping notebook in try run mode")
		return
	}
	code := fmt.Sprintf(formatPlainTextNotebookCode, inst.nbRegexp.String(), inst.newSuffix)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	p := exec.CommandContext(ctx, inst.wolframScript, "-code", code, nb)
	buf := bytes.NewBuffer(make([]byte, 0, 4096))
	p.Stdin = nil
	p.Stderr = buf
	p.Stdout = buf
	err := p.Run()
	if err != nil {
		log.Warn().Str("code", code).Err(err).Str("stderr", buf.String()).Str("nb", nb).Msg("unable to format notebook")
		err = nil
	}
	log.Info().Str("nb", nb).Msg("formated notebook to plain text")
	return
}
func (inst *plainTextNotebooks) isOriginalNonPlainNotebook(path string) bool {
	return !strings.HasSuffix(path, inst.newSuffix) && inst.nbRegexp.MatchString(path)
}
func (inst *plainTextNotebooks) watch() (err error) {
	var watcher *fs2.BoundedFsWatcher
	watcher, err = fs2.NewBoundedFsWatcher(inst.maxEvents, func(event fsnotify.Event) (keep bool) {
		keep = (event.Op&fsnotify.Write != 0) && inst.isOriginalNonPlainNotebook(event.Name)
		return
	})
	if err != nil {
		err = eh.Errorf("unable to create file system watcher: %w", err)
		return
	}
	events := make([]fsnotify.Event, 0, inst.maxEvents)
	for {
		err = watcher.AddDirRecursive(os.DirFS(inst.dir), true)
		if err != nil {
			err = eb.Build().Str("dir", inst.dir).Errorf("unable to watch directory recursively")
			return
		}
		for i := 0; i < 100; i++ {
			events = watcher.GetAndClearEvents(events[:0])
			for _, p := range events {
				log.Info().Str("path", p.Name).Msg("detected change")
				inst.formatPlainTextNotebook(p.Name)
			}
			time.Sleep(time.Millisecond * 500)
		}
		err = watcher.ResetWatches()
		if err != nil {
			log.Warn().Err(err).Msg("unable to reset all watches, ignoring")
		}
		log.Trace().Msg("resetting watches to discover new directories")
	}
}
func (inst *plainTextNotebooks) singleShot() (err error) {
	f := os.DirFS(inst.dir)
	err = fs.WalkDir(f, ".", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			log.Debug().Str("path", path).Msg("inspecting directory")
		} else {
			if inst.isOriginalNonPlainNotebook(path) {
				log.Info().Str("path", path).Msg("found notebook")
				inst.formatPlainTextNotebook(path)
			}
		}
		return nil
	})
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
				Name:  "dir",
				Value: ".",
			},
			&cli.BoolFlag{
				Name:  "watch",
				Usage: "watch directory given by dir continuously and recursively",
				Value: false,
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
			o, err := newPlainTextNotebooks(context.Int("maxEvents"),
				context.String("dir"),
				context.String("notebookFileNameRegexp"),
				context.Bool("tryRun"),
				context.String("wolframscript"),
				context.String("plainTextSuffix"))
			if err != nil {
				return err
			}
			if context.Bool("watch") {
				return o.watch()
			}
			return o.singleShot()
		},
	}
}
