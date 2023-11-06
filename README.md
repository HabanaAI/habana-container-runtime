# habana-container-runtime

A modified version of [runc](https://github.com/opencontainers/runc) adding a custom [pre-start hook](https://github.com/HabanaAI/habana-container-hook) to all containers
If environment variable `HABANA_VISIBLE_DEVICES` is set in the OCI spec, the hook will configure Habana device access for the container by leveraging `habana-container-cli` from project [libhabana-container](https://github.com/HabanaAI/libhabana-container).

## Installation

### Build from source
#### Ubuntu distributions

```
make docker-amd64
dpkg -i dist/ubuntu18.04/amd64/*.deb
```

#### CentOS and Amazon linux distributions
```
make docker-x86_64
# amazonlinux2
yum install dist/amazonlinux2/x86_64/*.rpm
# centos8
yum install dist/centos8/x86_64/*.rpm
```

### Install pre-built package
***NOTICE: package is not ready. Please use build from source and install as above***
#### Ubuntu distributions

1. download package from [here](http://TBD).
2. Install the `habana-container-runtime.deb` package:
```
sudo dpkg -i habana-container-runtime.deb
```

#### CentOS and Amazon linux distributions
1. download package from [here](http://TBD)
2. Install the `habana-container-runtime.rpm` package:
```
sudo yum install habana-container-runtime.rpm
```

To register the `habana` runtime, use the method below that is best suited to your environment.
You might need to merge the new argument with your existing configuration.

## Docker Engine setup

#### Daemon configuration file
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

You can optionally reconfigure the default runtime by adding the following to `/etc/docker/daemon.json`:
```
"default-runtime": "habana"
```
### Usage example


Currently habana-container-runtime has to be used with habana-container-hook and libhabana-container
Bellow is the case when host machine has 8 Habana devices and mount `all` by HABANA_VISIBLE_DEVICES=all
```bash
docker run --rm --runtime=habana -e HABANA_VISIBLE_DEVICES=all ubuntu:18.04 /bin/bash -c "ls /dev/accel/*"

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


#### Command line
```bash
sudo dockerd --add-runtime=habana=/usr/bin/habana-container-runtime [...]
```

## Environment variables (OCI spec)

Each environment variable maps to an command-line argument for `habana-container-cli` from [libhabana-container](https://github.com/HabanaAI/libhabana-container).

### `HABANA_VISIBLE_DEVICES`
This variable controls which Habana devices will be made accessible inside the container.

#### Possible values
* `0,1,2` …: a comma-separated list of index(es).
* `all`: all Habana devices will be accessible, this is the default value in our container images.

## ContainerD

#### Containerd configuration file
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

## Issues and Contributing

* Please let us know by [filing a new issue](https://github.com/HabanaAI/habana-container-runtime/issues/new)
* You can contribute by opening a [pull request](https://docs.github.com/en/github/collaborating-with-issues-and-pull-requests/about-pull-requests)
