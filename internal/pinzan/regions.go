package pinzan

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"unicode"
)

// Region is an item accepted by the Pinzan area selector.
type Region struct {
	Label string `json:"label"`
	Code  string `json:"code"`
}

// Province groups the cities belonging to one Pinzan province area.
type Province struct {
	Label  string   `json:"label"`
	Code   string   `json:"code"`
	Cities []Region `json:"cities"`
}

// RegionCatalog is directly serializable for the QR page's province selector.
type RegionCatalog struct {
	Nationwide Region     `json:"nationwide"`
	Provinces  []Province `json:"provinces"`
}

type provinceInfo struct{ label, code string }

var provinceByPrefix = map[string]provinceInfo{
	"11": {"北京市", "110000"}, "12": {"天津市", "120000"}, "13": {"河北省", "130000"},
	"14": {"山西省", "140000"}, "15": {"内蒙古", "150000"}, "21": {"辽宁省", "210000"},
	"22": {"吉林省", "220000"}, "23": {"黑龙江省", "230000"}, "31": {"上海市", "310000"},
	"32": {"江苏省", "320000"}, "33": {"浙江省", "330000"}, "34": {"安徽省", "340000"},
	"35": {"福建省", "350000"}, "36": {"江西省", "360000"}, "37": {"山东省", "370000"},
	"41": {"河南省", "410000"}, "42": {"湖北省", "420000"}, "43": {"湖南省", "430000"},
	"44": {"广东省", "440000"}, "45": {"广西", "450000"}, "46": {"海南省", "460000"},
	"50": {"重庆市", "500000"},
	"51": {"四川省", "510000"}, "52": {"贵州省", "520000"}, "53": {"云南省", "530000"},
	"54": {"西藏", "540000"}, "61": {"陕西省", "610000"}, "62": {"甘肃省", "620000"},
	"63": {"青海省", "630000"}, "64": {"宁夏", "640000"}, "65": {"新疆", "650000"},
}

// ParseRegionCatalog parses the UTF-8 地区表 format: label followed by all or
// exactly six ASCII digits. Duplicate codes are replaced by their last record.
func ParseRegionCatalog(data []byte) (RegionCatalog, error) {
	type record struct {
		region Region
		order  int
	}
	var records []record
	byCode := make(map[string]int)
	s := bufio.NewScanner(bytes.NewReader(data))
	lineNo := 0
	for s.Scan() {
		lineNo++
		line := stripFormatChars(s.Text())
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] == "" || !ValidRegionCode(fields[1]) {
			return RegionCatalog{}, fmt.Errorf("地区表第 %d 行格式无效: %q", lineNo, s.Text())
		}
		r := Region{Label: fields[0], Code: fields[1]}
		if old, ok := byCode[r.Code]; ok {
			records[old].region = r
		} else {
			byCode[r.Code] = len(records)
			records = append(records, record{r, len(records)})
		}
	}
	if err := s.Err(); err != nil {
		return RegionCatalog{}, fmt.Errorf("读取地区表失败: %w", err)
	}
	var out RegionCatalog
	provinces := make(map[string]*Province)
	provinceOrder := make([]string, 0)
	for _, rec := range records {
		if rec.region.Code == "all" {
			out.Nationwide = rec.region
			continue
		}
		prefix := rec.region.Code[:2]
		if prefix == "57" {
			prefix = "46"
		}
		if prefix == "83" || prefix == "84" {
			prefix = "65"
		}
		info, ok := provinceByPrefix[prefix]
		if !ok {
			return RegionCatalog{}, fmt.Errorf("地区表第 %d 行缺少省份映射: 编码 %s", rec.order+1, rec.region.Code)
		}
		p, exists := provinces[prefix]
		if !exists {
			p = &Province{Label: info.label, Code: info.code}
			provinces[prefix] = p
			provinceOrder = append(provinceOrder, prefix)
		}
		if rec.region.Code == info.code {
			p.Label = rec.region.Label
			p.Code = rec.region.Code
			continue
		}
		p.Cities = append(p.Cities, rec.region)
	}
	if out.Nationwide.Code != "all" {
		return RegionCatalog{}, fmt.Errorf("地区表缺少全国映射")
	}
	for _, prefix := range provinceOrder {
		out.Provinces = append(out.Provinces, *provinces[prefix])
	}
	return out, nil
}

func stripFormatChars(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
}

func ValidRegionCode(s string) bool {
	if s == "all" {
		return true
	}
	if len(s) != 6 {
		return false
	}
	for _, b := range []byte(s) {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}

func (c RegionCatalog) HasCode(code string) bool {
	if c.Nationwide.Code == code {
		return true
	}
	for _, province := range c.Provinces {
		if province.Code == code {
			return true
		}
		for _, city := range province.Cities {
			if city.Code == code {
				return true
			}
		}
	}
	return false
}
