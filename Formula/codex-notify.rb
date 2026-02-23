class CodexNotify < Formula
  desc "macOS desktop notification bridge for Codex CLI"
  homepage "https://github.com/MiUPa/codex-notify"
  version "0.3.0"
  license "Apache-2.0"

  if Hardware::CPU.arm?
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.3.0/codex-notify_v0.3.0_darwin_arm64.tar.gz"
    sha256 "5cff78577c4675a9a390753bc688ea4751da6c3a5c0415d5eb4664184073f7ba"
  else
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.3.0/codex-notify_v0.3.0_darwin_amd64.tar.gz"
    sha256 "2384a60efe5e4c93d94287220393b487542562addd82ba5872418cdda03c9a3a"
  end

  def install
    bin.install "codex-notify"
  end

  test do
    assert_match "macOS desktop notifications for Codex CLI", shell_output("#{bin}/codex-notify help")
  end
end
