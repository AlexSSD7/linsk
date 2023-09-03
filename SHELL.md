# Linsk shell

`linsk shell` will start a VM and open a full-fledged Linux shell for you. Linsk will not mount nor configure any file server.

Linsk's VM runs Alpine Linux, a lightweight busybox-based Linux distribution. Upon the startup, you will find little to no preinstalled tools. This is intentional as the goal is to have the lightest VM possible. There is no default text editor preinstalled either. However, Linsk's Alpine Linux is supplied with `apk` package manager. You can use it to install packages of any kind. An installation of `vim`, for example, would mean running `apk add vim`.

Linsk's shell can be used to format disks with tools like `mkfs` and run diagnostics with tools like `fsck`. Please note that you will need to install these separately using Alpine Linux's `apk` package manager. The other important purpose of Linsk's shell is to assist with troubleshooting.

# Access the shell within `linsk run`

In addition to `linsk shell` that can be used to help investigate disk-related issues, Linsk also provides a `--debug-shell` CLI flag for the `run` command. Unlike `linsk shell` , `linsk run --debug-shell` will start the shell *after* starting a network share. This is useful for investigating file share issues.

Please note that by default, networking is restricted when `linsk run --debug-shell` is run. You will need to add `--vm-unrestricted-networking` flag in order to be able to access the outside network, which is commonly needed to install packages with `apk`.