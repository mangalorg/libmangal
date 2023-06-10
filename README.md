# ![libmangal logo] libmangal β

> **Warning**
> 
> This is a beta software. The API is not stable and may change at any time.

This is the *engine* for downloading, managing, tagging, reading manga
with native Anilist integration. A powerful wrapper around
anything that implements its `Provider` interface.

It's designed to be a backend for various frontends that 
can be built on top of it.
Such as a CLI, a web app, a mobile app, gRPC server, etc. *Anything!*

## Features

- Smart caching - only download what you need
- Different export formats
  - PDF - chapters stored a single PDF file
  - CBZ - Comic Book ZIP format
  - Images - a plain directory of images
- Monolith - no runtime dependencies. 
- Generates metadata files
  - `ComicInfo.xml` - The ComicInfo.xml file originates from the ComicRack application, which is not developed anymore. The ComicInfo.xml however is used by a variety of applications.
  - `series.json` - A JSON file containing metadata about the series. Originates from [mylar3](https://github.com/mylar3/mylar3)
- Automatically populates missing metadata by querying [Anilist](https://anilist.co).
- Filesystem abstraction - can be used with any filesystem that implements [afero](https://github.com/spf13/afero)
    - Remote filesystems
    - In-memory filesystems
    - etc.
- Highly configurable
    - Define how you want to **name** your files
    - Define how you want to **organize** your files
    - Define how you want to **tag** your files
    - Define how you want to **cache** your files
- Cross-platform - every OS that Go compiles to is supported
    - Windows
    - Linux
    - MacOS
    - WASM
    - etc.

## Install

```bash
go get github.com/mangalorg/libmangal
```

## Providers

- [luaprovider](https://github.com/mangalorg/luaprovider) - Generic provider based on Lua scripts

[libmangal logo]: https://github.com/mangalorg/libmangal/assets/62389790/1438a6c4-9651-4071-8e46-d28b32a57250
