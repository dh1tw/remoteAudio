REM Create a release folder
mkdir %GOPATH%\src\github.com\dh1tw\remoteAudio\release\

REM copy the needed shared libraries and the binary
%MSYS_PATH%\usr\bin\bash -lc "cp /mingw%MSYS2_BITS%/**/libogg-0.dll /c/gopath/src/github.com/dh1tw/remoteAudio/release/"
%MSYS_PATH%\usr\bin\bash -lc "cp /mingw%MSYS2_BITS%/**/libopus-0.dll /c/gopath/src/github.com/dh1tw/remoteAudio/release/"
%MSYS_PATH%\usr\bin\bash -lc "cp /mingw%MSYS2_BITS%/**/libopusfile-0.dll /c/gopath/src/github.com/dh1tw/remoteAudio/release/"
%MSYS_PATH%\usr\bin\bash -lc "cp /mingw%MSYS2_BITS%/**/libportaudio-2.dll /c/gopath/src/github.com/dh1tw/remoteAudio/release/"
%MSYS_PATH%\usr\bin\bash -lc "cp /mingw%MSYS2_BITS%/**/libsamplerate-0.dll /c/gopath/src/github.com/dh1tw/remoteAudio/release/"
REM %MSYS_PATH%\usr\bin\bash -lc "cd /c/gopath/src/github.com/dh1tw/remoteAudio && ci/release"
%MSYS_PATH%\usr\bin\bash -lc "cp /c/gopath/src/github.com/dh1tw/remoteAudio/remoteAudio.exe /c/gopath/src/github.com/dh1tw/remoteAudio/release"

REM zip everything
%MSYS_PATH%\usr\bin\bash -lc "cd /c/gopath/src/github.com/dh1tw/remoteAudio/release && 7z a -tzip remoteAudio-v$APPVEYOR_REPO_TAG_NAME-$GOARCH-$GOOS.zip *"

REM copy it into the build folder
xcopy %GOPATH%\src\github.com\dh1tw\remoteAudio\release\remoteAudio-v%APPVEYOR_REPO_TAG_NAME%-%GOARCH%-%GOOS%.zip %APPVEYOR_BUILD_FOLDER%\ /e /i > nul