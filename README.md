audible-downloader
==================

Download your Audible library for personal use/backup. Please do not use for illegal purposes.

## Usage

1. Make sure you have [ffmpeg](https://ffmpeg.org/download.html) installed and available via `ffmpeg` in your PATH.
1. Download the latest binary for your system from the [releases](https://github.com/jvatic/audible-downloader/releases) page (binaries are listed in the assets for each release).
1. Run the downloaded binary and follow the prompts.

This will download your entire Audible library and remove the DRM for your personal use.

## Build

Run `make` to build all targets except GUI for windows (see below).

### Windows GUI

You will need Docker running for this to work as [xgo](https://github.com/techknowlogick/xgo) is used.

To build just the Windows GUI, run `make windows_gui`. And to build everything, run `make build_all`.
