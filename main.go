package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/mackerelio/checkers"
)

// version by Makefile
var version string

type cmdOpts struct {
	OptArgs       []string
	OptCommand    string
	OptIdentifier string `long:"identifier" description:"indetify a file store the command result with given string"`
	OptWarn       bool   `short:"w" long:"warn" description:"Set the error level to warning"`
	Version       bool   `short:"v" long:"version" description:"Show version"`
}

func runCmd(curFile *os.File, opts cmdOpts) error {
	cmd := exec.Command(opts.OptCommand, opts.OptArgs...)
	var stderr bytes.Buffer
	cmd.Stdout = curFile
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	err := cmd.Wait()
	if err != nil {
		return fmt.Errorf("%s - %s", err, stderr.String())
	}
	return nil
}

func calcDiff(diffCmd, from, to string) ([]byte, error) {
	return exec.Command(diffCmd, "-U", "1", from, to).Output()
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func checkDiff(opts cmdOpts) *checkers.Checker {
	diffCmd, err := exec.LookPath("diff")
	if err != nil {
		return checkers.Critical(err.Error())
	}

	tmpDir := os.TempDir()

	hasher := md5.New()
	hasher.Write([]byte(opts.OptIdentifier))
	hasher.Write([]byte(opts.OptCommand))
	for _, v := range opts.OptArgs {
		hasher.Write([]byte(v))
	}
	commandKey := fmt.Sprintf("%x", hasher.Sum(nil))

	curUser, _ := user.Current()
	prevPath := filepath.Join(tmpDir, "check-diff-"+curUser.Uid+"-"+commandKey)

	curFile, err := ioutil.TempFile(tmpDir, "temp")
	if err != nil {
		return checkers.Critical(err.Error())
	}

	defer os.Remove(curFile.Name())

	err = runCmd(curFile, opts)
	if err != nil {
		return checkers.Critical(err.Error())
	}

	if !fileExists(prevPath) {
		err = os.Rename(curFile.Name(), prevPath)
		if err != nil {
			return checkers.Critical(err.Error())
		}
		msg := ""
		if len(opts.OptArgs) > 0 {
			msg = fmt.Sprintf("first time execution command: '%s %s'", opts.OptCommand, strings.Join(opts.OptArgs, " "))
		} else {
			msg = fmt.Sprintf("first time execution command: '%s'", opts.OptCommand)
		}
		return checkers.Ok(msg)
	}

	// diff
	diffOut, diffError := calcDiff(diffCmd, prevPath, curFile.Name())
	err = os.Rename(curFile.Name(), prevPath)
	if err != nil {
		return checkers.Critical(err.Error())
	}

	if diffError == nil {
		// no difference
		curOpen, err := os.Open(curFile.Name())
		if err != nil {
			return checkers.Critical(err.Error())
		}
		defer curOpen.Close()

		fileinfo, _ := curOpen.Stat()
		data := make([]byte, 128)
		count, err := curOpen.Read(data)
		if err != nil {
			return checkers.Critical(err.Error())
		}
		cur := string(data[0:count])
		cur = regexp.MustCompile("(\r\n|\r|\n)$").ReplaceAllString(cur, "")
		msg := ""
		if fileinfo.Size() > 128 {
			msg = fmt.Sprintf("no difference: ```%s...```\n", cur)
		} else {
			msg = fmt.Sprintf("no difference: ```%s```\n", cur)
		}
		return checkers.Ok(msg)
	} else if regexp.MustCompile("exit status 1").MatchString(diffError.Error()) {
		// found diff
		diffRet := strings.Split(string(diffOut), "\n")
		diffRetString := strings.Join(diffRet[2:], "\n")
		diffRetString = regexp.MustCompile("(\r\n|\r|\n)$").ReplaceAllString(diffRetString, "")
		msg := ""
		if len(diffRetString) > 512 {
			msg = fmt.Sprintf("found difference: ```%s...```\n", diffRetString[0:512])
		} else {
			msg = fmt.Sprintf("found difference: ```%s```\n", diffRetString)
		}
		if opts.OptWarn {
			return checkers.Warning(msg)
		}
		return checkers.Critical(msg)
	}
	return checkers.Critical(diffError.Error())
}

func printVersion() {
	fmt.Printf(`%s %s
Compiler: %s %s
`,
		os.Args[0],
		version,
		runtime.Compiler,
		runtime.Version())
}

func main() {
	opts := cmdOpts{}
	psr := flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash)
	psr.Usage = "[OPTIONS] -- command args1 args2"
	args, err := psr.Parse()
	if opts.Version {
		printVersion()
		os.Exit(0)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	opts.OptCommand = args[0]
	if len(args) > 1 {
		opts.OptArgs = args[1:]
	}

	ckr := checkDiff(opts)
	ckr.Name = "check-diff"
	ckr.Exit()
}
