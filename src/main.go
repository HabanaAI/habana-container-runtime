package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pelletier/go-toml"
)

const (
	configOverride = "XDG_CONFIG_HOME"
	configFilePath = "habana-container-runtime/config.toml"

	hookDefaultFilePath = "/usr/bin/habana-container-runtime-hook"
)

var (
	configDir    = "/etc/"
	execLookPath = exec.LookPath
	execRunc     = execRuncFunc
)

type config struct {
	debugFilePath string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}

func run(args []string) (err error) {
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("getting config: %w", err)
	}

	logFile, err := os.OpenFile(cfg.debugFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// If has 'create' command', then need to modify runc
	// otherwide, no modification needed
	if hasCreateCommand(args) {
		bundleDir, err := parseBundle(args)
		if err != nil {
			return err
		}
		logger.Debug("Bundle directory", "path", bundleDir)
		if err := modifySpec(logger, bundleDir); err != nil {
			return fmt.Errorf("modifing OCI spec: %w", err)
		}

		logger.Info("Prestart hook added, executing runc")
	}
	return execRunc(logger, args)
}

func getConfig() (*config, error) {
	cfg := &config{}

	if XDGConfigDir := os.Getenv(configOverride); len(XDGConfigDir) != 0 {
		configDir = XDGConfigDir
	}

	configFilePath := path.Join(configDir, configFilePath)

	toml, err := toml.LoadFile(configFilePath)
	if err != nil {
		return nil, err
	}

	cfg.debugFilePath = toml.GetDefault("habana-container-runtime.debug", "/dev/null").(string)

	return cfg, nil
}

func modifySpec(logger *slog.Logger, bundleDir string) error {

	bundleConfigFile := bundleDir + "/config.json"
	logger.Info("Using bundle file", "path", bundleConfigFile)

	jsonFile, err := os.OpenFile(filepath.Clean(bundleConfigFile), os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("opening OCI spec file: %w", err)
	}
	defer jsonFile.Close()

	jsonContent, err := io.ReadAll(jsonFile)
	if err != nil {
		return fmt.Errorf("reading OCI spec file: %w", err)
	}

	var spec specs.Spec
	err = json.Unmarshal(jsonContent, &spec)
	if err != nil {
		return fmt.Errorf("unmarshaling OCI spec file: %w", err)
	}

	err = addHabanaHook(logger, &spec)
	if err != nil {
		return fmt.Errorf("injecting Habana hook: %w", err)
	}

	jsonOutput, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshaling OCI spec: %w", err)
	}

	_, err = jsonFile.WriteAt(jsonOutput, 0)
	if err != nil {
		return fmt.Errorf("writing to OCI spec file: %w", err)
	}
	return nil
}

func execRuncFunc(logger *slog.Logger, args []string) error {
	logger.Debug("Looking for 'docker-runc' in PATH")
	runcPath, err := exec.LookPath("docker-runc")
	if err != nil {
		logger.Info(`"docker-runc" binary not found, looking for "runc" binary`)
		runcPath, err = exec.LookPath("runc")
		if err != nil {
			return err
		}
	}
	logger.Debug("runc path", "path", runcPath)
	return syscall.Exec(runcPath, append([]string{runcPath}, os.Args[1:]...), os.Environ())
}

func hasCreateCommand(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if arg == "create" {
			return true
		}
	}
	return false
}

func addHabanaHook(logger *slog.Logger, spec *specs.Spec) error {
	path, err := execLookPath("habana-container-runtime-hook")
	if err != nil {
		path = hookDefaultFilePath
		_, err = os.Stat(path)
		if err != nil {
			return err
		}
	}
	logger.Info("Prestart hook path", "path", path)

	args := []string{path}
	if spec.Hooks == nil {
		spec.Hooks = &specs.Hooks{}
	} else if len(spec.Hooks.Prestart) != 0 {
		for _, hook := range spec.Hooks.Prestart {
			if !strings.Contains(hook.Path, "habana-container-runtime-hook") {
				continue
			}
			logger.Info("Existing habana prestart hook in OCI spec file")
			return nil
		}
	}

	spec.Hooks.Prestart = append(spec.Hooks.Prestart, specs.Hook{
		Path: path,
		Args: append(args, "prestart"),
	})

	return nil
}
