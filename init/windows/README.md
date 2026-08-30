Windows packaging stamps PE VERSIONINFO at `wails3 generate syso` time
(`init/windows/stamp-version.sh`), not in GoReleaser. Release CI builds the
exe with Wails first; GoReleaser only publishes the prebuilt binary.

Explorer File version is the `x.y.z` prefix of the release tag plus
`REVISION` (`git rev-list --count --all`). Product version is the release
string (tag without `v`). Icon and the DPI-aware manifest stay in the same syso.

Authenticode (`signexe.sh`, YubiKey via golift/codesign) signs the **payload
exe before NSIS packs it**, then signs the installer wrapper. Signing only the
installer leaves the installed `captain-updater.exe` unsigned; Smart App
Control then reports that it cannot verify the publisher. `CODESIGN_URL` unset
skips signing so local snapshots still build.
