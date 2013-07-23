package delayed_job

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"github.com/fd/go-shellwords/shellwords"
	"os"
	"os/exec"
	"strings"
	"sync"
	"text/template"
)

var default_directory = flag.String("exec.directory", ".", "the work directory for execute")

type execHandler struct {
	work_directory string
	prompt         string
	command        string
	arguments      []string
}

func newExecHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == ctx {
		return nil, errors.New("ctx is nil")
	}
	if nil == params {
		return nil, errors.New("params is nil")
	}

	work_directory := stringWithDefault(params, "work_directory", *default_directory)
	prompt := stringWithDefault(params, "prompt", "")
	command := stringWithDefault(params, "command", "")
	if 0 == len(command) {
		return nil, errors.New("'command' is required.")
	}

	if args, ok := params["arguments"]; ok {
		t, e := template.New("default").Parse(command)
		if nil != e {
			return nil, errors.New("create template failed, " + e.Error())
		}
		var buffer bytes.Buffer
		e = t.Execute(&buffer, args)
		if nil != e {
			return nil, errors.New("execute template failed, " + e.Error())
		}
		command = buffer.String()
	}

	arguments, e := shellwords.Split(command)
	if nil != e {
		return nil, errors.New("split shell command failed, " + e.Error())
	}

	return &execHandler{work_directory: work_directory, prompt: prompt, command: arguments[0], arguments: arguments[1:]}, nil
}

func (self *execHandler) Perform() error {
	cmd := exec.Command(self.command, self.arguments...)
	cmd.Dir = self.work_directory

	if 0 == len(self.prompt) {
		return cmd.Start()
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return errors.New("create pipe failed, " + err.Error())
	}
	defer func() {
		pr.Close()
		pw.Close()
	}()

	cmd.Stdout = pw
	cmd.Stderr = pw

	var scan_error error
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()

		buffer := bytes.NewBuffer(make([]byte, 0, 10240))
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), self.prompt) {
				return
			}
			buffer.Write(scanner.Bytes())

			if buffer.Len() > 10*1024*1024 {
				buffer.WriteString("\r\n ************************* read too large *************************\r\n")
				goto end
			}
		}
		buffer.WriteString("\r\n ************************* not found *************************\r\n")
	end:
		scan_error = errors.New(buffer.String())
	}()

	err = cmd.Run()
	if nil != err {
		return errors.New("start cmd failed, " + err.Error())
	}
	pw.Close()
	pr.Close()
	wait.Wait()

	return scan_error
}

func init() {
	Handlers["exec"] = newExecHandler
	Handlers["exec_command"] = newExecHandler
}