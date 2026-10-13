package leaf

// A private mutable static backing array must remain a single object even
// when an accessor is inlined into a different package.
var Values = []int{11, 12, 13}

func At(i int) int { return Values[i] }
