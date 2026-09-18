package cmd

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ViBiOh/kmux/pkg/log"
	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	dryRun    bool
	rawOutput bool

	noFollow bool

	since          time.Duration
	labelsSelector map[string]string

	jsonColorKeys []string

	logFilters []string
	invertGrep bool

	logColorFilter *color.Color
)

var logCmd = &cobra.Command{
	Use:               "log TYPE NAME",
	Aliases:           []string{"logs"},
	Short:             "Get logs of a given resource",
	ValidArgsFunction: resourceCompletion(logKinds...),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !hasLogTarget(args) {
			return errors.New("either labels, `TYPE NAME` or a namespace must be specified")
		}

		ctx, cancel := commandContext(cmd)
		defer cancel()

		if err := compileContainerFilter(); err != nil {
			return err
		}

		logRegexes := make([]*regexp.Regexp, len(logFilters))

		for index, logFilter := range logFilters {
			var err error

			logRegexes[index], err = regexp.Compile(logFilter)
			if err != nil {
				return fmt.Errorf("compile log filter `%s`: %w", logFilter, err)
			}
		}

		if grepColor := viper.GetString("grepColor"); len(grepColor) != 0 {
			if logColorFilter = log.ColorFromName(strings.ToLower(grepColor)); logColorFilter == nil {
				return fmt.Errorf("unknown color `%s`, expected one of %s", grepColor, strings.Join(log.ColorNames(), ", "))
			}
		}

		if levelKeys := viper.GetStringSlice("levelKeys"); len(levelKeys) != 0 {
			jsonColorKeys = append(jsonColorKeys, levelKeys...)
		}

		if statusCodeKeys := viper.GetStringSlice("statusCodeKeys"); len(statusCodeKeys) != 0 {
			jsonColorKeys = append(jsonColorKeys, statusCodeKeys...)
		}

		var kind, name string
		if len(args) > 1 {
			kind = args[0]
			name = args[1]
		}

		logger := log.NewLogger(kind, name, labelsSelector, since).
			WithDryRun(dryRun).
			WithContainerRegexp(containerRegexp).
			WithNoFollow(noFollow).
			WithLogRegexes(logRegexes).
			WithInvertRegexp(invertGrep).
			WithColorFilter(logColorFilter).
			WithJsonColorKeys(jsonColorKeys).
			WithRawOutput(rawOutput)

		clients.Execute(ctx, logger.Log)

		return nil
	},
}

func hasLogTarget(args []string) bool {
	switch {
	case len(args) == 2, len(labelsSelector) != 0:
		return true

	case len(args) == 1:
		return slices.Contains([]string{"ns", "namespace", "namespaces"}, args[0])

	default:
		return false
	}
}

func initLog() {
	flags := logCmd.Flags()

	flags.DurationVarP(&since, "since", "s", time.Hour, "Display logs since given duration")
	flags.StringVarP(&container, "container", "c", "", "Filter container's name by regexp, default to all containers")

	flags.BoolVarP(&dryRun, "dry-run", "d", false, "Dry-run, print only pods")
	flags.BoolVarP(&rawOutput, "raw-output", "r", false, "Raw output, don't print context or pod prefixes")

	flags.BoolVarP(&noFollow, "no-follow", "", false, "Don't follow logs")

	flags.StringToStringVarP(&labelsSelector, "selector", "l", nil, "Labels to filter pods")

	flags.StringArrayVarP(&logFilters, "grep", "g", nil, "Regexp to filter log")
	flags.BoolVarP(&invertGrep, "invert-match", "v", false, "Invert regexp filter matching")

	flags.String("grepColor", "", "Get logs only above given color (red > yellow > green)")
	if err := viper.BindPFlag("grepColor", flags.Lookup("grepColor")); err != nil {
		output.Fatal("bind `grepColor` flag: %s", err)
	}

	flags.StringSlice("levelKeys", []string{"level", "severity"}, "Keys for level in JSON")
	if err := viper.BindPFlag("levelKeys", flags.Lookup("levelKeys")); err != nil {
		output.Fatal("bind `levelKeys` flag: %s", err)
	}

	flags.StringSlice("statusCodeKeys", []string{"status", "statusCode", "response_code", "http_status", "OriginStatus"}, "Keys for HTTP Status code in JSON")
	if err := viper.BindPFlag("statusCodeKeys", flags.Lookup("statusCodeKeys")); err != nil {
		output.Fatal("bind `statusCodeKeys` flag: %s", err)
	}
}
