# lsd

A mistyped `ldd` on psychedelics.

## Install

`go get github.com/nseps/lsd`

## What is

lsd is tool that helps you resolve dynamic linking dependencies for binaries and package them in a flat directory

## Why?

* Embedded or minimal system
* Static build not an option
* Convenience

In the future it might strip down libraries and reassemble everything in runable package just to have a single "static" binary

This tool parses the ELF data of the binary for information and uses it's own library lookup path. The output can differ between `lsd <target>` and `lsd <target> --trace`

## How to use

`lsd /bin/bash`

```
Path lookup order:
/bin
/lib64
/lib
/usr/lib
/usr/lib/x86_64-linux-gnu/libfakeroot
/usr/local/lib
/lib/x86_64-linux-gnu
/usr/lib/x86_64-linux-gnu
/usr/lib/x86_64-linux-gnu/mesa-egl
/usr/lib/x86_64-linux-gnu/mesa

Target: /bin/bash, Class: ELFCLASS64
  bash => /bin/bash
  libtinfo.so.5 => /lib/x86_64-linux-gnu/libtinfo.so.5
  libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6
  ld-linux-x86-64.so.2 => /lib64/ld-linux-x86-64.so.2
  libdl.so.2 => /lib/x86_64-linux-gnu/libdl.so.2
```

Or simulate ldd by adding the `--trace` flag

`lsd /bin/bash --trace`

```
Target: /bin/bash, Class: ELFCLASS64
        linux-vdso.so.1 =>  (0x00007ffd713bd000)
        libtinfo.so.5 => /lib/x86_64-linux-gnu/libtinfo.so.5 (0x00007f9c3563a000)
        libdl.so.2 => /lib/x86_64-linux-gnu/libdl.so.2 (0x00007f9c35436000)
        libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007f9c35056000)
        /lib64/ld-linux-x86-64.so.2 (0x00007f9c35863000)
```

You can have your dependecies in a nice tree. Have fun with huge binaries.

`lsd /bin/bash --tree`

```
Path lookup order:
/bin
/lib64
/lib
/usr/lib
/usr/lib/x86_64-linux-gnu/libfakeroot
/usr/local/lib
/lib/x86_64-linux-gnu
/usr/lib/x86_64-linux-gnu
/usr/lib/x86_64-linux-gnu/mesa-egl
/usr/lib/x86_64-linux-gnu/mesa

-bash
  |-libtinfo.so.5
  |  |-libc.so.6
  |  |  |-ld-linux-x86-64.so.2
  |-libdl.so.2
  |  |-libc.so.6
  |  |  |-ld-linux-x86-64.so.2
  |  |-ld-linux-x86-64.so.2
  |-libc.so.6
  |  |-ld-linux-x86-64.so.2
```

Export to directory

`lsd /bin/bash --export=out`

```
Path lookup order:
/bin
/lib64
/lib
/usr/lib
/usr/lib/x86_64-linux-gnu/libfakeroot
/usr/local/lib
/lib/x86_64-linux-gnu
/usr/lib/x86_64-linux-gnu
/usr/lib/x86_64-linux-gnu/mesa-egl
/usr/lib/x86_64-linux-gnu/mesa

Copy bash: /bin/bash => out
Copy libtinfo.so.5: /lib/x86_64-linux-gnu/libtinfo.so.5 => out
Copy libc.so.6: /lib/x86_64-linux-gnu/libc.so.6 => out
Copy ld-linux-x86-64.so.2: /lib64/ld-linux-x86-64.so.2 => out
Copy libdl.so.2: /lib/x86_64-linux-gnu/libdl.so.2 => out
```