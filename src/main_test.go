package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"
)

const (
	habanaRuntime      = "habana-container-runtime"
	habanaHook         = "habana-container-runtime-hook"
	bundlePath         = "./"
	specFile           = "config.json"
	unmodifiedSpecFile = "test_spec.json"
)

var workingDir string

func TestMain(m *testing.M) {
	// TEST SETUP

	// Confirm path setup correctly
	_, err := exec.LookPath("runc")
	if err != nil {
		log.Fatal("runc not found")
	}

	// RUN TESTS
	exitCode := m.Run()

	// TEST CLEANUP
	os.Remove(specFile)

	os.Exit(exitCode)
}

func TestBadInput(t *testing.T) {
	err := generateNewRuntimeSpec()
	if err != nil {
		t.Fatal(err)
	}

	cmdRun := exec.Command(habanaRuntime, "run", "--bundle")
	t.Logf("executing: %s\n", strings.Join(cmdRun.Args, " "))
	err = cmdRun.Run()
	require.Error(t, err, "runtime should return an error")

	cmdCreate := exec.Command(habanaRuntime, "create", "--bundle")
	t.Logf("executing: %s\n", strings.Join(cmdCreate.Args, " "))
	err = cmdCreate.Run()
	require.Error(t, err, "runtime should return an error")
}

func TestAddHabanaHook(t *testing.T) {
	err := generateNewRuntimeSpec()
	if err != nil {
		t.Fatal(err)
	}

	var spec specs.Spec
	spec, err = getRuntimeSpec(bundlePath + specFile)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("inserting habana prestart hook to config.json")
	if err = addHABANAHook(&spec); err != nil {
		t.Fatal(err)
	}
	path, err := exec.LookPath("habana-container-runtime-hook")
	require.NoError(t, err, "habana-container-runtime-hook not found")
	require.Equal(t, path, spec.Hooks.Prestart[0].Path, "bin path")
	require.Equal(t, []string{path, "prestart"}, spec.Hooks.Prestart[0].Args, "arguments")
}

func TestGetConfigWithCustomConfig(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	// By default debug is disabled
	contents := []byte("[habana-container-runtime]\ndebug = \"/habana-container-hook.log\"")
	testDir := path.Join(wd, "test")
	filename := path.Join(testDir, configFilePath)

	os.Setenv(configOverride, testDir)

	require.NoError(t, os.MkdirAll(filepath.Dir(filename), 0766))
	require.NoError(t, ioutil.WriteFile(filename, contents, 0766))

	defer func() { require.NoError(t, os.RemoveAll(testDir)) }()

	cfg, err := getConfig()
	require.NoError(t, err)
	require.Equal(t, cfg.debugFilePath, "/habana-container-hook.log")
}

func generateNewRuntimeSpec() error {
	cmd := exec.Command("cp", unmodifiedSpecFile, specFile)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func getRuntimeSpec(filePath string) (specs.Spec, error) {
	var spec specs.Spec
	jsonFile, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	defer jsonFile.Close()
	if err != nil {
		return spec, err
	}

	jsonContent, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return spec, err
	} else if json.Valid(jsonContent) {
		err = json.Unmarshal(jsonContent, &spec)
		if err != nil {
			return spec, err
		}
	} else {
		err = json.NewDecoder(bytes.NewReader(jsonContent)).Decode(&spec)
		if err != nil {
			return spec, err
		}
	}

	return spec, err
}
