#!/bin/bash
set -euxo pipefail

if [ $# -ne 1 ]; then
	echo "Usage: $0 [language]"
	exit 1
fi

cd "$(dirname "$0")"
mkdir -p ./build
mkdir -p ./build/assets

cp ./$1/index.html ./build/
cp ./$1/messages.js ./build/assets/
cp -r ./telwidget ./build/assets/
cp -r ./fonts ./build/assets/
cp -r ./images ./build/assets/
cp ./common.css ./common.js ./build/assets
cp ./node_modules/jquery/dist/jquery.min.js ./build/assets/
cp ./node_modules/@privacybydesign/yivi-frontend/dist/yivi.js ./build/assets/
