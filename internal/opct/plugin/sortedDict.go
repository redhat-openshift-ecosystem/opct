package plugin

// SortedDict stores and sorts the key/value map to be ranked by value.
type SortedDict struct {
	Key   string
	Value int
}

// SortedList stores the list of key/value map, implementing interfaces
// to sort/rank a map strings with integers as values.
type SortedList []SortedDict

func (p SortedList) Len() int           { return len(p) }
func (p SortedList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p SortedList) Less(i, j int) bool { return p[i].Value < p[j].Value }
