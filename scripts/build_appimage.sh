#!/bin/bash
set -e

cd gui-installer

echo "==> Сборка приложения через Wails..."
wails build -tags webkit2_41

APP_DIR="build/linux/AppDir"
BIN_NAME="gui-installer"

mkdir -p "$APP_DIR/usr/bin"
cp "build/bin/$BIN_NAME" "$APP_DIR/usr/bin/"

if [ ! -f "./appimagetool" ]; then
    echo "==> Скачивание appimagetool..."
    curl -LO https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage
    chmod +x appimagetool-x86_64.AppImage
    mv appimagetool-x86_64.AppImage appimagetool
fi
echo "==> Исправление окончаний строк (CRLF -> LF)..."
sed -i 's/\r$//' "$APP_DIR/servehub.desktop"
sed -i 's/\r$//' "$APP_DIR/AppRun"

echo "==> Упаковка в AppImage..."
export ARCH=x86_64
./appimagetool "$APP_DIR" "build/bin/ServeHub-Installer-x86_64.AppImage"

echo "==> Готово! Файл сохранен в build/bin/ServeHub-Installer-x86_64.AppImage"