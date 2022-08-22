package resource

func IsService(name string) bool {
	switch name {
	case "svc", "service", "services":
		return true
	default:
		return false
	}
}
