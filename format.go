package libmangal

//go:generate enumer -type=Format -trimprefix=Format

// Format is the format for saving chapters
type Format uint8

const (
	// FormatPDF saves chapter as a PDF document
	FormatPDF Format = iota + 1

	// FormatImages saves chapter as a directory of plain images
	FormatImages

	// FormatCBZ saves chapter as CBZ archive.
	// CBZ stands for Comic Book Zip format.
	// Common among comic readers
	FormatCBZ
)

var FormatExtensions = map[Format]string{
	FormatPDF:    ".pdf",
	FormatCBZ:    ".cbz",
	FormatImages: "",
}
