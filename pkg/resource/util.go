package resource

import (
	"regexp"
	"slices"

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

func IsContainerSelected(container v1.Container, filter *regexp.Regexp) bool {
	if filter == nil {
		return true
	}

	return filter.MatchString(container.Name)
}

func SelectedContainers(spec v1.PodSpec, filter *regexp.Regexp) []v1.Container {
	containers := slices.Concat(spec.InitContainers, spec.Containers)

	return slices.DeleteFunc(containers, func(container v1.Container) bool {
		return !IsContainerSelected(container, filter)
	})
}
