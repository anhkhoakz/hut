package gitsrht

import "strings"

func (cursor ReferenceCursor) Tags() []string {
	return cursor.ReferencesByType("tags")
}

func (cursor ReferenceCursor) Heads() []string {
	return cursor.ReferencesByType("heads")
}

func (cursor ReferenceCursor) ReferencesByType(refType string) []string {
	var refList []string
	for _, ref := range cursor.Results {
		split := strings.SplitN(ref.Name, "/", 3)
		if len(split) == 3 && split[1] == refType {
			refList = append(refList, split[2])
		}
	}
	return refList
}
