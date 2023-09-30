# Serious bug: Faulty raw device block size detection

## Key Aspects

* **Affected versions**: below v0.2.0.
* **Affected *UNLESS***:
  * The use of Linsk was limited exclusively to devices with a 512-byte logical block (sector) size; **or**
  * The experimental USB passthrough was used.
* **Possible outcomes**: A disk may be locked to Linsk after writing a considerable amount of data through Linsk. This means that the disk will not be accessible by any Linux machine, unless a 512-byte block size is emulated. Reformat is required to recover from this condition.
* **Severity**: Although there is **no data loss**, the severity can be considered as **High**.

## Brief description

Prior to version v0.2.0, Linsk assumed that the block size detection was done by QEMU. However, that wasn't the case. QEMU used a default block size of 512 bytes, no matter the physical/logical block (sector) size of the actual drive.

Originally, this was not an issue as the emulated block size of 512 bytes did not hinder mounting, reading, and writing the files to the disk. However, if a significant amount of writes were committed to the drive, the OS may attempt to optimize the file system(s). This is a problem since all possible optimizations will assume the emulated 512-byte block size to be the real logical block size. If this happens, the drive will get effectively locked to the emulated block size of 512 bytes. In that case, the host OS that recognizes a valid block size of over 512 bytes will not be able to access the files and/or recognize any of the filesystems in the disk.

The solution to recover from this condition is to copy the files to another (temporary) drive through Linsk, reformat the original, broken drive, and copy the files back to the place.

## Compatibility

Linsk v0.2.0 is packaged with a custom `dev_faulty_bs` device type that preserves the original behavior that implied the use of the faulty block size detection. This should not be used for anything else but to recover the files from a drive that was locked to the emulated 512-byte block size.

Example use:
```sh
sudo linsk run dev_faulty_bs:/dev/diskX vdb3
```

Notice the change from `dev` to `dev_faulty_bs`.