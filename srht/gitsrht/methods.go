package gitsrht

import "strings"

func (cursor ReferenceCursor) Tags() []string {
	var tagList []string
	for _, ref := range cursor.Results {
		split := strings.SplitN(ref.Name, "/", 3)
		if len(split) == 3 && split[1] == "tags" {
			tagList = append(tagList, split[2])
		}
	}
	return tagList
}
