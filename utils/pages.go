package utils

import "math"

func SplitIntoPages[T any](input []T, pageSize int) [][]T {
	inputLength := len(input)
	capacity := int(math.Ceil(float64(inputLength) / float64(pageSize)))
	pages := make([][]T, 0, capacity)
	for i := 0; i < inputLength; {
		end := i + pageSize
		if end > inputLength {
			end = inputLength
		}
		pages = append(pages, input[i:end])
		i = end
	}
	return pages
}
