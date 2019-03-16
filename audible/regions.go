package audible

type Region struct {
	Name string
	TLD  string
}

var Regions = []Region{
	{"Australia", "com.au"},
	{"Canada", "ca"},
	{"France", "fr"},
	{"Germany", "de"},
	{"India", "in"},
	{"Italy", "it"},
	{"Japan", "co.jp"},
	{"United Kingdom", "co.uk"},
	{"United States", "com"},
}
