class CodexNotify < Formula
  desc "macOS desktop notification bridge for Codex CLI"
  homepage "https://github.com/MiUPa/codex-notify"
  version "0.1.0"
  license "Apache-2.0"

  if Hardware::CPU.arm?
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.1.0/codex-notify_v0.1.0_darwin_arm64.tar.gz"
    sha256 "0f3989ce5335e731b0c785666dcadf80940e230c5b3f385a3b2b8a5f1ff32d37"
  else
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.1.0/codex-notify_v0.1.0_darwin_amd64.tar.gz"
    sha256 "6236a1018079354ae5c18c53e0a1a6f1e8c521d739842219f3aedfc255fc3ad1"
  end

  def install
    bin.install "codex-notify"
  end

  test do
    assert_match "macOS desktop notifications for Codex CLI", shell_output("#{bin}/codex-notify help")
  end
end
