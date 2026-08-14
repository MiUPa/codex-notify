class CodexNotify < Formula
  desc "macOS desktop notification bridge for Codex CLI"
  homepage "https://github.com/MiUPa/codex-notify"
  version "0.4.0"
  license "Apache-2.0"

  if Hardware::CPU.arm?
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.4.0/codex-notify_v0.4.0_darwin_arm64.tar.gz"
    sha256 "d72e00e2ee65c49c596c3e2e3fdaa54bf02d841a525d31d70fec95d86193c815"
  else
    url "https://github.com/MiUPa/codex-notify/releases/download/v0.4.0/codex-notify_v0.4.0_darwin_amd64.tar.gz"
    sha256 "ca4414d2b168c4ebbb0220ae41e6ef6cd8c1d73aa52ff99e0e6e56ed0ff2967c"
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
