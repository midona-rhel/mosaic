# Signed portable sensor driver

Unmodified official PawnIO **2.1.0** images. GPL-2.0-or-later with the upstream
IOCTL linking exception. See `../licenses/PawnIO-GPL-2.0.txt`,
`../licenses/PawnIO-exception.txt` and `../sources/PawnIO-2.1.0-source.zip`.
Source commit: `e1043d7750f501b6b0d3ac0a71bd5b9406cd17b6`.
PawnPP submodule: `e64e4c37b2d8ba0d8ee57205faf8183aee12c438` (included).

Extracted without executing the official distribution:
https://github.com/namazso/PawnIO.Setup/releases/download/2.1.0/PawnIO_setup.exe

Distribution SHA-256:
`a3a46226c5e2824f4cdd42be0eecbabfc672c86f7889710f5ab1e6ad385b47a0`

Its PE resource RCDATA/105 is a Zstandard-compressed CAB. The CAB contains
repeated filenames in different folders. The official signed ARM64 image is
folder 3; the official signed x64 image is folder 9. Do not flatten all entries:
that would replace the signed ARM64 build with a different variant.

| Image | SHA-256 |
| --- | --- |
| amd64/PawnIO.sys | f92de04e5a02256e86ffa1b4252fe669282198b7329227f40e51bca9267d133d |
| arm64/PawnIO.sys | 2b663b621004c1646726aafffd0e2feffa468f6c6b8053ffb7f6995c436d797c |

Windows validates kernel signatures when loading. CI additionally checks
Authenticode signatures. The helper validates hashes before copying to a
protected directory. No installer executable, unsigned driver, certificate
installation, signature bypass or boot-start registration is distributed.

2.1 supports direct `DriverEntry` device creation and `DriverUnload`; 2.2 moved
to a PnP `AddDevice` path. Keep the version choice explicit when updating.
