package libmangal

//go:generate enumer -type=Format -trimprefix=Format
type Format int

const (
	FormatPDF Format = iota + 1
	FormatImages
	FormatCBZ
)
