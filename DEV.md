Contents:
* [Runtime](#runtime)
* [Enable CDI in Containerd](#enable-cdi-in-containerd)
* [Generated source code](#generated-source-code)
  * [Required tools](#required-tools)


# Runtime

Runtime needs to have CDI injection support

- CRI-O: 1.23+, enabled by default.
- Containerd: v1.7+, disabled by default.

## Enable CDI in Containerd

Containerd config file should have `enable_cdi` and `cdi_spec_dirs`. Example `/etc/containerd/config.toml`:
```
version = 2
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    enable_cdi = true
    cdi_spec_dirs = ["/etc/cdi", "/var/run/cdi"]
```

### Determine your go binaries location from `go install --help`, quote:
> Executables are installed in the directory named by the GOBIN environment
> variable, which defaults to $GOPATH/bin or $HOME/go/bin if the GOPATH
> environment variable is not set. Executables in $GOROOT
> are installed in $GOROOT/bin or $GOTOOLDIR instead of $GOBIN.

### Way 1 : install tools with Go:

#### Add Go binaries directory to PATH
Add this to the end of your `$HOME/.bashrc`:
```bash
export PATH="<go binaries dir>:$PATH"
```

#### install tools
```bash
GO111MODULE=on go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
GO111MODULE=on go install k8s.io/code-generator/cmd/client-gen@latest
```

### Way 2 : clone and build it:
```bash
git clone https://github.com/kubernetes-sigs/controller-tools.git
cd controller-tools
go build ./cmd/controller-gen
cd -
git clone https://github.com/kubernetes/code-generator.git
cd code-generator
go build ./cmd/client-gen
cd -
```

Make them available in PATH, for instance $HOME/go/bin:
```bash
cp controller-tools/controller-gen code-generator/client-gen $HOME/go/bin
# ensure it's in the path. You may want to add export to $HOME/.bashrc
echo $PATH | grep -q $HOME/go/bin || export PATH=$HOME/go/bin:$PATH
```

## Running tests in container

To have your own user ID inside container image without access / permission issues, build a fresh
container image, then run tests. The CI uses its own user ID.

```shell
$ make test-image
$ make test-containerized
```

Tests provide coverage data. If you need to see the coverage report, just run Make target for needed
coverage target, e.g.

```
make gaudi-coverage
```
