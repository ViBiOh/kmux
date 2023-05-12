package resource

import (
	"regexp"

	v1 "k8s.io/api/core/v1"
)

func IsService(name string) bool {
	switch name {
	case "svc", "service", "services":
		return true
	default:
		return false
	}
}

func IsContainedSelected(container v1.Container, filter *regexp.Regexp) bool {
	if filter == nil {
		return true
	}

	return filter.MatchString(container.Name)
}
