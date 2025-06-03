package target

type Target struct {
	Enable bool
	ID     string
	Type   string
	Args   map[string]interface{}
}
