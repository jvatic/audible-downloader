package flacvorbis

const (
	APP_VERSION = "0.1.0"
)

const (
	// FIELD_TITLE Track/Work name
	FIELD_TITLE = "TITLE"
	// FIELD_VERSION The version field may be used to differentiate multiple versions of the same track title in a single collection. (e.g. remix info)
	FIELD_VERSION = "VERSION"
	// FIELD_ALBUM The collection name to which this track belongs
	FIELD_ALBUM = "ALBUM"
	// FIELD_TRACKNUMBER The track number of this piece if part of a specific larger collection or album
	FIELD_TRACKNUMBER = "TRACKNUMBER"
	// FIELD_ARTIST The artist generally considered responsible for the work. In popular music this is usually the performing band or singer. For classical music it would be the composer. For an audio book it would be the author of the original text.
	FIELD_ARTIST = "ARTIST"
	// FIELD_PERFORMER The artist(s) who performed the work. In classical music this would be the conductor, orchestra, soloists. In an audio book it would be the actor who did the reading. In popular music this is typically the same as the ARTIST and is omitted.
	FIELD_PERFORMER = "PERFORMER"
	// FIELD_COPYRIGHT Copyright attribution, e.g., '2001 Nobody's Band' or '1999 Jack Moffitt'
	FIELD_COPYRIGHT = "COPYRIGHT"
	// FIELD_LICENSE License information, eg, 'All Rights Reserved', 'Any Use Permitted', a URL to a license such as a Creative Commons license ("www.creativecommons.org/blahblah/license.html") or the EFF Open Audio License ('distributed under the terms of the Open Audio License. see http://www.eff.org/IP/Open_licenses/eff_oal.html for details'), etc.
	FIELD_LICENSE = "LICENSE"
	// FIELD_ORGANIZATION Name of the organization producing the track (i.e. the 'record label')
	FIELD_ORGANIZATION = "ORGANIZATION"
	// FIELD_DESCRIPTION A short text description of the contents
	FIELD_DESCRIPTION = "DESCRIPTION"
	// FIELD_GENRE A short text indication of music genre
	FIELD_GENRE = "GENRE"
	// FIELD_DATE Date the track was recorded
	FIELD_DATE = "DATE"
	// FIELD_LOCATION Location where track was recorded
	FIELD_LOCATION = "LOCATION"
	// FIELD_CONTACT Contact information for the creators or distributors of the track. This could be a URL, an email address, the physical address of the producing label.
	FIELD_CONTACT = "CONTACT"
	// FIELD_ISRC ISRC number for the track; see the ISRC intro page for more information on ISRC numbers.
	FIELD_ISRC = "ISRC"
)
