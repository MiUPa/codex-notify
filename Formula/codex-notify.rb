class CodexNotify < Formula
  desc "macOS desktop notification bridge for Codex CLI"
  homepage "https://github.com/MiUPa/codex-notify"
  version "0.2.1"
  license "Apache-2.0"

  if Hardware::CPU.arm?
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.2.1/codex-notify_v0.2.1_darwin_arm64.tar.gz"
    sha256 "e711c64d30416ee3807d6b28449ac188c7e227ce56e8e62243821c7d0b6945d7"
  else
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.2.1/codex-notify_v0.2.1_darwin_amd64.tar.gz"
    sha256 "a0409a2a5283b4d8cb74627a81827e79a4461817ba172238eda4cedd7e3edffa"
  end

  def install
    bin.install "codex-notify"
  end

  test do
    assert_match "macOS desktop notifications for Codex CLI", shell_output("#{bin}/codex-notify help")
  end
end
