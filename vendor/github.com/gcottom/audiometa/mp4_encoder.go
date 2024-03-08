package audiometa

import (
	"bytes"
	"errors"
	"image/png"
	"os"
	"reflect"
	"strings"

	"github.com/abema/go-mp4"
	"github.com/aler9/writerseeker"
	"github.com/sunfish-shogi/bufseekio"
)

var atomsMap = map[string]mp4.BoxType{
	"album":        {'\251', 'a', 'l', 'b'},
	"albumArtist":  {'a', 'A', 'R', 'T'},
	"artist":       {'\251', 'A', 'R', 'T'},
	"comments":     {'\251', 'c', 'm', 't'},
	"composer":     {'\251', 'w', 'r', 't'},
	"copyrightMsg": {'c', 'p', 'r', 't'},
	"albumArt":     {'c', 'o', 'v', 'r'},
	"genre":        {'\251', 'g', 'e', 'n'},
	"title":        {'\251', 'n', 'a', 'm'},
	"year":         {'\251', 'd', 'a', 'y'},
}

func mp4copy(w *mp4.Writer, h *mp4.ReadHandle) error {
	if _, err := w.StartBox(&h.BoxInfo); err != nil {
		return err
	}
	box, _, err := h.ReadPayload()
	if err != nil {
		return err
	}
	if _, err = mp4.Marshal(w, box, h.BoxInfo.Context); err != nil {
		return err
	}
	if _, err = h.Expand(); err != nil {
		return err
	}
	_, err = w.EndBox()
	return err
}

// See which atoms don't already exist and will need creating.
func populateAtoms(f *os.File, _tags *IDTag) (map[string]bool, error) {
	ilst, err := mp4.ExtractBox(
		f, nil, mp4.BoxPath{mp4.BoxTypeMoov(), mp4.BoxTypeUdta(), mp4.BoxTypeMeta(), mp4.BoxTypeIlst()})
	if err != nil {
		return nil, err
	}
	if len(ilst) == 0 {
		return nil, errors.New("ilst atom is missing")
	}
	atoms := map[string]bool{}
	fields := reflect.VisibleFields(reflect.TypeOf(*_tags))
	for _, field := range fields {
		fieldName := field.Name
		if fieldName == "Custom" {
			continue
		}
		boxType, ok := atomsMap[fieldName]
		if !ok {
			continue
		}
		boxes, err := mp4.ExtractBox(
			f, nil, mp4.BoxPath{mp4.BoxTypeMoov(), mp4.BoxTypeUdta(), mp4.BoxTypeMeta(), mp4.BoxTypeIlst(), boxType})
		if err != nil {
			return nil, err
		}
		atoms[fieldName] = len(boxes) == 0
	}
	return atoms, nil
}

func marshalData(w *mp4.Writer, ctx mp4.Context, val interface{}) error {
	_, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeData()})
	if err != nil {
		return err
	}
	var boxData mp4.Data
	switch v := val.(type) {
	case string:
		boxData.DataType = mp4.DataTypeStringUTF8
		boxData.Data = []byte(v)
	case []byte:
		boxData.DataType = mp4.DataTypeBinary
		boxData.Data = v
	}
	_, err = mp4.Marshal(w, &boxData, ctx)
	if err != nil {
		return err
	}
	_, err = w.EndBox()
	return err
}

func writeMeta(w *mp4.Writer, tag mp4.BoxType, ctx mp4.Context, val interface{}) error {
	_, err := w.StartBox(&mp4.BoxInfo{Type: tag})
	if err != nil {
		return err
	}
	err = marshalData(w, ctx, val)
	if err != nil {
		return err
	}
	_, err = w.EndBox()
	return err
}

// Make new atoms and write to.
func createAndWrite(h *mp4.ReadHandle, w *mp4.Writer, ctx mp4.Context, _tags *IDTag, atoms map[string]bool) error {
	if _, err := w.StartBox(&h.BoxInfo); err != nil {
		return err
	}
	box, _, err := h.ReadPayload()
	if err != nil {
		return err
	}
	if _, err = mp4.Marshal(w, box, h.BoxInfo.Context); err != nil {
		return err
	}
	if _tags.albumArt != nil {
		buf := new(bytes.Buffer)
		if err := png.Encode(buf, *_tags.albumArt); err != nil {
			if err := writeMeta(w, atomsMap["albumArt"], ctx, buf.Bytes()); err != nil {
				return err
			}
		}
	}
	for tagName, needCreate := range atoms {
		if tagName == "albumArt" {
			continue
		}
		val := reflect.ValueOf(*_tags).FieldByName(tagName).String()
		if !needCreate || val == "" {
			continue
		}
		boxType := atomsMap[tagName]
		if err = writeMeta(w, boxType, ctx, val); err != nil {
			return err
		}
	}
	if _, err = h.Expand(); err != nil {
		return err
	}
	_, err = w.EndBox()
	return err
}

func writeExisting(h *mp4.ReadHandle, w *mp4.Writer, _tags *IDTag, currentKey string) (bool, error) {
	if currentKey == "albumArt" && _tags.albumArt == nil {
		return true, nil
	}
	if currentKey == "albumArt" && _tags.albumArt != nil {
		if _, err := w.StartBox(&h.BoxInfo); err != nil {
			return false, err
		}
		box, _, err := h.ReadPayload()
		if err != nil {
			return false, err
		}
		data := box.(*mp4.Data)
		data.DataType = mp4.DataTypeBinary
		buf := new(bytes.Buffer)
		if err := png.Encode(buf, *_tags.albumArt); err != nil {
			data.Data = buf.Bytes()
		}
		if _, err = mp4.Marshal(w, data, h.BoxInfo.Context); err != nil {
			return false, err
		}
		if _, err = w.EndBox(); err != nil {
			return false, err
		}
	} else {
		toWrite := reflect.ValueOf(*_tags).FieldByName(currentKey).String()
		if toWrite == "" {
			return true, nil
		}
		if _, err := w.StartBox(&h.BoxInfo); err != nil {
			return false, err
		}
		box, _, err := h.ReadPayload()
		if err != nil {
			return false, err
		}
		data := box.(*mp4.Data)
		data.DataType = mp4.DataTypeStringUTF8
		data.Data = []byte(toWrite)
		if _, err = mp4.Marshal(w, data, h.BoxInfo.Context); err != nil {
			return false, err
		}
		_, err = w.EndBox()
		return false, err

	}
	return false, nil
}

func containsAtom(boxType mp4.BoxType, boxes []mp4.BoxType) mp4.BoxType {
	for _, _boxType := range boxes {
		if boxType == _boxType {
			return boxType
		}
	}
	return mp4.BoxType{}
}

func containsTag(delete []string, currentTag string) bool {
	for _, tag := range delete {
		if strings.EqualFold(tag, currentTag) {
			return true
		}
	}
	return false
}

func getTag(boxType mp4.BoxType) string {
	for k, v := range atomsMap {
		if v == boxType {
			return k
		}
	}
	return ""
}

func getAtomsList() []mp4.BoxType {
	var atomsList []mp4.BoxType
	for _, atom := range atomsMap {
		atomsList = append(atomsList, atom)
	}
	return atomsList
}

func WriteMP4(trackPath string, _tags *IDTag, delete MP4Delete) error {
	var currentKey string
	ctx := mp4.Context{UnderIlstMeta: true}
	atomsList := getAtomsList()
	outFile, err := os.OpenFile(trackPath, os.O_RDONLY, 0755)
	if err != nil {
		return err
	}
	defer outFile.Close()

	ws := &writerseeker.WriterSeeker{}
	r := bufseekio.NewReadSeeker(outFile, 128*1024, 4)
	atoms, err := populateAtoms(outFile, _tags)
	if err != nil {
		return err
	}
	w := mp4.NewWriter(ws)
	_, err = mp4.ReadBoxStructure(r, func(h *mp4.ReadHandle) (interface{}, error) {
		switch h.BoxInfo.Type {
		case mp4.BoxTypeMoov(), mp4.BoxTypeUdta(), mp4.BoxTypeMeta():
			err := mp4copy(w, h)
			return nil, err
		case mp4.BoxTypeIlst():
			err := createAndWrite(h, w, ctx, _tags, atoms)
			return nil, err
		case containsAtom(h.BoxInfo.Type, atomsList):
			if h.BoxInfo.Type == atomsMap["albumArt"] && _tags.albumArt != nil {
				return nil, nil
			}
			currentKey = getTag(h.BoxInfo.Type)
			if containsTag(delete, currentKey) {
				return nil, nil
			}
			err = mp4copy(w, h)
			return nil, err
		case mp4.BoxTypeData():
			if currentKey == "" {
				return nil, w.CopyBox(r, &h.BoxInfo)
			}
			if !atoms[currentKey] {
				valEmpty, err := writeExisting(h, w, _tags, currentKey)
				currentKey = ""
				if err != nil {
					return nil, err
				} else if valEmpty {
					return nil, w.CopyBox(r, &h.BoxInfo)
				}
			}
			return nil, nil
		default:
			return nil, w.CopyBox(r, &h.BoxInfo)
		}
	})
	if err != nil {
		return err
	}
	outFile.Close()
	file, err := os.OpenFile(trackPath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err = file.Write(ws.Bytes()); err != nil {
		return err
	}
	return nil
}
