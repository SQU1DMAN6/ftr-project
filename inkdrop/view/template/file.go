package template

var (
	// Frontend
	LoginMain                 = Parse("themes/login/login.html")
	RegisterMain              = Parse("themes/register/register.html")
	RenderSuccessfulRegister  = ParseBackEndMessage("themes/register/successregister.html")
	IndexMain                 = Parse("themes/index/index.html")
	IndexMainBrowseRepository = Parse("themes/index/browse.html")
)

func init() {
	// print the name of each parsed template so we can verify root naming
	println("[template:init] LoginMain name=", LoginMain.Name())
	println("[template:init] RegisterMain name=", RegisterMain.Name())
}
