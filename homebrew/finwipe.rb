class Finwipe < Formula
  desc "DIY Financial Data Deletion tool for India under DPDP Act 2023"
  homepage "https://github.com/Das-rebel/finwipe"
  license "MIT"
  head "https://github.com/Das-rebel/finwipe.git", branch: "main"

  on_macos do
    on_intel do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.1.4/finwipe-darwin-amd64"
      sha256 "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
      version "0.1.4"
      def install
        bin.install "finwipe-darwin-amd64" => "finwipe"
      end
    end

    on_arm do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.1.4/finwipe-darwin-arm64"
      sha256 "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
      version "0.1.4"
      def install
        bin.install "finwipe-darwin-arm64" => "finwipe"
      end
    end
  end

  on_linux do
    url "https://github.com/Das-rebel/finwipe/releases/download/v0.1.4/finwipe-linux-amd64"
    sha256 "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    version "0.1.4"
    def install
      bin.install "finwipe-linux-amd64" => "finwipe"
    end
  end

  def post_install
    require "fileutils"
    dest = File.join(ENV["HOME"], ".finwipe", "nbfcs.yaml")
    return if File.exist?(dest)
    FileUtils.mkdir_p(File.dirname(dest))
    # Create placeholder - user should run: finwipe update-registry
    FileUtils.touch(dest)
  end

  test do
    assert_match "FinWipe", shell_output("#{bin}/finwipe --help")
  end
end
