# Habana-Container-Runtime

A modified version of [runc](https://github.com/opencontainers/runc) integrates a customized [pre-start hook](https://github.com/HabanaAI/habana-container-hook) into all containers.
If the environment variable `HABANA_VISIBLE_DEVICES` is set in the OCI specification, this hook will configure Habana device access for the container by editing the container bundle specification and using `habana-container-cli` to expose the devices' network ports.

- [Habana-Container-Runtime](#habana-container-runtime)
  - [Building from source](#building-from-source)
    - [Build binaries](#build-binaries)
    - [Building Packages From Source](#building-packages-from-source)
  - [Installing a Pre-built Package](#installing-a-pre-built-package)
  - [Docker Engine Configuration](#docker-engine-configuration)
      - [Daemon Configuration File](#daemon-configuration-file)
  - [ContainerD Setup](#containerd-setup)
    - [Containerd Configuration File](#containerd-configuration-file)
  - [CRI-O Setup](#cri-o-setup)
    - [CRI-O Configuration File](#cri-o-configuration-file)
  - [Usage example](#usage-example)
  - [Environment variables (OCI spec)](#environment-variables-oci-spec)
    - [`HABANA_VISIBLE_DEVICES`](#habana_visible_devices)
    - [`HABANA_RUNTIME_ERROR` **Auto generated**](#habana_runtime_error-auto-generated)
  - [Config](#config)
  - [Issues and Contributing](#issues-and-contributing)

## Building from source

All binaries are built under dist/{BINARY_NAME}_architecture/{BINARY_NAME}.

Available architectures:
- linux_amd64
- linux_386
- linux_arm64

### Build binaries

```bash
# Build all binaries
make build-binary

# Build only habana-container-runtime
make build-runtime

# Build only habana-container-hook
make build-hook

# Build only habana-container-cli (libhabana-container)
make build-cli
```

After building the binaries, copy the config from `packaging/config.toml`
into `/etc/habana-container-runtime/config.toml` and edit for your
environment.

### Building Packages From Source

You must have docker installed. Building the packages is done using goreleaser.

```bash
make release
```

Artifacts are found under `dist/` folder.

## Installing a Pre-built Package

Installation and usage guides available in [habana.ai docs](https://docs.habana.ai/en/latest/Installation_Guide/Bare_Metal_Fresh_OS.html#set-up-container-usage)


## Docker Engine Configuration

#### Daemon Configuration File

```bash
sudo tee /etc/docker/daemon.json <<EOF
{
    "runtimes": {
        "habana": {
            "path": "/usr/bin/habana-container-runtime",
            "runtimeArgs": []
        }
    }
}
EOF
sudo systemctl restart docker
```

Optionally, you can adjust the default runtime by appending the following to `/etc/docker/daemon.json`:

```
"default-runtime": "habana"
```

## ContainerD Setup

### Containerd Configuration File

```bash
sudo tee /etc/containerd/config.toml <<EOF
disabled_plugins = []
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "habana"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.habana]
          runtime_type = "io.containerd.runc.v2"
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.habana.options]
            BinaryName = "/usr/bin/habana-container-runtime"
  [plugins."io.containerd.runtime.v1.linux"]
    runtime = "habana-container-runtime"
EOF
sudo systemctl restart containerd
```

## CRI-O Setup

Create new config file at `/etc/crio/crio.conf.d/99-habana-ai.conf`.

### CRI-O Configuration File

```toml
[crio.runtime]
default_runtime = "habana-ai"

[crio.runtime.runtimes.habana-ai]
runtime_path = "/usr/local/habana/bin/habana-container-runtime"
monitor_env = [
        "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
]
```

Restart crio service: `systemctl restart crio.service`

## Usage example

Currently, `habana-container-runtime` must be used with `habana-container-hook` and libhabana-container.

The example below assumes a host machine featuring 8 Habana devices, and demonstrates how to mount all of them inside a container. This is done via `HABANA_VISIBLE_DEVICES=all` environment variable.

```bash
docker run --rm --runtime=habana -e HABANA_VISIBLE_DEVICES=all ubuntu:22.04 /bin/bash -c "ls /dev/accel/*"

/dev/accel/accel0
/dev/accel/accel1
/dev/accel/accel2
/dev/accel/accel3
/dev/accel/accel4
/dev/accel/accel5
/dev/accel/accel6
/dev/accel/accel7
/dev/accel/accel_controlD0
/dev/accel/accel_controlD1
/dev/accel/accel_controlD2
/dev/accel/accel_controlD3
/dev/accel/accel_controlD4
/dev/accel/accel_controlD5
/dev/accel/accel_controlD6
/dev/accel/accel_controlD7
```


## Environment variables (OCI spec)

Each environment variable maps to an command-line argument for `habana-container-cli` from [libhabana-container](https://github.com/HabanaAI/libhabana-container).

### `HABANA_VISIBLE_DEVICES`
The variable determines which Habana devices will be accessible within the container.

**Possible values**
* `0,1,2` …: a comma-separated list of index(es).
* `all`: all Habana devices will be accessible, this is the default value in our container images.


### `HABANA_RUNTIME_ERROR` **Auto generated**
The variable holds the last error from the runtime flow. The runtime
does not fail the pod creation in most cases, so the error is propagated inside the container for debugging purposes.


## Config

See options [here](./packaging/config.toml)

## Issues and Contributing

* Please let us know by [filing a new issue](https://github.com/HabanaAI/habana-container-runtime/issues/new)
* You can contribute by opening a [pull request](https://docs.github.com/en/github/collaborating-with-issues-and-pull-requests/about-pull-requests)
