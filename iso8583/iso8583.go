package iso8583

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultSpecFile string = "isopackager.yml"

var isoConfig map[int]FieldConfig

type ISO8583Object interface {
	Parse(message string) error
	ComposeMessage() (string, error)
	GetField(index int) string
	GetMTI() string
	SetField(index int, val any)
	SetMTI(val string)
	Clear()
	PrettyPrint() string
}

type FieldConfig struct {
	ContentType string `yaml:"ContentType"`
	Label       string `yaml:"Label"`
	LenType     string `yaml:"LenType"`
	MaxLen      int    `yaml:"MaxLen"`
}

type isoObject struct {
	MTI        string
	Bitmap     string
	isoElement map[int]string
}

func Load(specFile string) (er error) {
	data, er := os.ReadFile(specFile)
	if er != nil {
		return er
	}

	isoConfig = make(map[int]FieldConfig)
	if er := yaml.Unmarshal(data, &isoConfig); er != nil {
		return er
	}

	return
}

func NewISO8583() (ISO8583Object, error) {
	if isoConfig == nil {
		return nil, errors.New("load iso 8583 spesification first")
	}

	return &isoObject{
		isoElement: make(map[int]string, 0),
	}, nil
}

func (p *isoObject) Parse(message string) error {
	// parsedData := make(map[int]string)
	pos := 0

	// Parse MTI
	mtiConfig, ok := isoConfig[0]
	if !ok {
		return errors.New("MTI configuration missing")
	}
	p.isoElement[0] = message[:mtiConfig.MaxLen]
	pos += mtiConfig.MaxLen

	// Parse Bitmap
	bitmapConfig, ok := isoConfig[1]
	if !ok {
		return errors.New("bitmap configuration missing")
	}
	bitmapHex := message[pos : pos+bitmapConfig.MaxLen]
	p.isoElement[1] = bitmapHex
	bitmapBytes, err := hex.DecodeString(bitmapHex)
	if err != nil {
		return err
	}
	pos += bitmapConfig.MaxLen

	// Process bitmap p.isoElement
	for i := 2; i <= 128; i++ {
		if (bitmapBytes[(i-1)/8] & (1 << (7 - ((i - 1) % 8)))) > 0 {
			fieldConfig, exists := isoConfig[i]
			if !exists {
				return fmt.Errorf("field %d configuration missing", i)
			}

			switch fieldConfig.LenType {
			case "fixed":
				p.isoElement[i] = message[pos : pos+fieldConfig.MaxLen]
				pos += fieldConfig.MaxLen
			case "llvar":
				length, _ := strconv.Atoi(message[pos : pos+2])
				pos += 2
				p.isoElement[i] = message[pos : pos+length]
				pos += length
			case "lllvar":
				length, _ := strconv.Atoi(message[pos : pos+3])
				pos += 3
				p.isoElement[i] = message[pos : pos+length]
				pos += length
			default:
				return fmt.Errorf("unsupported length type for field %d", i)
			}
		}
	}

	return nil
}

// ComposeMessage: Membuat message ISO8583 berdasarkan input field
func (p *isoObject) ComposeMessage() (string, error) {
	elements := p.isoElement
	if len(elements) == 0 {
		return "", errors.New("iso8583 element is empty")
	}

	if _, ok := elements[0]; !ok {
		return "", errors.New("MTI harus ada di field 0")
	}

	// Susun MTI
	message := elements[0]

	// Cek apakah ada field di atas 64 (butuh secondary bitmap)
	maxField := 0
	for k := range elements {
		if k > maxField {
			maxField = k
		}
	}

	useSecondaryBitmap := maxField > 64
	bitmapSize := 8
	if useSecondaryBitmap {
		bitmapSize = 16
	}

	// Buat bitmap kosong
	bitmap := make([]byte, bitmapSize)

	// Set bit pertama di primary bitmap kalau ada secondary
	if useSecondaryBitmap {
		bitmap[0] |= 0x80 // Set bit paling kiri ke 1
	}

	// Set active bits in bitmap
	for field := range elements {
		if field > 1 {
			byteIndex := (field - 1) / 8
			bitIndex := (field - 1) % 8
			bitmap[byteIndex] |= (1 << (7 - bitIndex))
		}
	}

	// Encode bitmap to hex (HARUS 16 byte kalau secondary aktif)
	bitmapHex := hex.EncodeToString(bitmap)
	message += strings.ToUpper(bitmapHex)

	// Susun Data Field
	for i := 2; i <= 128; i++ {
		if value, exists := elements[i]; exists {
			fieldConfig, ok := isoConfig[i]
			if !ok {
				return "", fmt.Errorf("config untuk field %d tidak ditemukan", i)
			}

			switch fieldConfig.LenType {
			case "fixed":
				value = p.padValue(value, fieldConfig.MaxLen, fieldConfig.ContentType)
				message += value
			case "llvar":
				length := fmt.Sprintf("%02d", len(value))
				message += length + value
			case "lllvar":
				length := fmt.Sprintf("%03d", len(value))
				message += length + value
			default:
				return "", fmt.Errorf("tipe panjang tidak dikenal untuk field %d", i)
			}

		}
	}

	return message, nil
}

func (p *isoObject) padValue(value string, maxLen int, contentType string) string {
	if len(value) > maxLen {
		return value[:maxLen] // Truncate jika lebih panjang dari MaxLen
	}
	if contentType == "n" {
		return fmt.Sprintf("%0*s", maxLen, value) // Padding 0 di kiri untuk numerik
	}
	return fmt.Sprintf("%-*s", maxLen, value) // Padding spasi di kanan untuk non-numerik
}

// GetField implements ISO8583Object.
func (p *isoObject) GetField(index int) string {
	return p.isoElement[index]
}

func (p *isoObject) SetMTI(val string) {
	p.isoElement[0] = val
}

// GetMTI implements ISO8583Object.
func (p *isoObject) GetMTI() string {
	return p.isoElement[0]
}

// SetField implements ISO8583Object.
func (p *isoObject) SetField(index int, val any) {
	p.isoElement[index] = fmt.Sprint(val)
}

// PrintPretty implements ISO8583Object.
func (p *isoObject) PrettyPrint() string {
	isoBuffer := []string{}

	keys := make([]int, 0)
	for k := range p.isoElement {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, k := range keys {
		isoBuffer = append(isoBuffer, fmt.Sprintf("[%03d][%s]\n", k, p.isoElement[k]))
	}
	return strings.Join(isoBuffer, "")
}

func (p *isoObject) Clear() {
	p.isoElement = make(map[int]string, 0)
}
