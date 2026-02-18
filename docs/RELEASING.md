# Releasing

## 1. Prepare

- Confirm `main` is green (`ci` workflow).
- Ensure `README.md` and command help are up to date.

## 2. Tag and push

```bash
git tag v0.1.0
git push origin v0.1.0
```

This triggers `.github/workflows/release.yml` and creates a GitHub Release with:
- `codex-notify_<version>_darwin_amd64.tar.gz`
- `codex-notify_<version>_darwin_arm64.tar.gz`
- `checksums.txt`

## 3. Update Homebrew tap

Generate the Formula content:

```bash
./scripts/gen_formula.sh v0.1.0
```

Copy the output into `Formula/codex-notify.rb` in `MiUPa/homebrew-codex-notify`, then commit and push there.

## 4. Verify install

```bash
brew update
brew tap MiUPa/homebrew-codex-notify
brew install codex-notify
codex-notify doctor
```
