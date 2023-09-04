rm -rf build
mkdir build

trimpath=$PWD
echo "Trimming path $trimpath"

version=$1

if [ -z "$version" ]; then
    echo "Version is not specified (first positional argument)"
    exit 1
fi

function build() {
    file_name="linsk_${1}_${2}_${version}"
    CGO_ENABLED=0 GOOS=$1 GOARCH=$2 go build -trimpath -o build/$file_name
    cd build
    zip $file_name.zip $file_name
    rm $file_name
    cd ..
}

build windows amd64
build darwin amd64
build darwin arm64

cd build

hashes_file="linsk_sha256_$version.txt"

sha256sum * > $hashes_file
gpg --output ${hashes_file}.sig --detach-sign --armor $hashes_file