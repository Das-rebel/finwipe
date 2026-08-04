class Finwipe < Formula
  desc "DIY Financial Data Deletion tool for India under DPDP Act 2023"
  homepage "https://github.com/Das-rebel/finwipe"
  license "MIT"
  version "0.2.4"

  on_macos do
    on_intel do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.2.4/finwipe-darwin-amd64"
      sha256 "9f96120a7b6ef7f352269415ecf69f72724351390cc8fd1f33ed228e6baa56dc"
    end
    on_arm do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.2.4/finwipe-darwin-arm64"
      sha256 "8a42772de09badbfcfe9c5762ee86eaa73a82556ce6dc8e6a5a7dc3f35d2cd25"
    end
  end

  on_linux do
    on_x86_64 do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.2.4/finwipe-linux-amd64"
      sha256 "7079ff8b9389f92e05688d303541b89e26ec08ac8794714188e04b1458c13f9b"
    end
    on_arm64 do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.2.4/finwipe-linux-arm64"
      sha256 "e3b7d1ea1c0752b5e8e1dbe8c50cd4078a2ffcbeb1f47390f26d706625d40ff7"
    end
  end

  def install
    bin.install "finwipe-darwin-amd64" => "finwipe" if OS.mac? && Hardware::CPU.intel?
    bin.install "finwipe-darwin-arm64" => "finwipe" if OS.mac? && Hardware::CPU.arm?
    bin.install "finwipe-linux-amd64" => "finwipe" if OS.linux? && Hardware::CPU.intel?
    bin.install "finwipe-linux-arm64" => "finwipe" if OS.linux? && Hardware::CPU.arm?
  end
end
