class Finwipe < Formula
  desc "DIY Financial Data Deletion tool for India under DPDP Act 2023"
  homepage "https://github.com/Das-rebel/finwipe"
  license "MIT"
  version "0.1.4"

  on_macos do
    on_intel do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.1.4/finwipe-darwin-amd64"
      sha256 "91bd820ce525e1c4f8810f4fd3112a25cfa620b938caee228fe5d7e0f8e661ad"
      def install
        bin.install "finwipe-darwin-amd64" => "finwipe"
      end
    end

    on_arm do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.1.4/finwipe-darwin-arm64"
      sha256 "991ca0703f5be8b100db57d1bb92927fdd65c01ae959912ea03cb94111f4684e"
      def install
        bin.install "finwipe-darwin-arm64" => "finwipe"
      end
    end
  end

  on_linux do
    url "https://github.com/Das-rebel/finwipe/releases/download/v0.1.4/finwipe-linux-amd64"
    sha256 "4a8e75a4ceadfef617d60f2b7498d005a4783a972079003a4b04c005d826cbc5"
    def install
      bin.install "finwipe-linux-amd64" => "finwipe"
    end
  end

  def post_install
    require "fileutils"
    dest_dir = File.join(ENV["HOME"], ".finwipe")
    FileUtils.mkdir_p(dest_dir)
    puts "FinWipe installed! Run: finwipe init"
  end

  test do
    assert_match "FinWipe", shell_output("#{bin}/finwipe --help")
  end
end
