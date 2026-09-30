package application

type Application struct {
	Enable       bool
	Confirmed    bool
	Token        string
	Name         string
	IconPath     string
	Format       string
	CustomFormat CFormat

	EncryptionType       string // "none", "age"
	EncryptionRecipients []string
	EncryptTitle         bool
	EncryptMessage       bool
	EncryptAttachment    bool

	Target     string
	TargetArgs map[string]interface{}
}
