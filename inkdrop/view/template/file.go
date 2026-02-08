package template

var (
	// Frontend
	LoginMain                = Parse("themes/login/login.html")
	RegisterMain             = Parse("themes/register/register.html")
	RenderSuccessfulRegister = ParseBackEndMessage("themes/register/successregister.html")
)
