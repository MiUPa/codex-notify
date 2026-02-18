class CodexNotify < Formula
  desc "macOS desktop notification bridge for Codex CLI"
  homepage "https://github.com/MiUPa/codex-notify"
  url "https://github.com/MiUPa/codex-notify/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "76dc730d6fced67c27e778e2a82d797d76704eb277a6a31a0bd97927f20fa2d3"
  license "Apache-2.0"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "."
  end

  test do
    assert_match "macOS desktop notifications for Codex CLI", shell_output("#{bin}/codex-notify help")
  end
end
