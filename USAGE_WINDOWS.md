# Linsk Windows usage instructions

In this document, you will find instructions on how to get started using Linsk on Windows.

# How Linsk works
As you probably have realized from the initial README, Linsk takes use of a lightweight Alpine Linux virtual machine to tap into the rich world of Linux filesystems.

Linsk will pass through the disk as a raw block device to an ephemeral virtual machine, set up a file share, and then expose it to your host computer, along with logging the file share connection details. It's as simple as that.

# Use Linsk

## Step 0. The first-time Linsk initialization

To use Linsk, you will need to build a virtual machine image to use. Doing this is as easy as running the following command:
```powershell
linsk build
```

This will take a minute or two. This is what you will see:
```
# linsk command output
time=2023-09-03T10:33:07.993+01:00 level=INFO msg="Starting to download file" caller=storage from=https://dl-cdn.alpinelinux.org/alpine/v3.18/releases/x86_64/alpine-virt-3.18.3-x86_64.iso to=C:\Users\Alex\Linsk\alpine-3.18.3-x86_64.img
time=2023-09-03T10:33:10.506+01:00 level=INFO msg="Successfully downloaded file" caller=storage from=https://dl-cdn.alpinelinux.org/alpine/v3.18/releases/x86_64/alpine-virt-3.18.3-x86_64.iso to=C:\Users\Alex\Linsk\alpine-3.18.3-x86_64.img out-size="58 MB"
time=2023-09-03T10:33:10.506+01:00 level=INFO msg="Building VM image" caller=storage tags=3.18.3-x86_64 overwriting=false dst=C:\Users\Alex\Linsk\3.18.3-x86_64-linsk1.qcow2
time=2023-09-03T10:33:10.537+01:00 level=WARN msg="Using unrestricted VM networking" caller=storage subcaller=imgbuilder subcaller=vm
time=2023-09-03T10:33:10.538+01:00 level=INFO msg="Booting the VM" caller=storage subcaller=imgbuilder subcaller=vm
time=2023-09-03T10:33:15.546+01:00 level=INFO msg="The VM is up, setting it up" caller=storage subcaller=imgbuilder subcaller=vm
time=2023-09-03T10:33:20.814+01:00 level=INFO msg="The VM is ready" caller=storage subcaller=imgbuilder subcaller=vm
time=2023-09-03T10:33:20.845+01:00 level=INFO msg="VM OS installation in progress" caller=storage subcaller=imgbuilder
time=2023-09-03T10:33:31.320+01:00 level=WARN msg="Canceling the VM context" caller=storage subcaller=imgbuilder subcaller=vm
time=2023-09-03T10:33:31.350+01:00 level=WARN msg="Sending poweroff command to the VM" caller=storage subcaller=imgbuilder subcaller=vm
time=2023-09-03T10:33:31.382+01:00 level=INFO msg="Shutting the VM down safely" caller=storage subcaller=imgbuilder subcaller=vm
time=2023-09-03T10:33:31.718+01:00 level=INFO msg="Removed base image" caller=storage path=C:\Users\Alex\Linsk\alpine-3.18.3-x86_64.img
time=2023-09-03T10:33:31.718+01:00 level=INFO msg="VM image built successfully" path=C:\Users\Alex\Linsk\3.18.3-x86_64-linsk1.qcow2
```

**NOTE:** Building a VM image requires an internet connection. After the initial image build is done, you can use Linsk offline.

## Step 1. Select the drive you want to pass through

Find the path of the physical drive you want to pass through by executing the following command:
```powershell
wmic diskdrive list brief
```

Find your disk, and take note of the disk path that looks like `\\.\PhysicalDriveX` (where X is a number). We will need this in the next step.

**IMPORTANT NOTE:** Please ensure that the physical device you are trying to pass through is not mounted anywhere else on the host machine. Otherwise, you run serious risks. No further warnings will be issued.

## Step 2. Use `linsk ls` to see what partitions are available in the VM

Run `linsk ls` while specifying the block device path you obtained in the previous step:
```powershell
# This should be run in a terminal open with administrator privileges.
linsk ls dev:\\.\PhysicalDriveX
```

You will then see something like this:
```
# linsk command output
time=2023-09-03T10:37:35.728+01:00 level=WARN msg="Using raw block device passthrough. Please note that it's YOUR responsibility to ensure that no device is mounted in your OS and the VM at the same time. Otherwise, you run serious risks. No further warnings will be issued." caller=vm
time=2023-09-03T10:37:35.730+01:00 level=INFO msg="Booting the VM" caller=vm
time=2023-09-03T10:37:45.742+01:00 level=INFO msg="The VM is up, setting it up" caller=vm
time=2023-09-03T10:37:48.578+01:00 level=INFO msg="The VM is ready" caller=vm
NAME               SIZE FSTYPE
vda                  1G
├─vda1             300M ext4
├─vda2             256M swap
└─vda3             467M ext4
vdb               10.5T 
├─vdb1               2T crypto_LUKS
├─vdb2             1.5T ext4
├─vdb3             1.5T crypto_LUKS
└─vdb4             5.5T LVM2_member
  ├─vghdd-archive    3T crypto_LUKS
  └─vghdd-media    2.5T xfs
time=2023-09-03T10:37:49.075+01:00 level=WARN msg="Canceling the VM context" caller=vm
time=2023-09-03T10:37:49.105+01:00 level=WARN msg="Sending poweroff command to the VM" caller=vm
time=2023-09-03T10:37:49.117+01:00 level=INFO msg="Shutting the VM down safely" caller=vm
```

Filtering the logs out, this is the point of your interest:
```
NAME               SIZE FSTYPE
vda                  1G
├─vda1             300M ext4
├─vda2             256M swap
└─vda3             467M ext4
vdb               10.5T 
├─vdb1               2T crypto_LUKS
├─vdb2             1.5T ext4
├─vdb3             1.5T crypto_LUKS
└─vdb4             5.5T LVM2_member
  ├─vghdd-archive    3T crypto_LUKS
  └─vghdd-media    2.5T xfs
```

This is an output of `lsblk` command Linsk ran for you under the VM's hood.

You should ignore the `vda` drive as this is the system drive you have the Alpine Linux installation on. Assuming that you used raw device passthrough, commonly, `vdb` is going to be the drive you passed through. But please note that this may not always be the case, and you should inspect the output above and confirm that the partitions shown match your drive.

**Having an LVM volume group behind an encrypted LUKS container?** Extra configuration is required. Please see the [Use an LVM volume group contained inside a LUKS volume](#use-an-lvm-volume-group-contained-inside-a-luks-volume) section.

## Step 3. Run Linsk

Let's assume that we decided to run Linsk with the `vdb2` volume we found in the previous step. To do so, you may execute the following command:

```powershell
# This should be run in a terminal open with administrator privileges.
linsk run dev:\\.\PhysicalDriveX vdb2
```

Explanation of the command above:
- `dev:\\.\PhysicalDriveX` - Tell Linsk to pass through the drive path you obtained from step 1.
- `vdb2` - Tell Linsk to mount `/dev/vdb2` inside the filesystem. This was gathered from step 2.

**Pro Tip**: If the entire drive is just a single filesystem (without a partition table), you can omit the second parameter that specifies the VM device to mount. It will be automatically set to `vdb` as the default.

Upon running, you will see logs similar to this in your terminal:
```
# linsk command output
time=2023-09-03T10:53:57.385+01:00 level=WARN msg="Using raw block device passthrough. Please note that it's YOUR responsibility to ensure that no device is mounted in your OS and the VM at the same time. Otherwise, you run serious risks. No further warnings will be issued." caller=vm
time=2023-09-03T10:53:57.387+01:00 level=INFO msg="Booting the VM" caller=vm
time=2023-09-03T10:54:07.397+01:00 level=INFO msg="The VM is up, setting it up" caller=vm
time=2023-09-03T10:54:11.662+01:00 level=INFO msg="The VM is ready" caller=vm
time=2023-09-03T10:54:11.906+01:00 level=INFO msg="Mounting the device" dev=vdb2 fs=<auto> luks=false
time=2023-09-03T10:54:12.363+01:00 level=INFO msg="Started the network share successfully" backend=smb
===========================
[Network File Share Config]
The network file share was started. Please use the credentials below to connect to the file server.

Type: SMB
URL: \\fe8f-5980-3253-7df4-f4b-6db1-5d59-bc77.ipv6-literal.net\linsk
Username: linsk
Password: <random password>
===========================
```

At this point, you can open the file explorer -> Right-click "This PC" -> Show more options (if you're on Windows 11) -> Map network drive. Afterward, you should specify the share URL (the one that starts with `\\`), the static `linsk` username, and a randomly generated password.

**That's it!** After that, you should see the network share mounted successfully. That means that you can now access the files on the `vdb2` volume right from your Mac.

The network share will remain open until you close Linsk, which you can do at any time by hitting Ctrl+C.

# The advanced use of Linsk

The example provided above is just a mere preview of the endless power Linsk's native Linux VM has.

## Use LVM

Linsk supports LVM2. You can mount LVM2 drives by specifying `mapper/<device name>` as the VM device name. Let's assume that you want to mount `vghdd-media` you found in the `linsk ls` output above. To do so, you may run:
```powershell
# This should be run in a terminal open with administrator privileges.
linsk run dev:\\.\PhysicalDriveX mapper/vghdd-media
```

## Use LUKS with `cryptsetup`

As well as with LVM2, LUKS via `cryptsetup` is natively supported by Linsk. To mount LUKS volumes, you may specify the `-l` flag in `linsk run` command. Let's assume that we want to access LUKS-encrypted volume `vghdd-archive` we found in the `linsk ls` example provided in step 2. To mount it, you may execute:
```powershell
# This should be run in a terminal open with administrator privileges.
linsk run -l dev:\\.\PhysicalDriveX mapper/vghdd-archive
```

`-l` flag tells Linsk that it is a LUKS volume, and Linsk will prompt you for the password. Combined, your terminal will look like this:

```
# linsk command output
time=2023-09-03T11:44:55.962+01:00 level=WARN msg="Using raw block device passthrough. Please note that it's YOUR responsibility to ensure that no device is mounted in your OS and the VM at the same time. Otherwise, you run serious risks. No further warnings will be issued." caller=vm
time=2023-09-03T11:44:55.964+01:00 level=INFO msg="Booting the VM" caller=vm
time=2023-09-03T11:45:05.975+01:00 level=INFO msg="The VM is up, setting it up" caller=vm
time=2023-09-03T11:45:08.472+01:00 level=INFO msg="The VM is ready" caller=vm
time=2023-09-03T11:45:08.709+01:00 level=INFO msg="Mounting the device" dev=mapper/vghdd-archive fs=<auto> luks=true
time=2023-09-03T11:45:08.740+01:00 level=INFO msg="Attempting to open a LUKS device" caller=file-manager vm-path=/dev/mapper/vghdd-archive
Enter Password: <you will get prompted for the password here>
time=2023-09-03T11:46:08.444+01:00 level=INFO msg="LUKS device opened successfully" caller=file-manager vm-path=/dev/mapper/vghdd-archive
time=2023-09-03T11:46:08.642+01:00 level=INFO msg="Started the network share successfully" backend=smb
===========================
[Network File Share Config]
The network file share was started. Please use the credentials below to connect to the file server.

Type: SMB
URL: \\fe8f-5980-3253-7df4-f4b-6db1-5d59-bc77.ipv6-literal.net\linsk
Username: linsk
Password: <random password>
===========================
```

This example showed how you can use LUKS with LVM2 volumes, but that doesn't mean that you can't use volumes without LVM. You can specify plain device paths like `vdb3` without any issue.

## Use an LVM volume group contained inside a LUKS volume

This is a common scenario that is widely used to enable full-disk encryption on various Linux distributions. It implies having a master LUKS volume that, once decrypted, exposes an LVM volume group (vg).

Linsk supports this by exposing `--luks-container` flag. You can use it with both `linsk ls` and `linsk run`.

## `linsk ls` with LUKS Container

Let's assume that you have a similar file system structure:
```
NAME                  FSTYPE
vdb
├─vdb1                vfat
├─vdb2                ext4
└─vdb3                crypto_LUKS
  └─cryptroot         LVM2_member
    ├─vgubuntu-lvswap swap
    └─vgubuntu-lvroot ext4
```

If you run `linsk ls` without `--luks-container` flag like this:
```powershell
linsk ls dev:\\.\PhysicalDriveX
```

You are going to get this:
```
NAME                  FSTYPE
vdb
├─vdb1                vfat
├─vdb2                ext4
└─vdb3                crypto_LUKS
```

As you see, `vdb3` is locked, and the partitions inside thus remain invisible to the `linsk ls` command.

To tell Linsk to decrypt the `vdb3` container before going with anything, you may specify the `--luks-container` flag. Here is an example:
```powershell
linsk ls dev:\\.\PhysicalDriveX --luks-container vdb3
```

You will then get prompted a password, and once the LUKS container is open, you will see all of your partitions under the `vdb3` LUKS container. Your `lsblk` output will look like this:
```
NAME                  FSTYPE
vdb
├─vdb1                vfat
├─vdb2                ext4
└─vdb3                crypto_LUKS
  └─cryptcontainer    LVM2_member
    ├─vgubuntu-lvswap swap
    └─vgubuntu-lvroot ext4
```

## `linsk run` with LUKS Container

Let's assume that you want to mount `vgubuntu-lvroot`.

You need to use the same `--luks-container vdb3` flag we used with `linsk ls`. Combined, your command should look like the following:
```powershell
# This should be run in a terminal open with administrator privileges.
linsk run dev:\\.\PhysicalDriveX --luks-container vdb3 mapper/vgubuntu-lvroot
```

**Pro Tip**: If the entire passed-through volume is a LUKS container (i.e., you are attempting to run with `--luks-container vdb`), you may use the `-c` flag as a shortcut (or long `--luks-container-entire-drive`). It is equivalent to `--luks-container vdb`.

# FAQ

### How do I format disks with Linsk?

Use `linsk shell`. Please see [SHELL.md](SHELL.md).

# Troubleshooting

Please refer to [TROUBLESHOOTING.md](TROUBLESHOOTING.md).