class CodexNotify < Formula
  desc "macOS desktop notification bridge for Codex CLI"
  homepage "https://github.com/MiUPa/codex-notify"
  version "0.3.5"
  license "Apache-2.0"

  if Hardware::CPU.arm?
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.3.5/codex-notify_v0.3.5_darwin_arm64.tar.gz"
    sha256 "63b3e69554af6758e2c31ea122a0c6e8ea08d475e44fb85fe1a6c737ead7015c"
  else
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.3.5/codex-notify_v0.3.5_darwin_amd64.tar.gz"
    sha256 "7b0efac09dd6096c01fb1d01fdf79ae40bcf3ad36cbbaf0bb9ca3b68ee993aba"
  end

  def install
    bin.install "codex-notify"
  end

  def post_install
    system_command(bin/"codex-notify", args: ["init"])
  rescue ErrorDuringExecution
    opoo "Automatic Codex notify hook setup failed. Run `codex-notify init --replace` manually."
  end

  test do
    assert_match "macOS desktop notifications for Codex CLI", shell_output("#{bin}/codex-notify help")
  end
end
