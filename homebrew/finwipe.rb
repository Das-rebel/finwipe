class Finwipe < Formula
  desc "DIY Financial Data Deletion tool for India under DPDP Act 2023"
  homepage "https://github.com/Das-rebel/finwipe"
  license "MIT"
  version "0.1.6"

  on_macos do
    on_intel do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.1.6/finwipe-darwin-amd64"
      sha256 "00d7901a6023aa9e17d78bf1ca10f3cf9c070b330c9aab079a20315cf47763e4"
      def install
        bin.install "finwipe-darwin-amd64" => "finwipe"
      end
    end

    on_arm do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.1.6/finwipe-darwin-arm64"
      sha256 "f0a296d9be212b8811726150e70dae324871bce64670b323c866aa433ef198ec"
      def install
        bin.install "finwipe-darwin-arm64" => "finwipe"
      end
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.1.6/finwipe-linux-amd64"
      sha256 "018f1e678d28ae5f62c0ee91c1da9662a693af9301fd4709985d89a9f3bb9f70"
      def install
        bin.install "finwipe-linux-amd64" => "finwipe"
      end
    end

    on_arm do
      url "https://github.com/Das-rebel/finwipe/releases/download/v0.1.6/finwipe-linux-arm64"
      sha256 "425abc6598330ceb18a8b9d1c862495c27856146d8c92dd4790ce5300161595f"
      def install
        bin.install "finwipe-linux-arm64" => "finwipe"
      end
    end
  end

  def post_install
    puts "FinWipe v#{version} installed! Run: finwipe init"
  end

  test do
    assert_match "FinWipe", shell_output("#{bin}/finwipe --help")
  end
end
