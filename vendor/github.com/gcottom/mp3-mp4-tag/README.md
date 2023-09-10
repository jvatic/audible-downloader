# mp3-mp4-tag


ID3/MP4/VorbisComment tag reader and writer for go


[![Go Reference](https://pkg.go.dev/badge/github.com/gcottom/mp3-mp4-tag.svg)](https://pkg.go.dev/github.com/gcottom/mp3-mp4-tag)


This package allows you to parse and write ID tags for mp3, mp4 (m4a, m4b, m4p), FLAC, and ogg (Vorbis, OPUS) files.

This is the only package available in Go that uses native Go to allow writing of ogg vorbis metadata. As an added bonus I've added support for ogg OPUS as well.

Simply use the OpenTag() function with a string file path as the argument and the library will return the IDTag.

You can access all of the fields of the IDTag through the accessor functions.


Fields that can be parsed:

MP3: Artist, AlbumArtist, Album, AlbumArt, Comments, Composer, Genre, Title, Year, BPM, ContentType, CopyrightMessage, Date, EncodedBy, Lyricist, FileType, Language, Length, PartOfSet, and Publisher

MP4: Artist, AlbumArtist, Album, AlbumArt, Comments, Composer, Genre, Title, Year, EncodedBy, and CopyrightMessage

FLAC: Artist, Album, AlbumArt, Date, Genre Title

OGG: Artist, AlbumArtist, Album, AlbumArt, Comment, Date, Genre, Title, Copyright, Publisher, Composer, and has extended support for all other custom or unmapped fields


Fields that can be written: 

MP3: Artist, AlbumArtist, Album, AlbumArt, Comments, Composer, Genre, Title, Year, BPM, ContentType, CopyrightMessage, Date, EncodedBy, Lyricist, FileType, Language, Length, PartOfSet, and Publisher

MP4: Artist, AlbumArtist, Album, AlbumArt, Comments, Composer, Genre, Title, Year, and CopyrightMessage

FLAC: Artist, Album, AlbumArt, Genre, Title

OGG: Artist, AlbumArtist, Album, AlbumArt, Comment, Date, Genre, Title, Copyright, Publisher, Composer, and has extended support to allow for passthrough of unknown fields and adding any custom or unmapped field