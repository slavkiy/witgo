This directory is part of the Go module and contains gzip-compressed shared
libraries plus SHA-256 files for:

linux_amd64.so.gz
linux_arm64.so.gz
darwin_amd64.dylib.gz
darwin_arm64.dylib.gz
windows_amd64.dll.gz
windows_arm64.dll.gz

The release-preparation workflow replaces the zero-length development
placeholders with tested native libraries before a release tag is created.
