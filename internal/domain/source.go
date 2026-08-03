package domain

type SourceRegistration struct {
	Key           string
	Company       string
	PriorityGroup string
	Type          string
	URL           string
	Adapter       string
	Enabled       bool
}
