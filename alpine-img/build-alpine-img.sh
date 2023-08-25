# wget https://raw.githubusercontent.com/alpinelinux/alpine-make-vm-image/v0.11.1/alpine-make-vm-image \
#     && echo '0d5d3e375cb676d6eb5c1a52109a3a0a8e4cd7ac  alpine-make-vm-image' | sha1sum -c \
#     || exit 1

set -e

sudo bash alpine-make-vm-image -t \
    --image-format qcow2 \
    --repositories-file img/repositories \
    --packages "$(cat img/packages)" \
    --script-chroot \
    alpine.qcow2 -- ./img/configure.sh

sudo rmmod nbd