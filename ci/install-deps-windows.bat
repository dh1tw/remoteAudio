SET PATH=%MSYS_PATH%\mingw%MSYS2_BITS%\bin;%PATH%

ECHO "install-deps starting..."
%MSYS_PATH%\usr\bin\bash -lc "pacman --noconfirm --needed -Sy make"
%MSYS_PATH%\usr\bin\bash -lc "pacman --noconfirm --needed -Sy mingw-w64-%MSYS2_ARCH%-gcc"
%MSYS_PATH%\usr\bin\bash -lc "pacman --noconfirm --needed -Sy mingw-w64-%MSYS2_ARCH%-pkg-config"
%MSYS_PATH%\usr\bin\bash -lc "pacman --noconfirm --needed -Sy mingw-w64-%MSYS2_ARCH%-libsamplerate"
%MSYS_PATH%\usr\bin\bash -lc "pacman --noconfirm --needed -Sy mingw-w64-%MSYS2_ARCH%-portaudio"
%MSYS_PATH%\usr\bin\bash -lc "pacman --noconfirm --needed -Sy mingw-w64-%MSYS2_ARCH%-opusfile"
%MSYS_PATH%\usr\bin\bash -lc "pacman --noconfirm --needed -Sy mingw-w64-%MSYS2_ARCH%-opus"
%MSYS_PATH%\usr\bin\bash -lc "pacman --noconfirm --needed -Sy mingw-w64-%MSYS2_ARCH%-protobuf"
%MSYS_PATH%\usr\bin\bash -lc "pacman --noconfirm --needed -Sy curl"

ECHO "deps cleanup..."
%MSYS_PATH%\usr\bin\bash -lc "yes|pacman --noconfirm -Sc"

pacman-cross -Sy make
pacman-cross -Sy --noconfirm --needed make
pacman-cross -Sy --noconfirm --needed mingw-w64-x86_64-gcc
pacman-cross -Sy --noconfirm --needed mingw-w64-x86_64-pkg-config
pacman-cross -Sy --noconfirm --needed mingw-w64-x86_64-libsamplerate
pacman-cross -Sy --noconfirm --needed mingw-w64-x86_64-portaudio
pacman-cross -Sy --noconfirm --needed mingw-w64-x86_64-opusfile
pacman-cross -Sy --noconfirm --needed mingw-w64-x86_64-opus
pacman-cross -Sy --noconfirm --needed mingw-w64-x86_64-protobuf