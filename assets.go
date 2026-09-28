package assets

import _ "embed"

//go:embed resource/templates/index.html
var IndexHTML string

//go:embed resource/templates/scan.html
var ScanHTML string

//go:embed 地区表.txt
var PinzanRegions []byte
