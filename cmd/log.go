package cmd

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/ViBiOh/kmux/pkg/log"
	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/ViBiOh/kmux/pkg/resource"
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
	Use:     "log TYPE NAME",
	Aliases: []string{"logs"},
	Short:   "Get logs of a given resource",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{
				"cronjobs",
				"daemonsets",
				"deployments",
				"jobs",
				"namespaces",
				"nodes",
				"pods",
				"services",
			}, cobra.ShellCompDirectiveNoFileComp
		}

		if len(args) == 1 {
			lister, err := resource.ListerFor(args[0])
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}

			clients, err = getKubernetesClient(strings.Split(viper.GetString("context"), ","))
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}

			return listObjects(cmd.Context(), viper.GetString("namespace"), lister), cobra.ShellCompDirectiveNoFileComp
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if !(len(args) == 2 || len(labelsSelector) != 0 || (len(args) == 1 && slices.Contains([]string{"ns", "namespace", "namespaces"}, args[0]))) {
			return errors.New("either labels or `TYPE NAME` args must be specified")
		}

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		go func() {
			waitForEnd(syscall.SIGINT, syscall.SIGTERM)
			cancel()
		}()

		if len(container) != 0 {
			var err error

			containerRegexp, err = regexp.Compile(container)
			if err != nil {
				return fmt.Errorf("container filter compile: %w", err)
			}
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
			logColorFilter = log.ColorFromName(strings.ToLower(grepColor))
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

func initLog() {
	flags := logCmd.Flags()

	flags.DurationVarP(&since, "since", "s", time.Hour, "Display logs since given duration")
	flags.StringVarP(&container, "container", "c", "", "Filter container's name by regexp, default to all containers")

	flags.BoolVarP(&dryRun, "dry-run", "d", false, "Dry-run, print only pods")
	flags.BoolVarP(&rawOutput, "raw-output", "r", false, "Raw ouput, don't print context or pod prefixes")

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
