class Finwipe < Formula
  desc "DIY Financial Data Deletion tool for India under DPDP Act 2023"
  homepage "https://github.com/Das-rebel/finwipe"
  license "MIT"
  version "0.2.2"

  on_macos do
    on_intel do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.2.2/finwipe-darwin-amd64"
      sha256 "7f6ef903dbe1a40388fefddb6d99d9705bafc69d3d273d62aada2dfdd9425afa"
    end
    on_arm do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.2.2/finwipe-darwin-arm64"
      sha256 "8a42772de09badbfcfe9c5762ee86eaa73a82556ce6dc8e6a5a7dc3f35d2cd25"
    end
  end

  on_linux do
    on_x86_64 do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.2.2/finwipe-linux-amd64"
      sha256 "2ee8661a1247d9dd0c8ba051479c31e678d9631c71d907c9e0307e77aa80bdc6"
    end
    on_arm64 do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.2.2/finwipe-linux-arm64"
      sha256 "fa77cb90b019895e204f14e3f3570e461d1df68c38ea2ad1b41e30eadbc3e6a8"
    end
  end

  def install
    bin.install "finwipe-darwin-amd64" => "finwipe" if OS.mac? && Hardware::CPU.intel?
    bin.install "finwipe-darwin-arm64" => "finwipe" if OS.mac? && Hardware::CPU.arm?
    bin.install "finwipe-linux-amd64" => "finwipe" if OS.linux? && Hardware::CPU.intel?
    bin.install "finwipe-linux-arm64" => "finwipe" if OS.linux? && Hardware::CPU.arm?
  end
end
