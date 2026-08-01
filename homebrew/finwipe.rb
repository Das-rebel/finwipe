class Finwipe < Formula
  desc "DIY Financial Data Deletion tool for India under DPDP Act 2023"
  homepage "https://github.com/Das-rebel/finwipe"
  license "MIT"
  version "0.1.10"

  on_macos do
    on_intel do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.1.10/finwipe-darwin-amd64"
      sha256 "c05bd820fea7382fe3d335429ddc505cec0c4d1bdbd932a23778ba6f9be43219"
    end
    on_arm do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.1.10/finwipe-darwin-arm64"
      sha256 "30124441768b709d997610b2a9eb116ed393a49f3e556214a3834203f7060611"
    end
  end

  on_linux do
    on_x86_64 do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.1.10/finwipe-linux-amd64"
      sha256 "c22d44b7d38a88de66ea4e41c8c76edad0bf578e511ab03f29a9133417897fd6"
    end
    on_arm64 do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.1.10/finwipe-linux-arm64"
      sha256 "1f75673789065cc356b13d71971eaee85276d6a754e2a4d8f968b0869c22aba3"
    end
  end

  def install
    bin.install "finwipe-darwin-amd64" => "finwipe" if OS.mac? && Hardware::CPU.intel?
    bin.install "finwipe-darwin-arm64" => "finwipe" if OS.mac? && Hardware::CPU.arm?
    bin.install "finwipe-linux-amd64" => "finwipe" if OS.linux? && Hardware::CPU.intel?
    bin.install "finwipe-linux-arm64" => "finwipe" if OS.linux? && Hardware::CPU.arm?
  end
end
