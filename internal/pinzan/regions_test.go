package pinzan

import "testing"

func TestParseRegionCatalog(t *testing.T) {
	input := "全国 all\n广东省 440000\n襄阳市 420600\n旧名 420600\n重庆市 500100\n万宁 571500\n石河子市 832061\n阿拉尔市 843300\n可克达拉 659008\u200c\n"
	catalog, err := ParseRegionCatalog([]byte(input))
	if err != nil {
		t.Fatalf("ParseRegionCatalog() error = %v", err)
	}
	if catalog.Nationwide.Code != "all" {
		t.Fatalf("nationwide = %#v", catalog.Nationwide)
	}
	var hainan, xinjiang *Province
	for i := range catalog.Provinces {
		switch catalog.Provinces[i].Code {
		case "460000":
			hainan = &catalog.Provinces[i]
		case "650000":
			xinjiang = &catalog.Provinces[i]
		}
	}
	if hainan == nil || xinjiang == nil {
		t.Fatalf("special provinces missing: %#v", catalog.Provinces)
	}
	if got := findRegion(hainan.Cities, "571500"); got.Label != "万宁" {
		t.Fatalf("Hainan city = %#v", got)
	}
	if got := findRegion(xinjiang.Cities, "832061"); got.Label != "石河子市" {
		t.Fatalf("Xinjiang 83 city = %#v", got)
	}
	if got := findRegion(xinjiang.Cities, "843300"); got.Label != "阿拉尔市" {
		t.Fatalf("Xinjiang 84 city = %#v", got)
	}
	if got := findRegion(xinjiang.Cities, "659008"); got.Label != "可克达拉" {
		t.Fatalf("format char stripping = %#v", got)
	}
	if !catalog.HasCode("500100") {
		t.Fatalf("Chongqing city missing: %#v", catalog.Provinces)
	}
	var hubei *Province
	for i := range catalog.Provinces {
		if catalog.Provinces[i].Code == "420000" {
			hubei = &catalog.Provinces[i]
		}
	}
	if hubei == nil || findRegion(hubei.Cities, "420600").Label != "旧名" {
		t.Fatalf("duplicate last-wins = %#v", hubei)
	}
}

func TestParseRegionCatalogRejectsMalformedAndMissingMappings(t *testing.T) {
	tests := []string{
		"全国 all\n广东省 440000\n坏 12345\n",
		"全国 all\n未知 990001\n",
		"广东省 440000\n",
	}
	for _, input := range tests {
		if _, err := ParseRegionCatalog([]byte(input)); err == nil {
			t.Errorf("input %q unexpectedly succeeded", input)
		}
	}
}

func findRegion(regions []Region, code string) Region {
	for _, region := range regions {
		if region.Code == code {
			return region
		}
	}
	return Region{}
}
