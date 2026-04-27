class Clippy < Formula
  desc "macOS clipboard manager - Go backend + Swift menu bar + liquid glass UI"
  homepage "https://github.com/j1angyuxuan811-lab/clippy-v2"
  url "https://github.com/j1angyuxuan811-lab/clippy-v2/releases/download/v1.0.0/Clippy-1.0.0.zip"
  sha256 "db7715474ece3f23e640c33c20acdcb05e06b709dd1bef900433b67f79fab533"
  version "1.0.0"

  depends_on macos: :ventura

  def install
    prefix.install "Clippy.app"
    ln_s prefix/"Clippy.app", Applications/"Clippy.app"
  end

  def caveats
    <<~EOS
      Clippy has been installed to #{prefix}/Clippy.app

      To open:
        open #{prefix}/Clippy.app

      ⚠️  Grant Accessibility permission:
        System Settings → Privacy & Security → Accessibility → Enable Clippy

      Shortcuts:
        ⌘ + Shift + V  — Toggle clipboard panel
    EOS
  end

  test do
    assert_predicate prefix/"Clippy.app", :exist?
  end
end
