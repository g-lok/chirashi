# Third-Party Notices

## REX SDK

The REX SDK v1.9.2 is Copyright © Reason Studios AB and is used under the terms
of the Reason Studios General License Agreement (see `REX_SDK_LICENSE.txt`).

**The REX SDK is NOT open source and is NOT covered by this project's MIT license.**
It is proprietary Reason Studios software. You must accept the Reason Studios
license terms to build or run this software.

The following files from the REX SDK are included in this repository and are
governed solely by the Reason Studios license:

- `internal/rexengine/REX.h` — C API header (patched for MinGW compatibility)
- `internal/rexengine/rex/REX.c` — Windows dynamic DLL loader
- `internal/rexengine/libs/macos/REX Shared Library.framework/` — macOS framework binary

Release archives also bundle the framework binary (`Frameworks/REX Shared Library.framework/`)
for end-user convenience. This binary remains proprietary Reason Studios property
and is not subject to the MIT license.

## rexconverter

Copyright (c) 2026 G — All rights reserved.

All rexconverter source code, excluding the REX SDK components listed above,
is licensed under the MIT License — see `LICENSE` for details.
