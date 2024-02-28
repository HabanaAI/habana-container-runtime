package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
)

var (
	debugflag  = flag.Bool("debug", false, "enable debug output")
	configflag = flag.String("config", "", "configuration file")

	defaultPATH = []string{"/usr/local/sbin", "/usr/local/bin", "/usr/sbin", "/usr/bin", "/sbin", "/bin"}
)

func exit() {
	if err := recover(); err != nil {
		if _, ok := err.(runtime.Error); ok {
			fmt.Fprintln(os.Stderr, err)
		}
		if *debugflag {
			fmt.Fprintf(os.Stderr, "%v\n", debug.Stack())
		}
		os.Exit(1)
	}
	os.Exit(0)
}

func getPATH(config CLIConfig) string {
	dirs := filepath.SplitList(os.Getenv("PATH"))
	// directories from the hook environment have higher precedence
	dirs = append(dirs, defaultPATH...)

	if config.Root != nil {
		rootDirs := []string{}
		for _, dir := range dirs {
			rootDirs = append(rootDirs, path.Join(*config.Root, dir))
		}
		// directories with the root prefix have higher precedence
		dirs = append(rootDirs, dirs...)
	}
	return strings.Join(dirs, ":")
}

func getCLIPath(config CLIConfig) (string, error) {
	if config.Path != nil {
		return *config.Path, nil
	}

	if err := os.Setenv("PATH", getPATH(config)); err != nil {
		return "", fmt.Errorf("couldn't set PATH variable: %w", err)
	}

	path, err := exec.LookPath("habana-container-cli")
	if err != nil {
		return "", fmt.Errorf("couldn't find binary habana-container-cli in $PATH (%s): %w", os.Getenv("PATH"), err)
	}
	return path, nil
}

// getRootfsPath returns an absolute path. We don't need to resolve symlinks for now.
func getRootfsPath(config containerConfig) string {
	rootfs, err := filepath.Abs(config.Rootfs)
	if err != nil {
		log.Panicln(err)
	}
	return rootfs
}

func doHook(lifecycle string) {
	var err error

	defer exit()
	log.SetFlags(0)

	hook := getHookConfig()
	cli := hook.HabanaContainerCLI

	container := getContainerConfig(hook)
	habana := container.Habana
	if habana == nil {
		// Not a HL devices, nothing to do.
		return
	}

	rootfs := getRootfsPath(container)

	args := []string{}
	if len(habana.Devices) > 0 {
		args = append(args, fmt.Sprintf("--device=%s", habana.Devices))
	}
	if cli.Root != nil {
		args = append(args, fmt.Sprintf("--root=%s", *cli.Root))
	}
	if cli.Debug != nil {
		args = append(args, fmt.Sprintf("--debug=%s", *cli.Debug))
	}
	if cli.MountAccelerators != nil {
		args = append(args, fmt.Sprintf("--mount-accelerators=%t", *cli.MountAccelerators))
	}
	if cli.MountUverbs != nil {
		args = append(args, fmt.Sprintf("--mount-uverbs=%t", *cli.MountUverbs))
	}

	args = append(args, fmt.Sprintf("--hook=%s", lifecycle))
	args = append(args, fmt.Sprintf("--pid=%s", strconv.FormatUint(uint64(container.Pid), 10)))
	args = append(args, rootfs)
	env := append(os.Environ(), cli.Environment...)

	cliPath, err := getCLIPath(cli)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cmd := exec.Command(cliPath, args...)
	cmd.Env = env
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(output))
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nCommands:\n")
	fmt.Fprintf(os.Stderr, "  prestart\n        run the prestart hook\n")
	fmt.Fprintf(os.Stderr, "  createRuntime\n        run the createRuntime hook\n")
	fmt.Fprintf(os.Stderr, "  poststart\n        no-op\n")
	fmt.Fprintf(os.Stderr, "  poststop\n        no-op\n")
}

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	switch args[0] {
	case "prestart", "createRuntime":
		doHook(args[0])
		os.Exit(0)
	case "poststart":
		fallthrough
	case "poststop":
		os.Exit(0)
	default:
		flag.Usage()
		os.Exit(2)
	}
}
