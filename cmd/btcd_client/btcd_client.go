package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
)

const (
	showHelpMessage = "Specify -h to show available options"
	listCmdMessage  = "Specify -l to list available commands"
)

var (
	defaultHomeDir = btcutil.AppDataDir("satsnet_btcd", false)
	currentNetwork = &chaincfg.SatsMainNetParams
	currentCfg     = &config{}
)

// commandUsage display the usage for a specific command.
func commandUsage(method string) {
	usage, err := btcjson.MethodUsageText(method)
	if err != nil {
		// This should never happen since the method was already checked
		// before calling this function, but be safe.
		fmt.Fprintln(os.Stderr, "Failed to obtain command usage:", err)
		return
	}

	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintf(os.Stderr, "  %s\n", usage)
}

// usage displays the general usage when the help flag is not displayed and
// and an invalid command was specified.  The commandUsage function is used
// instead when a valid command was specified.
func usage(errorMessage string) {
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	fmt.Fprintln(os.Stderr, errorMessage)
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintf(os.Stderr, "  %s [OPTIONS] <command> <args...>\n\n",
		appName)
	fmt.Fprintln(os.Stderr, showHelpMessage)
	fmt.Fprintln(os.Stderr, listCmdMessage)
}

func main() {
	fmt.Printf("Starting btcd command is %v\n", os.Args)
	// homeDir := flag.String("homedir", defaultHomeDir, "all data path")
	// flag.Parse()
	cfg, args, err := loadConfig()
	if err != nil {
		os.Exit(1)
	}

	currentCfg = cfg

	fmt.Printf("args is %v\n", args)
	// if len(args) < 1 {
	// 	usage("No command specified")
	// 	os.Exit(1)
	// }

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("\n\n\nwait for your input>>")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println(err)
			continue
		}

		words := strings.Fields(input)
		length := len(words)
		if length == 0 {
			continue
		}

		// Ensure the specified method identifies a valid registered command and
		// is one of the usable types.
		method := words[0]

		if method == "exit" {
			return
		} else if method == "anchortx" {
			lockedTTxid := ""
			if length > 1 {
				lockedTTxid = words[1]
			}
			testAnchorTx(lockedTTxid)
			continue
		}

		// Ensure the number of arguments are within bounds.
		usageFlags, err := btcjson.MethodUsageFlags(method)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unrecognized command '%s'\n", method)
			fmt.Fprintln(os.Stderr, listCmdMessage)
			//os.Exit(1)
			continue
		}
		if usageFlags&unusableFlags != 0 {
			fmt.Fprintf(os.Stderr, "The '%s' command can only be used via "+
				"websockets\n", method)
			fmt.Fprintln(os.Stderr, listCmdMessage)
			//os.Exit(1)
			continue
		}

		// Convert remaining command line words to a slice of interface values
		// to be passed along as parameters to new command creation function.
		//
		// Since some commands, such as submitblock, can involve data which is
		// too large for the Operating System to allow as a normal command line
		// parameter, support using '-' as an argument to allow the argument
		// to be read from a stdin pipe.
		bio := bufio.NewReader(os.Stdin)
		params := make([]interface{}, 0, len(words[1:]))
		for _, arg := range words[1:] {
			if arg == "-" {
				param, err := bio.ReadString('\n')
				if err != nil && err != io.EOF {
					fmt.Fprintf(os.Stderr, "Failed to read data "+
						"from stdin: %v\n", err)
					os.Exit(1)
				}
				if err == io.EOF && len(param) == 0 {
					fmt.Fprintln(os.Stderr, "Not enough lines "+
						"provided on stdin")
					os.Exit(1)
				}
				param = strings.TrimRight(param, "\r\n")
				params = append(params, param)
				continue
			}

			params = append(params, arg)
		}

		// Attempt to create the appropriate command using the arguments
		// provided by the user.
		cmd, err := btcjson.NewCmd(method, params...)
		if err != nil {
			// Show the error along with its error code when it's a
			// btcjson.Error as it reallistcally will always be since the
			// NewCmd function is only supposed to return errors of that
			// type.
			if jerr, ok := err.(btcjson.Error); ok {
				fmt.Fprintf(os.Stderr, "%s command: %v (code: %s)\n",
					method, err, jerr.ErrorCode)
				commandUsage(method)
				//os.Exit(1)
				continue
			}

			// The error is not a btcjson.Error and this really should not
			// happen.  Nevertheless, fallback to just showing the error
			// if it should happen due to a bug in the package.
			fmt.Fprintf(os.Stderr, "%s command: %v\n", method, err)
			commandUsage(method)
			//os.Exit(1)
			continue
		}

		// Marshal the command into a JSON-RPC byte slice in preparation for
		// sending it to the RPC server.
		marshalledJSON, err := btcjson.MarshalCmd(btcjson.RpcVersion1, 1, cmd)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			//os.Exit(1)
			continue
		}

		// Send the JSON-RPC request to the server using the user-specified
		// connection configuration.
		result, err := sendPostRequest(marshalledJSON, cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			//os.Exit(1)
			continue
		}

		// Choose how to display the result based on its type.
		strResult := string(result)
		if strings.HasPrefix(strResult, "{") || strings.HasPrefix(strResult, "[") {
			var dst bytes.Buffer
			if err := json.Indent(&dst, result, "", "  "); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to format result: %v",
					err)
				//os.Exit(1)
				continue
			}
			fmt.Println(dst.String())

		} else if strings.HasPrefix(strResult, `"`) {
			var str string
			if err := json.Unmarshal(result, &str); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to unmarshal result: %v",
					err)
				//os.Exit(1)
				continue
			}
			fmt.Println(str)

		} else if strResult != "null" {
			fmt.Println(strResult)
		}
	}
}
