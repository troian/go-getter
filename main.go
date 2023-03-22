package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-getter/gcs/v2"
	"github.com/hashicorp/go-getter/s3/v2"
	"github.com/hashicorp/go-getter/v2"

	"github.com/urfave/cli/v2"
)

const (
	flagMode                 = "mode"
	flagProgress             = "progress"
	flagDisableSymlinks      = "disable-symlinks"
	flagTimeout              = "timeout"
	flagTimeoutHeaders       = "timeout-headers"
	flagNetrc                = "netrc"
	flagMaxBytes             = "max-bytes"
	flagSkipHead             = "skip-head"
	flagXTerraformGetDisable = "x-terraform-get-disable"
	flagXTerraformGetLimit   = "x-terraform-get-limit"
)

var modeStrMap = map[string]getter.Mode{
	"any":  getter.ModeAny,
	"file": getter.ModeFile,
	"dir":  getter.ModeDir,
}

var modeValMap = map[getter.Mode]string{
	getter.ModeAny:  "any",
	getter.ModeFile: "file",
	getter.ModeDir:  "dir",
}

type modeValue struct {
	choices []string     // the choices that this value can take
	value   *getter.Mode // the actual value
}

func (v *modeValue) Set(value string) error {
	for _, choice := range v.choices {
		if strings.Compare(choice, value) == 0 {
			*v.value = modeStrMap[value]
			return nil
		}
	}
	return fmt.Errorf("%s is not a valid option. need %+v", value, v.choices)
}

func (v *modeValue) String() string {
	if v.value == nil {
		return ""
	}
	return modeValMap[*v.value]
}

func main() {
	var mode getter.Mode

	app := &cli.App{
		Name:      "go-getter",
		ArgsUsage: "go-getter <URL> <dst>",
		Before: func(c *cli.Context) error {
			if c.Args().Len() != 2 {
				return fmt.Errorf("invalid args\n")
			}
			return nil
		},
		Flags: []cli.Flag{
			&cli.GenericFlag{
				Name:  flagMode,
				Usage: "get mode",
				Value: &modeValue{
					choices: []string{"any", "file", "dir"},
					value:   &mode,
				},
				DefaultText: "any",
			},
			&cli.BoolFlag{
				Name: flagProgress,
			},
			&cli.BoolFlag{
				Name:  flagNetrc,
				Value: true,
			},
			&cli.BoolFlag{
				Name: flagDisableSymlinks,
			},
			&cli.BoolFlag{
				Name:  flagSkipHead,
				Value: false,
			},
			&cli.BoolFlag{
				Name:  flagXTerraformGetDisable,
				Value: true,
			},
			&cli.IntFlag{
				Name:  flagXTerraformGetLimit,
				Value: 0,
			},
			&cli.DurationFlag{
				Name:  flagTimeout,
				Value: 0,
			},
			&cli.DurationFlag{
				Name:  flagTimeoutHeaders,
				Value: 10 * time.Second,
			},
			&cli.Int64Flag{
				Name:  flagMaxBytes,
				Value: 0,
			},
		},
		Action: func(c *cli.Context) error {
			pwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("error getting wd: %s", err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Build the client
			req := &getter.Request{
				Src:             c.Args().Slice()[0],
				Dst:             c.Args().Slice()[1],
				Pwd:             pwd,
				GetMode:         mode,
				DisableSymlinks: c.Bool(flagDisableSymlinks),
			}

			if c.Bool(flagProgress) {
				req.ProgressListener = defaultProgressBar
			}

			client := getter.DefaultClient

			httpGetter := &getter.HttpGetter{
				Netrc:                 c.Bool(flagNetrc),
				XTerraformGetLimit:    c.Int(flagXTerraformGetLimit),
				XTerraformGetDisabled: c.Bool(flagXTerraformGetDisable),
				DoNotCheckHeadFirst:   c.Bool(flagSkipHead),
				HeadFirstTimeout:      c.Duration(flagTimeoutHeaders),
				ReadTimeout:           c.Duration(flagTimeout),
			}

			// The order of the Getters in the list may affect the result
			// depending on if the Request.Src is detected as valid by multiple getters
			client.Getters = []getter.Getter{
				&getter.GitGetter{
					Detectors: []getter.Detector{
						new(getter.GitHubDetector),
						new(getter.GitDetector),
						new(getter.BitBucketDetector),
						new(getter.GitLabDetector),
					},
				},
				new(getter.HgGetter),
				new(getter.SmbClientGetter),
				new(getter.SmbMountGetter),
				httpGetter,
				new(getter.FileGetter),
				new(gcs.Getter),
				new(s3.Getter),
			}

			res, err := client.Get(ctx, req)

			log.Printf("-> %s", res.Dst)

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Printf(err.Error())
		os.Exit(1)
	}
}
