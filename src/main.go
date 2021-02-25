package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pelletier/go-toml"
)

const (
	configOverride      = "XDG_CONFIG_HOME"
	configFilePath      = "habana-container-runtime/config.toml"

	hookDefaultFilePath = "/usr/bin/habana-container-runtime-hook"
)

var (
	configDir           = "/etc/"
)

var fileLogger *log.Logger = nil

type args struct {
	bundleDirPath string
	cmd           string
}

type config struct {
	debugFilePath string
}

func getConfig() (*config, error) {
	cfg := &config{}

	if XDGConfigDir := os.Getenv(configOverride); len(XDGConfigDir) != 0 {
		configDir = XDGConfigDir
	}

	configFilePath := path.Join(configDir, configFilePath)

	tomlContent, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}

	toml, err := toml.Load(string(tomlContent))
	if err != nil {
		return nil, err
	}

	cfg.debugFilePath = toml.GetDefault("habana-container-runtime.debug", "/dev/null").(string)

	return cfg, nil
}

func getArgs() (*args, error) {
	args := &args{}

	for i, param := range os.Args {
		if param == "--bundle" || param == "-b" {
			if len(os.Args) - i <= 1 {
				return nil, fmt.Errorf("bundle option needs an argument")
			}
			args.bundleDirPath = os.Args[i + 1]
		} else if param == "create" {
			args.cmd = param
		}
	}

	return args, nil
}

func exitOnError(err error, msg string) {
	if err != nil {
		if fileLogger != nil {
			fileLogger.Printf("ERROR: %s: %v\n", msg, err)
		}
		log.Fatalf("ERROR: %s: %s: %v\n", os.Args[0], msg, err)
	}
}

func execRunc() {
	fileLogger.Println("Looking for \"docker-runc\" binary")
	runcPath, err := exec.LookPath("docker-runc")
	if err != nil {
		fileLogger.Println("\"docker-runc\" binary not found")
		fileLogger.Println("Looking for \"runc\" binary")
		runcPath, err = exec.LookPath("runc")
		exitOnError(err, "find runc path")
	}

	fileLogger.Printf("Runc path: %s\n", runcPath)

	err = syscall.Exec(runcPath, append([]string{runcPath}, os.Args[1:]...), os.Environ())
	exitOnError(err, "exec runc binary")
}

func addHABANAHook(spec *specs.Spec) error {
	path, err := exec.LookPath("habana-container-runtime-hook")
	if err != nil {
		path = hookDefaultFilePath
		_, err = os.Stat(path)
		if err != nil {
			return err
		}
	}

	if fileLogger != nil {
		fileLogger.Printf("prestart hook path: %s\n", path)
	}

	args := []string{path}
	if spec.Hooks == nil {
		spec.Hooks = &specs.Hooks{}
	} else if len(spec.Hooks.Prestart) != 0 {
		for _, hook := range spec.Hooks.Prestart {
			if !strings.Contains(hook.Path, "habana-container-runtime-hook") {
				continue
			}
			if fileLogger != nil {
				fileLogger.Println("existing habana prestart hook in OCI spec file")
			}
			return nil
		}
	}

	spec.Hooks.Prestart = append(spec.Hooks.Prestart, specs.Hook{
		Path: path,
		Args: append(args, "prestart"),
	})

	return nil
}

func main() {
	cfg, err := getConfig()
	exitOnError(err, "fail to get config")

	logFile, err := os.OpenFile(cfg.debugFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	exitOnError(err, "fail to open debug log file")
	defer logFile.Close()

	fileLogger = log.New(logFile, "", log.LstdFlags)
	fileLogger.Printf("Running %s\n", os.Args[0])

	args, err := getArgs()
	exitOnError(err, "fail to get args")

	if args.cmd != "create" {
		fileLogger.Println("Command is not \"create\", executing runc doing nothing")
		execRunc()
		log.Fatalf("ERROR: %s: fail to execute runc binary\n", os.Args[0])
	}

	if args.bundleDirPath == "" {
		args.bundleDirPath, err = os.Getwd()
		exitOnError(err, "get working directory")
		fileLogger.Printf("Bundle dirrectory path is empty, using working directory: %s\n", args.bundleDirPath)
	}

	fileLogger.Printf("Using bundle file: %s\n", args.bundleDirPath + "/config.json")
	jsonFile, err := os.OpenFile(args.bundleDirPath + "/config.json", os.O_RDWR, 0644)
	exitOnError(err, "open OCI spec file")

	defer jsonFile.Close()

	jsonContent, err := ioutil.ReadAll(jsonFile)
	exitOnError(err, "read OCI spec file")

	var spec specs.Spec
	err = json.Unmarshal(jsonContent, &spec)
	exitOnError(err, "unmarshal OCI spec file")

	err = addHABANAHook(&spec)
	exitOnError(err, "inject HABANA hook")

	jsonOutput, err := json.Marshal(spec)
	exitOnError(err, "marchal OCI spec file")

	_, err = jsonFile.WriteAt(jsonOutput, 0)
	exitOnError(err, "write OCI spec file")

	fileLogger.Print("Prestart hook added, executing runc")
	execRunc()
}
