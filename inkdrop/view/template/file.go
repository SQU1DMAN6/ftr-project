package template

var (
	// Frontend
	LoginMain                 = Parse("themes/login/login.html")
	RegisterMain              = Parse("themes/register/register.html")
	RenderSuccessfulRegister  = ParseBackEndMessage("themes/register/successregister.html")
	IndexMain                 = Parse("themes/index/index.html")
	IndexMainBrowseRepository = Parse("themes/index/browse.html")
)
