class Finwipe < Formula
  desc "DIY Financial Data Deletion tool for India under DPDP Act 2023"
  homepage "https://github.com/Das-rebel/finwipe"
  license "MIT"
  version "0.2.3"

  on_macos do
    on_intel do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.2.3/finwipe-darwin-amd64"
      sha256 "9109f767f7908b6547c9c3c699361689c539cc0403ba28ebebc3893cdef113c3"
    end
    on_arm do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.2.3/finwipe-darwin-arm64"
      sha256 "8a42772de09badbfcfe9c5762ee86eaa73a82556ce6dc8e6a5a7dc3f35d2cd25"
    end
  end

  on_linux do
    on_x86_64 do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.2.3/finwipe-linux-amd64"
      sha256 "3b245c43d8f68968d81b87bfd062d8c7fb2cb7127c1de8d23af4af34c4b10256"
    end
    on_arm64 do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.2.3/finwipe-linux-arm64"
      sha256 "aa65c24fe5f65997a3dc73c10e155f47e9bc5338ee61b256fdc425deaa7b353f"
    end
  end

  def install
    bin.install "finwipe-darwin-amd64" => "finwipe" if OS.mac? && Hardware::CPU.intel?
    bin.install "finwipe-darwin-arm64" => "finwipe" if OS.mac? && Hardware::CPU.arm?
    bin.install "finwipe-linux-amd64" => "finwipe" if OS.linux? && Hardware::CPU.intel?
    bin.install "finwipe-linux-arm64" => "finwipe" if OS.linux? && Hardware::CPU.arm?
  end
end
