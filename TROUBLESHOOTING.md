# Linsk troubleshooting tools

To aid with debugging/troubleshooting, Linsk is bundled with a few tools aimed to give you access to the virtual machine's internals.

### Error messages

By far, error messages is the single most helpful tool to help with problems of any kind. Before referring to anything, please analyze the errors and logs in detail.

**A great effort was put into ensuring that errors in Linsk are self-contained and provide enough information to understand what went wrong.** Please use them to your advantage.

### Shell

Linsk's shell is a powerful tool to assist with investigating issues inside the VM. Please refer to [SHELL.md](SHELL.md).

### `--vm-debug` flag

This flag is applicable in any `linsk` subcommand that starts a VM. Provided `--vm-debug`, Linsk will not start QEMU in headless mode and instead open a window with the virtual machine's display. **This is useful for investigating boot issues.**

# Common issues

TBD.