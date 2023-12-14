package audiometa

import (
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
)

func extractFLACComment(fileName string) (*flacvorbis.MetaDataBlockVorbisComment, int) {
	f, err := flac.ParseFile(fileName)
	if err != nil {
		panic(err)
	}

	var cmt *flacvorbis.MetaDataBlockVorbisComment
	var cmtIdx int
	for idx, meta := range f.Meta {
		if meta.Type == flac.VorbisComment {
			cmt, err = flacvorbis.ParseFromMetaDataBlock(*meta)
			cmtIdx = idx
			if err != nil {
				panic(err)
			}
		}
	}
	return cmt, cmtIdx
}
func getFLACPictureIndex(metaIn []*flac.MetaDataBlock) int {
	var cmtIdx = 0
	for idx, meta := range metaIn {
		if meta.Type == flac.Picture {
			cmtIdx = idx
			break
		}
	}
	return cmtIdx
}
func removeFLACMetaBlock(slice []*flac.MetaDataBlock, s int) []*flac.MetaDataBlock {
	return append(slice[:s], slice[s+1:]...)
}
