package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
	"github.com/spf13/cobra"
)

func buildCompleter(cmd *cobra.Command) *readline.PrefixCompleter {
	var items []readline.PrefixCompleterInterface
	for _, c := range cmd.Commands() {
		if c.Hidden {
			continue
		}
		if len(c.Commands()) > 0 {
			items = append(items, buildCompleter(c))
		} else {
			items = append(items, readline.PcItem(c.Name()))
		}
	}
	return readline.PcItem(cmd.Name(), items...)
}

func parseArgs(input string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false
	quoteChar := byte(0)

	for i := 0; i < len(input); i++ {
		c := input[i]
		if inQuotes {
			if c == quoteChar {
				inQuotes = false
			} else {
				current.WriteByte(c)
			}
		} else {
			switch c {
			case '"', '\'':
				inQuotes = true
				quoteChar = c
			case ' ', '\t':
				if current.Len() > 0 {
					args = append(args, current.String())
					current.Reset()
				}
			default:
				current.WriteByte(c)
			}
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

func RunREPL() error {
	historyFile := ""
	if usr, err := user.Current(); err == nil {
		dir := filepath.Join(usr.HomeDir, ".config", "zx")
		_ = os.MkdirAll(dir, 0700)
		historyFile = filepath.Join(dir, "history")
	}

	var rootItems []readline.PrefixCompleterInterface
	for _, c := range rootCmd.Commands() {
		if c.Hidden {
			continue
		}
		if len(c.Commands()) > 0 {
			rootItems = append(rootItems, buildCompleter(c))
		} else {
			rootItems = append(rootItems, readline.PcItem(c.Name()))
		}
	}
	completer := readline.NewPrefixCompleter(rootItems...)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "\033[36mzx>\033[0m ",
		HistoryFile:     historyFile,
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return fmt.Errorf("starting repl: %w", err)
	}
	defer rl.Close()

	fmt.Println("Welcome to zx interactive shell (Golang Zabbix CLI). Type 'help' or press Tab for commands, 'exit' to quit.")

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				continue
			}
			if err == io.EOF {
				break
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			break
		}

		args := parseArgs(line)
		if len(args) == 0 {
			continue
		}

		rootCmd.SetArgs(args)
		_ = rootCmd.ExecuteContext(context.Background())
	}
	return nil
}
