rm -rf build
mkdir build

version=$1

if [ -z "$version" ]; then
    echo "Version is not specified (first positional argument)"
    exit 1
fi

function build() {
    name="linsk_${1}_${2}_${version}"
    binary_name="$name"
    if [ $1 == "windows" ]; then
        binary_name="$binary_name.exe"
    fi
    
    CGO_ENABLED=0 GOOS=$1 GOARCH=$2 go build -trimpath -o build/$binary_name
    cd build
    zip $name.zip $binary_name
    rm $binary_name
    cd ..
}

build windows amd64
build darwin amd64
build darwin arm64

cd build

hashes_file="linsk_sha256_$version.txt"

sha256sum * > $hashes_file
gpg --output ${hashes_file}.sig --detach-sign --local-user F7231DFD3333A27F71D171383B627C597D3727BD --armor $hashes_file