package webhook

import (
	"strings"

	mapset "github.com/deckarep/golang-set"
)

type ArrayFlags []string

func (i *ArrayFlags) String() string {
	// return strings.Trim(fmt.Sprint(i), "[]")
	return ""
}

func (i *ArrayFlags) Set(value string) error {
	*i = append(*i, strings.TrimSpace(value))
	return nil
}

var targetResourcesSet mapset.Set

func SetTargetResourcesSet(targetResources ArrayFlags) {
	targetResourcesSet = mapset.NewSet()

	for _, resource := range targetResources {
		targetResourcesSet.Add(resource)
	}
}

func GetTargetResourcesSet() *mapset.Set {
	return &targetResourcesSet
}
