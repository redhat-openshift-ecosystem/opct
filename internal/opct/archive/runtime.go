package archive

type RuntimeInfoItem struct {
	// Name holds the name of the item/attribute.
	Name string `json:"name"`
	// Value holds the value of the item/attribute.
	Value string `json:"value"`
	// Config is a optional field containing extra config information from the item.
	Config string `json:"config,omitempty"`
	// Time is a optional field with the timestamp in the datetime format.
	Time string `json:"time,omitempty"`
	// Total is a optional field with the calculation of total time the item take.
	Total string `json:"total,omitempty"`
	// Detal is a optional field with the calculation of difference between two timers.
	Delta string `json:"delta,omitempty"`
}
