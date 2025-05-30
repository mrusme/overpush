package application

type Application struct {
	Enable       bool
	Token        string
	Name         string
	IconPath     string
	Format       string
	CustomFormat CFormat

	Target string
}
