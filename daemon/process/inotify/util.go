package inotify

import (
	"sort"
	"strings"
)

func omitChildrenDirectories(dirs []string) []string {
	sort.Strings(dirs) // sort to put the parent directories first

	// keep track for uniqueness
	set := map[string]struct{}{}

	var newVols []string

	omitted := map[int]struct{}{}
	for i := 0; i < len(dirs); i++ {
		// if the index is ommitted, skip
		if _, ok := omitted[i]; ok {
			continue
		}

		parent := dirs[i]
		if _, ok := set[parent]; !ok {
			newVols = append(newVols, parent)
			set[parent] = struct{}{}
		}

		for j := i + 1; j < len(dirs); j++ {
			child := dirs[j]
			if strings.HasPrefix(child, strings.TrimSuffix(parent, "/")+"/") {
				omitted[j] = struct{}{}
			}
		}
	}

	return newVols
}
