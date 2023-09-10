package mp4tag

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/abema/go-mp4"
	"github.com/sunfish-shogi/bufseekio"
)

var atomsMap = map[string]mp4.BoxType{
	"Album":       {'\251', 'a', 'l', 'b'},
	"AlbumArtist": {'a', 'A', 'R', 'T'},
	"Artist":      {'\251', 'A', 'R', 'T'},
	"Comment":     {'\251', 'c', 'm', 't'},
	"Composer":    {'\251', 'w', 'r', 't'},
	"Copyright":   {'c', 'p', 'r', 't'},
	"Cover":       {'c', 'o', 'v', 'r'},
	"Disk":        {'d', 'i', 's', 'k'},
	"Genre":       {'\251', 'g', 'e', 'n'},
	"Label":       {'\251', 'l', 'a', 'b'},
	"Title":       {'\251', 'n', 'a', 'm'},
	"Track":       {'t', 'r', 'k', 'n'},
	"Year":        {'\251', 'd', 'a', 'y'},
}

func copy(w *mp4.Writer, h *mp4.ReadHandle) error {
	_, err := w.StartBox(&h.BoxInfo)
	if err != nil {
		return err
	}
	box, _, err := h.ReadPayload()
	if err != nil {
		return err
	}
	_, err = mp4.Marshal(w, box, h.BoxInfo.Context)
	if err != nil {
		return err
	}
	_, err = h.Expand()
	if err != nil {
		return err
	}
	_, err = w.EndBox()
	return err
}

// See which atoms don't already exist and will need creating.
func populateAtoms(f *os.File, _tags *Tags) (map[string]bool, error) {
	ilst, err := mp4.ExtractBox(
		f, nil, mp4.BoxPath{mp4.BoxTypeMoov(), mp4.BoxTypeUdta(), mp4.BoxTypeMeta(), mp4.BoxTypeIlst()})
	if err != nil {
		return nil, err
	}
	if len(ilst) == 0 {
		return nil, errors.New("Ilst atom is missing. Implement me.")
	}
	atoms := map[string]bool{}
	fields := reflect.VisibleFields(reflect.TypeOf(*_tags))
	for _, field := range fields {
		fieldName := field.Name
		if fieldName == "Custom" || fieldName == "TrackTotal" || fieldName == "DiskTotal" {
			continue
		}
		if fieldName == "TrackNumber" {
			fieldName = "Track"
		} else if fieldName == "DiskNumber" {
			fieldName = "Disk"
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

func writeCustomMeta(w *mp4.Writer, ctx mp4.Context, field string, val interface{}) error {
	_, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxType{'-', '-', '-', '-'}, Context: ctx})
	if err != nil {
		return err
	}
	_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxType{'m', 'e', 'a', 'n'}, Context: ctx})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'\x00', '\x00', '\x00', '\x00'})
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, "com.apple.iTunes")
	if err != nil {
		return err
	}
	_, err = w.EndBox()
	if err != nil {
		return err
	}
	_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxType{'n', 'a', 'm', 'e'}, Context: ctx})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'\x00', '\x00', '\x00', '\x00'})
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, field)
	if err != nil {
		return err
	}
	_, err = w.EndBox()
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

// func writeCover(h *mp4.ReadHandle, w *mp4.Writer, ctx mp4.Context, coverData []byte) error {
// 	_, err := w.StartBox(&h.BoxInfo)
// 	if err != nil {
// 		return err
// 	}
// 	box, _, err := h.ReadPayload()
// 	if err != nil {
// 		return err
// 	}
// 	_, err = mp4.Marshal(w, box, h.BoxInfo.Context)
// 	if err != nil {
// 		return err
// 	}
// 	err = writeMeta(w, h.BoxInfo.Type, ctx, coverData)
// 	if err != nil {
// 		return err
// 	}
// 	_, err = w.EndBox()
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

// Make new atoms and write to.
func createAndWrite(h *mp4.ReadHandle, w *mp4.Writer, ctx mp4.Context, _tags *Tags, atoms map[string]bool) error {
	_, err := w.StartBox(&h.BoxInfo)
	if err != nil {
		return err
	}
	box, _, err := h.ReadPayload()
	if err != nil {
		return err
	}
	_, err = mp4.Marshal(w, box, h.BoxInfo.Context)
	if err != nil {
		return err
	}
	if _tags.Cover != nil {
		err := writeMeta(w, atomsMap["Cover"], ctx, _tags.Cover)
		if err != nil {
			return err
		}
	}
	if _tags.TrackNumber > 0 {
		trkn := make([]byte, 8)
		binary.BigEndian.PutUint32(trkn, uint32(_tags.TrackNumber))
		if _tags.TrackTotal > 0 {
			binary.BigEndian.PutUint16(trkn[4:], uint16(_tags.TrackTotal))
		}
		err = writeMeta(w, atomsMap["Track"], ctx, trkn)
		if err != nil {
			return err
		}
	}
	if _tags.DiskNumber > 0 {
		disk := make([]byte, 8)
		binary.BigEndian.PutUint32(disk, uint32(_tags.DiskNumber))
		if _tags.DiskTotal > 0 {
			binary.BigEndian.PutUint16(disk[4:], uint16(_tags.DiskTotal))
		}
		err = writeMeta(w, atomsMap["Disk"], ctx, disk)
		if err != nil {
			return err
		}
	}
	for tagName, needCreate := range atoms {
		if tagName == "Cover" || tagName == "Track" || tagName == "Disk" {
			continue
		}
		val := reflect.ValueOf(*_tags).FieldByName(tagName).String()
		if !needCreate || val == "" {
			continue
		}
		boxType := atomsMap[tagName]
		err = writeMeta(w, boxType, ctx, val)
		if err != nil {
			return err
		}
	}
	for field, value := range _tags.Custom {
		if value == "" {
			continue
		}
		err = writeCustomMeta(w, ctx, field, strings.ToUpper(value))
		if err != nil {
			return err
		}
	}
	_, err = h.Expand()
	if err != nil {
		return err
	}
	_, err = w.EndBox()
	return err
}

func writeExisting(h *mp4.ReadHandle, w *mp4.Writer, _tags *Tags, currentKey string, ctx mp4.Context) (bool, error) {
	if currentKey == "Cover" && _tags.Cover == nil {
		return true, nil
	}
	if currentKey == "Cover" && _tags.Cover != nil {
		// err := writeCover(h, w, ctx, _tags.Cover)
		// if err != nil {
		// 	return false, nil
		// }
		//err := writeMeta(w, atomsMap["Cover"], ctx, _tags.Cover)
		_, err := w.StartBox(&h.BoxInfo)
		if err != nil {
			return false, err
		}
		box, _, err := h.ReadPayload()
		if err != nil {
			return false, err
		}
		data := box.(*mp4.Data)
		data.DataType = mp4.DataTypeBinary
		data.Data = []byte(_tags.Cover)
		_, err = mp4.Marshal(w, data, h.BoxInfo.Context)
		if err != nil {
			return false, err
		}
		_, err = w.EndBox()
		if err != nil {
			return false, err
		}
		// err := writeMeta(w, h.BoxInfo.Type, ctx, _tags.Cover)
		// if err != nil {
		// 	return false, err
		// }
	} else if currentKey == "Disk" {
		if _tags.DiskNumber < 1 {
			return true, nil
		}
		// disk := make([]byte, 8)
		// binary.BigEndian.PutUint32(disk, uint32(_tags.DiskNumber))
		// if _tags.DiskTotal > 0 {
		// 	binary.BigEndian.PutUint16(disk[4:], uint16(_tags.DiskTotal))
		// }
		// err := writeMeta(w, h.BoxInfo.Type, ctx, disk)
		// if err != nil {
		// 	return false, err
		// }
	} else if currentKey == "Track" {
		if _tags.TrackNumber < 1 {
			return true, nil
		}
		// trkn := make([]byte, 8)
		// binary.BigEndian.PutUint32(trkn, uint32(_tags.TrackNumber))
		// if _tags.TrackTotal > 0 {
		// 	binary.BigEndian.PutUint16(trkn[4:], uint16(_tags.TrackTotal))
		// }
		// // err := writeMeta(w, h.BoxInfo.Type, ctx, trkn)
	} else {
		toWrite := reflect.ValueOf(*_tags).FieldByName(currentKey).String()
		if toWrite == "" {
			return true, nil
		}
		// Not working here.
		// err := writeMeta(w, h.BoxInfo.Type, ctx, toWrite)
		// if err != nil {
		// 	return false, err
		// }
		_, err := w.StartBox(&h.BoxInfo)
		if err != nil {
			return false, err
		}
		box, _, err := h.ReadPayload()
		if err != nil {
			return false, err
		}
		data := box.(*mp4.Data)
		data.DataType = mp4.DataTypeStringUTF8
		data.Data = []byte(toWrite)
		_, err = mp4.Marshal(w, data, h.BoxInfo.Context)
		if err != nil {
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

func copyTrack(srcPath, destPath string) error {
	srcFile, err := os.OpenFile(srcPath, os.O_RDONLY, 0755)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer outFile.Close()
	_, err = io.Copy(outFile, srcFile)
	return err
}

func Write(trackPath string, _tags *Tags) error {
	var currentKey string
	ctx := mp4.Context{UnderIlstMeta: true}
	tempPath, err := os.MkdirTemp(os.TempDir(), "go-mp4tag")
	if err != nil {
		return errors.New(
			"Failed to make temp directory.\n" + err.Error())
	}
	defer os.RemoveAll(tempPath)
	tempPath = filepath.Join(tempPath, "tmp.m4a")
	atomsList := getAtomsList()
	outFile, err := os.OpenFile(trackPath, os.O_RDONLY, 0755)
	if err != nil {
		return err
	}
	tempFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		outFile.Close()
		return err
	}
	r := bufseekio.NewReadSeeker(outFile, 128*1024, 4)
	atoms, err := populateAtoms(outFile, _tags)
	if err != nil {
		outFile.Close()
		tempFile.Close()
		return err
	}
	w := mp4.NewWriter(tempFile)
	_, err = mp4.ReadBoxStructure(r, func(h *mp4.ReadHandle) (interface{}, error) {
		switch h.BoxInfo.Type {
		case mp4.BoxTypeMoov(), mp4.BoxTypeUdta(), mp4.BoxTypeMeta():
			err := copy(w, h)
			return nil, err
		case mp4.BoxTypeIlst():
			err := createAndWrite(h, w, ctx, _tags, atoms)
			return nil, err
		case containsAtom(h.BoxInfo.Type, atomsList):
			if h.BoxInfo.Type == atomsMap["Cover"] && _tags.Cover != nil {
				return nil, nil
			}
			currentKey = getTag(h.BoxInfo.Type)
			if containsTag(_tags.Delete, currentKey) {
				return nil, nil
			}
			err = copy(w, h)
			return nil, err
		case mp4.BoxTypeData():
			if currentKey == "" {
				return nil, w.CopyBox(r, &h.BoxInfo)
			}
			needCreate := atoms[currentKey]
			if !needCreate {
				valEmpty, err := writeExisting(h, w, _tags, currentKey, ctx)
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
	outFile.Close()
	tempFile.Close()
	if err != nil {
		return err
	}
	err = copyTrack(tempPath, trackPath)
	return err
}
