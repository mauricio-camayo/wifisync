# Building WifiSync

## Option A — Build on Linux/Mac using fyne-cross (recommended)

Produces a Windows `.exe` without needing a Windows machine.

**1. Install Go** from https://go.dev/dl/

**2. Install fyne-cross and Docker**
- Install Docker and make sure it is running
- Install fyne-cross:
  ```
  go install github.com/fyne-io/fyne-cross@latest
  ```

**3. Build:**
```
~/go/bin/fyne-cross windows -arch amd64
```

Output binary is placed in `fyne-cross/dist/windows-amd64/`.

---

## Option B — Build natively on Windows

**1. Install Go** from https://go.dev/dl/ (pick the Windows `.msi`). Open a new terminal after to pick up the PATH.

**2. Install a C compiler (required by Fyne)**
Fyne uses CGO/OpenGL, so you need gcc on PATH. The easiest way is **TDM-GCC**:
- Download from https://jmeubank.github.io/tdm-gcc/ → "tdm64-gcc-*.exe"
- Install with defaults — it adds itself to PATH automatically

Verify both are working:
```
go version
gcc --version
```

**3. Unzip the project**, open a terminal (`cmd` or PowerShell) in the project folder.

**4. Run the build:**
```
go build -ldflags="-H windowsgui -s -w" -o wifisync.exe .
```

Double-click `wifisync.exe` to run — no console window will appear.

## Notes

- `-s -w` strips debug symbols to reduce binary size
- If you get a `CGO_ENABLED` error on Windows, run `set CGO_ENABLED=1` before the build command
- With `make` installed (e.g. via MSYS2): `make windows` runs the fyne-cross command above
