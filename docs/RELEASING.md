# Releasing

## 1. Prepare

- Confirm `main` is green (`ci` workflow).
- Ensure `README.md` and command help are up to date.

## 2. Tag and push

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

This triggers `.github/workflows/release.yml` and creates a GitHub Release with:
- `codex-notify_<version>_darwin_amd64.tar.gz`
- `codex-notify_<version>_darwin_arm64.tar.gz`
- `checksums.txt`

## 3. Update Homebrew tap

`release.yml` automatically updates `MiUPa/homebrew-codex-notify` after the GitHub Release is published.
Prerequisite: repository secret `HOMEBREW_TAP_GITHUB_TOKEN` must be configured with push access to the tap repository.

Manual fallback (if automation fails):

```bash
./scripts/gen_formula.sh vX.Y.Z
```

Copy the output into `Formula/codex-notify.rb` in `MiUPa/homebrew-codex-notify`, then commit and push there.

## 4. Verify install

```bash
brew update
brew tap MiUPa/homebrew-codex-notify
brew install codex-notify
codex-notify doctor
```

`brew install codex-notify` runs `codex-notify init` automatically via Formula `post_install` for standard setup.
