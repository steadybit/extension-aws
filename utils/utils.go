package utils

import (
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/steadybit/extension-aws/v2/config"
)

func GetOptionalTargetAttribute(attributes map[string][]string, key string) *string {
	attr, ok := attributes[key]
	if !ok {
		return nil
	}
	return &attr[0]
}

func MatchesTagFilter(tags []types.Tag, filters []config.TagFilter) bool {
	if len(filters) == 0 {
		return true
	}

	for _, filter := range filters {
		matched := false

		for _, tag := range tags {
			if tag.Key != nil && *tag.Key == filter.Key {
				// Check if at least one value matches
				for _, filterValue := range filter.Values {
					if tag.Value != nil && *tag.Value == filterValue {
						matched = true
						break
					}
				}
			}
			if matched {
				break
			}
		}

		// If a filter didn't match any tags, return false
		if !matched {
			return false
		}
	}

	return true
}
