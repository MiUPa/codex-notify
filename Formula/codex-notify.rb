class CodexNotify < Formula
  desc "macOS desktop notification bridge for Codex CLI"
  homepage "https://github.com/MiUPa/codex-notify"
  version "0.3.2"
  license "Apache-2.0"

  if Hardware::CPU.arm?
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.3.2/codex-notify_v0.3.2_darwin_arm64.tar.gz"
    sha256 "b6c6a8616cfcb4f554554568bab7450f71b587e8a5e36a8189b048fc60e1559e"
  else
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.3.2/codex-notify_v0.3.2_darwin_amd64.tar.gz"
    sha256 "751f7fa6d668a4a4afbc316b8ee0b5c9fc13616a7e8a3f7e2e4c6a83e88fb71b"
  end

  def install
    bin.install "codex-notify"
  end

  test do
    assert_match "macOS desktop notifications for Codex CLI", shell_output("#{bin}/codex-notify help")
  end
end
