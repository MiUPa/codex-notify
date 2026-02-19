class CodexNotify < Formula
  desc "macOS desktop notification bridge for Codex CLI"
  homepage "https://github.com/MiUPa/codex-notify"
  version "0.2.0"
  license "Apache-2.0"

  if Hardware::CPU.arm?
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.2.0/codex-notify_v0.2.0_darwin_arm64.tar.gz"
    sha256 "6ffa7df5936ee8816f48fea1dd73735f6abad99d00102145aaaaec641a354550"
  else
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.2.0/codex-notify_v0.2.0_darwin_amd64.tar.gz"
    sha256 "2306d6cc8a82debf9c41467a86df0642f50692e0ba064dec267d4effcef6ddb4"
  end

  def install
    bin.install "codex-notify"
  end

  test do
    assert_match "macOS desktop notifications for Codex CLI", shell_output("#{bin}/codex-notify help")
  end
end
