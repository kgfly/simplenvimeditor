# Publishing to package repositories

This document explains how to publish SimpleNvimEditor to:

- **macOS:** Homebrew
- **Linux:** apt (Debian/Ubuntu), rpm (Fedora/RHEL/openSUSE), and Arch (AUR)
- **Windows:** winget

For each channel it lists the **one-time setup** and the **per-release
work**. It also shows how to automate each channel from
[`release.yml`](../.github/workflows/release.yml) so that pushing a
`vX.Y.Z` tag updates every repository.

## Summary: can it be automated?

Yes, all four channels can be updated by GitHub Actions on every release.
The limits come from the target repositories, not from GitHub:

| Channel | Where it is published | Fully automatic per release? | Manual work per release |
|---|---|---|---|
| Homebrew | Own tap `kgfly/homebrew-tap` (cask) | Yes | None |
| apt + rpm | Own signed repo on GitHub Pages `kgfly/simplenvim-packages` | Yes | None |
| Arch | AUR package `simplenvim-bin` | Yes | None |
| winget | `microsoft/winget-pkgs` | Yes, a PR is opened automatically | None. Microsoft's bot and moderators review and merge the PR. Act only if the PR gets a failure label. |

What is **not** possible right now:

- **Official `homebrew/cask`.** Homebrew requires casks to pass Gatekeeper
  ([Acceptable Casks](https://docs.brew.sh/Acceptable-Casks)), which means a
  paid Apple Developer ID signature plus notarization. Our `.dmg` is only
  ad-hoc signed, so we use our own tap. Once the app is notarized, the same
  cask can be submitted to `homebrew/cask`. It must also meet Homebrew's
  [notability rules](https://docs.brew.sh/Package-Acceptance-Policy#notability).
- **Official Debian/Ubuntu/Fedora/Arch repositories.** Those are built from
  source by distro packagers or sponsors under each distro's policy, such as
  vendored Go modules and offline builds. They can't be pushed from our CI.
  An own signed apt/rpm repo plus the AUR give users the same `apt install`
  / `dnf install` / `yay -S` experience.
- **The first winget version.** [winget-releaser](https://github.com/vedantmgoyal9/winget-releaser)
  needs one version already in `winget-pkgs`, so the first submission is
  manual.

## How it plugs into `release.yml`

### Run the publish jobs inside `release.yml`

The GitHub Release is created by `softprops/action-gh-release` using the
default `GITHUB_TOKEN`. GitHub does **not** start other workflows for events
caused by `GITHUB_TOKEN`, so a separate workflow with
`on: release: types: [published]` would never run.

Instead, add one job per channel to `release.yml`, each with
`needs: [meta, publish]`. They run after the release and its assets are
public. Each job is independent, so one channel failing doesn't block the
others, and a failed job can be retried alone with **Re-run failed jobs**.

Nightly builds (`nightly.yml`) must **not** publish to these repositories.
Only add the jobs to `release.yml`.

### Repository secrets

Add these under **Settings → Secrets and variables → Actions** of
`kgfly/SimpleNvimEditor`:

| Secret | Used by | What it is |
|---|---|---|
| `HOMEBREW_TAP_TOKEN` | Homebrew | Fine-grained PAT, repository access **only** `kgfly/homebrew-tap`, permission *Contents: Read and write* |
| `PKG_REPO_TOKEN` | apt/rpm | Fine-grained PAT, repository access **only** `kgfly/simplenvim-packages`, permission *Contents: Read and write* |
| `PKG_REPO_GPG_PRIVATE_KEY` | apt/rpm | ASCII-armored private key that signs the apt/rpm repo |
| `PKG_REPO_GPG_PASSPHRASE` | apt/rpm | Passphrase of that key |
| `AUR_SSH_PRIVATE_KEY` | AUR | Private SSH key registered on the AUR account |
| `WINGET_TOKEN` | winget | **Classic** PAT with scopes `public_repo` and `workflow` (winget-releaser doesn't support fine-grained PATs) |

Use the narrowest token scope that works, and set an expiration date on
every PAT. See [Recurring maintenance](#recurring-maintenance).

---

## 1. Homebrew (macOS)

Users will install with:

```sh
brew install --cask kgfly/tap/simplenvimeditor
```

Using the fully qualified name trusts only this cask. Since Homebrew 6.0,
taps that aren't official must be trusted explicitly
([Tap Trust](https://docs.brew.sh/Tap-Trust)).

### One-time setup

1. Create a **public** GitHub repository named exactly `kgfly/homebrew-tap`.
   The `homebrew-` prefix is what makes `kgfly/tap` resolve to it.
2. Add `Casks/simplenvimeditor.rb` (the CI job below generates it, but commit
   a first version by hand to test):

   ```ruby
   cask "simplenvimeditor" do
     arch arm: "arm64", intel: "amd64"

     version "1.2.3"
     sha256 arm:   "<sha256 of simplenvim_1.2.3_darwin_arm64.dmg>",
            intel: "<sha256 of simplenvim_1.2.3_darwin_amd64.dmg>"

     url "https://github.com/kgfly/SimpleNvimEditor/releases/download/v#{version}/simplenvim_#{version}_darwin_#{arch}.dmg"
     name "SimpleNvimEditor"
     desc "Simple, fast, native Neovim GUI"
     homepage "https://github.com/kgfly/SimpleNvimEditor"

     depends_on formula: "neovim"
     depends_on macos: ">= :big_sur"

     app "SimpleNvimEditor.app"

     caveats <<~EOS
       SimpleNvimEditor is not notarized. Before the first launch run:
         xattr -c "#{appdir}/SimpleNvimEditor.app"
     EOS
   end
   ```

   The checksums are in the release's `SHA256SUMS.txt`.
   `depends_on macos: ">= :big_sur"` matches `LSMinimumSystemVersion 11.0`
   in [make-dmg.sh](../packaging/macos/make-dmg.sh). `depends_on formula:
   "neovim"` installs Homebrew's Neovim too, even if the user already has
   another one. Remove that line if you don't want this.
3. Test it locally:

   ```sh
   brew install --cask kgfly/tap/simplenvimeditor
   brew audit --cask --strict kgfly/tap/simplenvimeditor
   brew uninstall --cask simplenvimeditor
   ```

4. Create the `HOMEBREW_TAP_TOKEN` secret (see the table above).
5. Add the job below to `release.yml`.
6. Add the `brew install` line to the README's *Installing* section.

### Automation: job for `release.yml`

```yaml
  homebrew:
    name: Update Homebrew tap
    needs: [meta, publish]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
        with:
          repository: kgfly/homebrew-tap
          token: ${{ secrets.HOMEBREW_TAP_TOKEN }}
          path: tap

      - name: Download checksums
        env:
          GH_TOKEN: ${{ github.token }}
        run: gh release download "${{ needs.meta.outputs.tag }}" -R "${{ github.repository }}" -p SHA256SUMS.txt

      - name: Write cask
        env:
          VERSION: ${{ needs.meta.outputs.version }}
        run: |
          set -euo pipefail
          arm=$(awk '/_darwin_arm64\.dmg$/ {print $1}' SHA256SUMS.txt)
          intel=$(awk '/_darwin_amd64\.dmg$/ {print $1}' SHA256SUMS.txt)
          [ -n "$arm" ] && [ -n "$intel" ]
          mkdir -p tap/Casks
          cat > tap/Casks/simplenvimeditor.rb <<EOF
          cask "simplenvimeditor" do
            arch arm: "arm64", intel: "amd64"

            version "${VERSION}"
            sha256 arm:   "${arm}",
                   intel: "${intel}"

            url "https://github.com/kgfly/SimpleNvimEditor/releases/download/v#{version}/simplenvim_#{version}_darwin_#{arch}.dmg"
            name "SimpleNvimEditor"
            desc "Simple, fast, native Neovim GUI"
            homepage "https://github.com/kgfly/SimpleNvimEditor"

            depends_on formula: "neovim"
            depends_on macos: ">= :big_sur"

            app "SimpleNvimEditor.app"

            caveats <<~EOS
              SimpleNvimEditor is not notarized. Before the first launch run:
                xattr -c "#{appdir}/SimpleNvimEditor.app"
            EOS
          end
          EOF

      - name: Commit and push
        working-directory: tap
        run: |
          set -euo pipefail
          git config user.name "github-actions[bot]"
          git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
          git add Casks/simplenvimeditor.rb
          git commit -m "simplenvimeditor ${{ needs.meta.outputs.version }}"
          git push
```

### Per release

Nothing. Users get the new version with `brew upgrade --cask`.

### Later: official `homebrew/cask`

After the app is signed with a Developer ID and notarized (see the
signing phase in [developer.md](developer.md#releasing-official-build)),
open a PR to [Homebrew/homebrew-cask](https://github.com/Homebrew/homebrew-cask)
with the same cask, minus the `caveats`. After that, Homebrew's autobump bot
updates it on new GitHub releases, and the tap job can be removed.

---

## 2. apt and rpm (Debian/Ubuntu, Fedora/RHEL/openSUSE)

The release already builds `simplenvim_<v>_linux_{amd64,arm64}.deb` and
`.rpm` with nfpm ([package.yml](../.github/workflows/package.yml)). This
section publishes them in a **GPG-signed apt + rpm repository** served for
free by GitHub Pages from a separate repository, `kgfly/simplenvim-packages`.

Users will install with:

```sh
# Debian / Ubuntu
sudo install -d -m 0755 /etc/apt/keyrings
curl -fsSL https://kgfly.github.io/simplenvim-packages/simplenvim.gpg \
  | sudo tee /etc/apt/keyrings/simplenvim.gpg >/dev/null
echo "deb [signed-by=/etc/apt/keyrings/simplenvim.gpg] https://kgfly.github.io/simplenvim-packages/apt stable main" \
  | sudo tee /etc/apt/sources.list.d/simplenvim.list
sudo apt update && sudo apt install simplenvim
```

```sh
# Fedora / RHEL (dnf); openSUSE uses the same file under /etc/zypp/repos.d
sudo tee /etc/yum.repos.d/simplenvim.repo <<'EOF'
[simplenvim]
name=SimpleNvimEditor
baseurl=https://kgfly.github.io/simplenvim-packages/rpm
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://kgfly.github.io/simplenvim-packages/simplenvim.asc
EOF
sudo dnf install simplenvim
```

After that, `apt upgrade` / `dnf upgrade` pick up each new release.

### One-time setup

1. **Create the signing key** on a trusted machine:

   ```sh
   gpg --quick-gen-key "SimpleNvimEditor packages <you@example.com>" rsa4096 sign 5y
   gpg --list-secret-keys --keyid-format long     # copy the 40-char fingerprint
   FPR=<fingerprint>
   gpg --armor --export "$FPR" > simplenvim.asc   # public key, for dnf
   gpg --export "$FPR" > simplenvim.gpg           # public key, binary, for apt
   gpg --armor --export-secret-keys "$FPR" > private.asc
   ```

   Put `private.asc` into the `PKG_REPO_GPG_PRIVATE_KEY` secret and the
   passphrase into `PKG_REPO_GPG_PASSPHRASE`. Back up the private key
   offline, then delete `private.asc`. If the key is lost, every user has to
   re-import a new one.

2. **Create the repository** `kgfly/simplenvim-packages` (public) with this
   layout:

   ```
   .nojekyll                 # empty; serve files as-is, no Jekyll
   simplenvim.asc            # from step 1
   simplenvim.gpg            # from step 1
   apt/conf/distributions    # reprepro config, below
   rpm/                      # empty for now (add a .gitkeep)
   ```

   `apt/conf/distributions`:

   ```
   Origin: SimpleNvimEditor
   Label: SimpleNvimEditor
   Codename: stable
   Architectures: amd64 arm64
   Components: main
   Description: SimpleNvimEditor packages
   SignWith: <fingerprint from step 1>
   ```

3. **Enable GitHub Pages** on that repository: **Settings → Pages → Deploy
   from a branch → `main` / `(root)`**. Check that
   `https://kgfly.github.io/simplenvim-packages/simplenvim.asc` loads.

4. Create the `PKG_REPO_TOKEN` secret (see the table above).

5. Add the job below to `release.yml`, run a release, then test the
   install commands above in a Debian/Ubuntu and a Fedora container:

   ```sh
   docker run --rm -it ubuntu:24.04 bash    # then: apt-get update && apt-get install -y curl, run the apt steps
   docker run --rm -it fedora:latest bash   # run the dnf steps
   ```

6. Add the install commands to the README.

### Automation: job for `release.yml`

```yaml
  linux-repo:
    name: Update apt/rpm repo
    needs: [meta, publish]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
        with:
          repository: kgfly/simplenvim-packages
          token: ${{ secrets.PKG_REPO_TOKEN }}

      - name: Install tools
        run: |
          sudo apt-get update
          sudo apt-get install -y reprepro rpm createrepo-c

      - name: Import signing key
        id: gpg
        uses: crazy-max/ghaction-import-gpg@v6
        with:
          gpg_private_key: ${{ secrets.PKG_REPO_GPG_PRIVATE_KEY }}
          passphrase: ${{ secrets.PKG_REPO_GPG_PASSPHRASE }}

      - name: Download packages
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          gh release download "${{ needs.meta.outputs.tag }}" -R "${{ github.repository }}" \
            -p '*.deb' -p '*.rpm' -D incoming

      - name: Update apt repo
        run: reprepro -b apt includedeb stable incoming/*.deb

      - name: Update rpm repo
        env:
          FPR: ${{ steps.gpg.outputs.fingerprint }}
        run: |
          set -euo pipefail
          rm -f rpm/*.rpm
          cp incoming/*.rpm rpm/
          rpmsign --addsign --define "_gpg_name $FPR" rpm/*.rpm
          createrepo_c rpm
          gpg --batch --yes --local-user "$FPR" --detach-sign --armor rpm/repodata/repomd.xml

      - name: Commit and push
        run: |
          set -euo pipefail
          git config user.name "github-actions[bot]"
          git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
          git add -A apt rpm
          git commit -m "simplenvim ${{ needs.meta.outputs.version }}"
          git push
```

Notes:

- `reprepro` keeps one version of a package per distribution and deletes
  the old `.deb` from `pool/`. The rpm step also keeps only the latest
  `.rpm`. This keeps the Pages site well under GitHub Pages' 1 GB limit.
- The `.rpm` files in this repo carry an extra GPG signature, so their
  checksums differ from the release assets. That is expected.
- `apt/db/` (reprepro's state) is committed with the rest. It holds no
  secrets.

### Per release

Nothing.

### Alternatives

- **Hosted service** instead of GitHub Pages: [Cloudsmith](https://cloudsmith.com)
  (free for open source) or [packagecloud](https://packagecloud.io) host
  signed apt and rpm repos and have upload CLIs/actions. They replace steps
  1 to 3 and the reprepro/createrepo commands with one upload call, but add
  an external account.
- **Ubuntu PPA (Launchpad) / Fedora COPR**: these build from a source
  package on their own servers. They are more work because the Go module
  cache must be vendored for offline builds (PPA). They're only worth it if
  you want the package to be built by the distro's own infrastructure.

---

## 3. Arch Linux (AUR)

The AUR hosts build recipes (`PKGBUILD`), not binaries. A `-bin` package
downloads our prebuilt release asset. It reuses the `.deb`, which already
contains the binary, the `.desktop` files and all the colored icons.

The `.pkg.tar.zst` in the release is a plain tarball without pacman's
`.MTREE`/`.BUILDINFO` metadata, so it isn't used here.

Users will install with an AUR helper:

```sh
yay -S simplenvim-bin      # or: paru -S simplenvim-bin
```

or by hand: `git clone https://aur.archlinux.org/simplenvim-bin.git && cd simplenvim-bin && makepkg -si`.

### One-time setup

1. Create an account at <https://aur.archlinux.org/register>.
2. Create an SSH key just for the AUR and register it under
   **My Account → SSH Public Key**:

   ```sh
   ssh-keygen -t ed25519 -f ~/.ssh/aur -C "aur simplenvim"
   cat ~/.ssh/aur.pub        # paste into the AUR account page
   ```

   Put the content of `~/.ssh/aur` (the private key) into the
   `AUR_SSH_PRIVATE_KEY` secret.
3. Add the template `packaging/arch/PKGBUILD` to this repo. `@VERSION@` is
   replaced by CI:

   ```bash
   # Maintainer: kgfly <you@example.com>
   pkgname=simplenvim-bin
   pkgver=@VERSION@
   pkgrel=1
   pkgdesc="A simple, fast, native Neovim GUI written in Go using Gio"
   arch=('x86_64' 'aarch64')
   url="https://github.com/kgfly/SimpleNvimEditor"
   license=('MIT')
   depends=('neovim')
   provides=('simplenvim')
   conflicts=('simplenvim')
   source_x86_64=("${url}/releases/download/v${pkgver}/simplenvim_${pkgver}_linux_amd64.deb")
   source_aarch64=("${url}/releases/download/v${pkgver}/simplenvim_${pkgver}_linux_arm64.deb")
   sha256sums_x86_64=('SKIP')
   sha256sums_aarch64=('SKIP')

   package() {
     bsdtar -xf data.tar.* -C "$pkgdir"
     install -Dm644 "$pkgdir/usr/share/doc/simplenvim/LICENSE" \
       "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
   }
   ```

   `makepkg` unpacks the `.deb` into `data.tar.*`, which holds the files
   to install. CI's `updpkgsums` replaces `SKIP` with the real checksums.
4. On an Arch machine or in an `archlinux` container (as a normal user,
   because `makepkg` refuses to run as root), build and check it once:

   ```sh
   sed 's/@VERSION@/1.2.3/' packaging/arch/PKGBUILD > PKGBUILD
   updpkgsums
   makepkg -si
   namcap PKGBUILD *.pkg.tar.zst
   ```

   `namcap` lists the shared libraries the binary links against, such as
   X11, Wayland and xkbcommon. Add those Arch packages to `depends=()` so
   the package also works on minimal installs.
5. Publish the first version. Cloning a name that doesn't exist yet gives an
   empty repo, and the first push creates the package:

   ```sh
   GIT_SSH_COMMAND="ssh -i ~/.ssh/aur" git clone ssh://aur@aur.archlinux.org/simplenvim-bin.git
   cd simplenvim-bin
   cp ../PKGBUILD .
   makepkg --printsrcinfo > .SRCINFO
   git add PKGBUILD .SRCINFO
   git commit -m "Initial import: simplenvim-bin 1.2.3"
   GIT_SSH_COMMAND="ssh -i ~/.ssh/aur" git push
   ```

6. Add the job below to `release.yml`.

### Automation: job for `release.yml`

```yaml
  aur:
    name: Update AUR
    needs: [meta, publish]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5

      - name: Render PKGBUILD
        run: sed "s/@VERSION@/${{ needs.meta.outputs.version }}/" packaging/arch/PKGBUILD > PKGBUILD

      - uses: KSXGitHub/github-actions-deploy-aur@v4.2.0
        with:
          pkgname: simplenvim-bin
          pkgbuild: ./PKGBUILD
          updpkgsums: true
          commit_username: kgfly
          commit_email: you@example.com
          ssh_private_key: ${{ secrets.AUR_SSH_PRIVATE_KEY }}
          commit_message: "Update to ${{ needs.meta.outputs.version }}"
          ssh_keyscan_types: rsa,ecdsa,ed25519
```

The action runs in an Arch container, computes the checksums, regenerates
`.SRCINFO` and pushes to the AUR.

### Per release

Nothing. Do reply to comments on the AUR package page; users report
packaging problems there, not on GitHub.

### Optional: a source package

A second AUR package `simplenvim` that builds from the source tarball (with
`go build`) is welcome on the AUR. It isn't needed for users to install the
editor, and it would need the build dependencies listed in
[developer.md](developer.md).

---

## 4. winget (Windows)

The release already builds `simplenvim_<v>_windows_amd64-setup.exe` with
Inno Setup ([simplenvim.iss](../packaging/windows/simplenvim.iss)). winget
knows Inno Setup's silent switches, so no extra installer work is needed.

Users will install with:

```powershell
winget install kgfly.SimpleNvimEditor
```

### One-time setup

1. **Fork** [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs)
   under the `kgfly` account, the same owner as this repository.
   winget-releaser opens its PRs from that fork.
2. **Submit the first version by hand** on a Windows machine using
   Microsoft's [wingetcreate](https://github.com/microsoft/winget-create):

   ```powershell
   winget install Microsoft.WingetCreate
   wingetcreate new "https://github.com/kgfly/SimpleNvimEditor/releases/download/v1.2.3/simplenvim_1.2.3_windows_amd64-setup.exe"
   ```

   It downloads the installer, detects `InstallerType: inno`, and prompts
   for the rest. Use these values:

   | Field | Value |
   |---|---|
   | PackageIdentifier | `kgfly.SimpleNvimEditor` |
   | Publisher | `kgfly` |
   | PackageName | `SimpleNvimEditor` |
   | License | `MIT` |
   | ShortDescription | `A simple, fast, native Neovim GUI.` |
   | Moniker | `simplenvim` |
   | Commands | `simplenvim` |

   Add Neovim as a dependency in the installer manifest so winget installs
   it too:

   ```yaml
   Dependencies:
     PackageDependencies:
       - PackageIdentifier: Neovim.Neovim
   ```

   Before submitting, test the manifest locally from an admin shell:

   ```powershell
   winget settings --enable LocalManifestFiles
   winget validate --manifest <manifest-folder>
   winget install --manifest <manifest-folder>
   ```

   Then let wingetcreate submit the PR, or run
   `wingetcreate submit <manifest-folder>`. It asks for a GitHub login.
3. **Wait for the PR to merge.** Microsoft's pipeline installs the package
   in a VM and scans it, then a moderator merges it. If the PR gets a
   `Needs-Author-Feedback` or a validation-failure label, read the bot
   comment, fix the manifest, and push to the same PR branch.
4. Create a **classic** PAT with scopes `public_repo` and `workflow` and save
   it as the `WINGET_TOKEN` secret.
5. Add the job below to `release.yml`.

### Automation: job for `release.yml`

```yaml
  winget:
    name: Submit to winget
    needs: [meta, publish]
    runs-on: ubuntu-latest
    steps:
      - uses: vedantmgoyal9/winget-releaser@main
        with:
          identifier: kgfly.SimpleNvimEditor
          release-tag: ${{ needs.meta.outputs.tag }}
          installers-regex: '_windows_amd64-setup\.exe$'
          max-versions-to-keep: 5
          token: ${{ secrets.WINGET_TOKEN }}
```

Setting `release-tag` explicitly is required. The action's default uses
`github.ref_name`, which is the branch name, not the tag, when `release.yml`
is started from **Run workflow**. The action copies the previous manifest,
updates the version, URL and checksum, and opens the PR from the fork.
Pin `@main` to a commit SHA and let Dependabot update it, as the action's
README recommends.

### Per release

None for the normal case. The PR that winget-releaser opens is reviewed and
merged by Microsoft's bot and moderators. Look at the PR link in the job log
only if it doesn't merge, such as after a validation failure, and fix it the
same way as in step 3.

---

## Per-release checklist

After the one-time setups, a release is:

1. Push the tag (`git tag vX.Y.Z && git push origin vX.Y.Z`) or start
   **Actions → Release → Run workflow**, as described in
   [developer.md](developer.md#releasing-official-build).
2. Check that the `homebrew`, `linux-repo`, `aur` and `winget` jobs are green.
   Re-run any failed job alone.
3. winget only: the job log has the PR URL. It merges on its own unless the
   bot labels it with a failure.

## Recurring maintenance

- **Renew PATs** (`HOMEBREW_TAP_TOKEN`, `PKG_REPO_TOKEN`, `WINGET_TOKEN`)
  before they expire. GitHub emails a reminder. An expired token makes
  only that channel's job fail.
- **GPG key** (apt/rpm): it expires after the period chosen in
  `--quick-gen-key` (5 years above). Before then, extend it
  (`gpg --quick-set-expire <FPR> 5y`), re-export `simplenvim.asc` /
  `simplenvim.gpg` to the packages repo, and update the secret. Users fetch
  the refreshed public key with the same `curl` command.
- **AUR SSH key**: no expiry. Rotate it if it leaks.
- **winget fork**: winget-releaser syncs it, which is why the token needs the
  `workflow` scope. If a run fails with `failed to sync upstream changes`,
  click **Sync fork** on GitHub and re-run the job.
