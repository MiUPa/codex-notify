class CodexNotify < Formula
  desc "macOS desktop notification bridge for Codex CLI"
  homepage "https://github.com/MiUPa/codex-notify"
  version "0.1.1"
  license "Apache-2.0"

  if Hardware::CPU.arm?
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.1.1/codex-notify_v0.1.1_darwin_arm64.tar.gz"
    sha256 "23dfba15d398ca4c39376353112fbb3416d7675641447fba469bbb7123fbce32"
  else
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.1.1/codex-notify_v0.1.1_darwin_amd64.tar.gz"
    sha256 "74b296f20dd98aa1918e1be888729b310197965b8f05a32d2baf03b25a8c908f"
  end

  def install
    bin.install "codex-notify"
  end

  test do
    assert_match "macOS desktop notifications for Codex CLI", shell_output("#{bin}/codex-notify help")
  end
end
