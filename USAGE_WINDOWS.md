# Linsk Windows usage instructions

In this document, you will find instructions on how to get started using Linsk on Windows.

# How Linsk works
As you probably have realized from the initial README, Linsk takes use of a lightweight Alpine Linux virtual machine to tap into the rich world of Linux filesystems.

Linsk will pass through the disk as a raw block device to an ephemeral virtual machine, set up a file share and then expose it to your host computer, along with logging the file share connection details. It's as simple as that.

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

**NOTE:** Building a VM image requires internet connection. After the initial image build is done, you can use Linsk offline.

## Step 1. Select the drive you want to pass through

Find the path of the physical drive you want to pass through by executing the following command:
```powershell
wmic diskdrive list brief
```

Find your disk, and take a note of the disk path that looks like `\\.\PhysicalDriveX` (where X is a number). We will need this in the next step.

## Step 2. Use `linsk ls` to see what partitions are available in the VM

Run `linsk ls` while specifying the block device path you obtained in the previous step:
```powershell
sudo linsk ls dev:\\.\PhysicalDriveX
```

You will then see something like this:
```
# linsk command output
time=2023-09-03T10:37:35.728+01:00 level=WARN msg="Using raw block device passthrough. Please note that it's YOUR responsibility to ensure that no device is mounted in your OS and the VM at the same time. Otherwise, you run serious risks. No further warnings will be issued." caller=vm
time=2023-09-03T10:37:35.730+01:00 level=INFO msg="Booting the VM" caller=vm
time=2023-09-03T10:37:45.742+01:00 level=INFO msg="The VM is up, setting it up" caller=vm
time=2023-09-03T10:37:48.578+01:00 level=INFO msg="The VM is ready" caller=vm
NAME               SIZE FSTYPE      LABEL
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
NAME               SIZE FSTYPE      LABEL
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

You should ignore `vda` drive as this is the system drive you have the Alpine Linux installation on. Assuming that you used raw device passthrough, commonly, `vdb` is going to be the drive you passed through. But please note that this may not always be the case, and you should inspect the output above and confirm that the partitions shown match your drive.

## Step 3. Run Linsk

Let's assume that we decided to run Linsk with the `vdb2` `ext4` volume we found in the previous step. To do so, you may execute the following command:

```powershell
sudo linsk run dev:\\.\PhysicalDriveX vdb2 ext4
```

Explanation of the command above:
- `dev:\\.\PhysicalDriveX` - Tell Linsk to pass through the drive path you obtained from the step 1.
- `vdb2` - Tell Linsk to mount `/dev/vdb2` inside the filesystem. This was gathered from from the step 2.
- `ext4` - Tell Linsk to use the Ext4 file system. As with the `vdb2`, this was acquired from the step 2. **NOTE:** Specifying the file system is **REQUIRED**—you need to explicitly tell Linsk what filesystem you want to use.

Upon running, you will see logs similar to this in your terminal:
```
# linsk command output
time=2023-09-03T10:53:57.385+01:00 level=WARN msg="Using raw block device passthrough. Please note that it's YOUR responsibility to ensure that no device is mounted in your OS and the VM at the same time. Otherwise, you run serious risks. No further warnings will be issued." caller=vm
time=2023-09-03T10:53:57.387+01:00 level=INFO msg="Booting the VM" caller=vm
time=2023-09-03T10:54:07.397+01:00 level=INFO msg="The VM is up, setting it up" caller=vm
time=2023-09-03T10:54:11.662+01:00 level=INFO msg="The VM is ready" caller=vm
time=2023-09-03T10:54:11.906+01:00 level=INFO msg="Mounting the device" dev=vdb2 fs=ext4 luks=false
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
