package register

import (
	"fmt"
	"inkdrop/config"
	userModel "inkdrop/model"
	viewBackend "inkdrop/view/connector"
	"net/http"
	"strings"
)

func RegisterMain(w http.ResponseWriter, r *http.Request) {
	p := viewBackend.FrontEndParams{
		Title:   "Register",
		Message: "Register for a new FtR account",
		Error:   make(map[string]string),
	}

	SS := config.GetSessionManager()
	name := SS.GetString(r.Context(), "name")
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	if name != "" && isLoggedIn == true {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	viewBackend.RegisterMain(w, p)
}

func RegisterMainPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse form entry: %s", err), http.StatusBadRequest)
		return
	}

	userNameRaw := strings.TrimSpace(r.FormValue("name"))
	userEmail := strings.TrimSpace(r.FormValue("email"))
	userPassword := strings.TrimSpace(r.FormValue("password"))

	if userEmail == "" || userPassword == "" || userNameRaw == "" {
		paramData := viewBackend.FrontEndParams{
			Title:   "Register",
			Message: "Register for a new FtR account",
			Error:   make(map[string]string),
		}

		paramData.Error["general"] = "Username, email, and password are required."

		viewBackend.RegisterMain(w, paramData)
		return
	}

	userNameCooked := strings.ReplaceAll(userNameRaw, " ", "_")

	fmt.Println("User name:", userNameCooked)
	fmt.Println("User email:", userEmail)
	fmt.Println("User password:", userPassword)

	db := config.GetDB()

	err = userModel.CreateUser(db, userNameCooked, userEmail, userPassword)
	if err != nil {
		fmt.Println("Error:", err)
		paramData := viewBackend.FrontEndParams{
			Title:   "Register",
			Message: "Register for a new FtR account",
			Error:   make(map[string]string),
		}

		paramData.Error["general"] = fmt.Sprintf("Error registering: %s", err)

		viewBackend.RegisterMain(w, paramData)
		return
	}

	paramData := viewBackend.FrontEndParams{
		Title:    "Register",
		Message:  "Successfully registered for a new account. Please proceed to login.",
		Message2: fmt.Sprintf("<br>Your email is: <strong style='color: #0f0'>%s</strong><br>Your username is: <strong style='color: #0f0'>%s</strong><br>Please remember your credentials when you log in.", userEmail, userNameCooked),
		Message3: "<br><br><a href='/login'><button class='redirect'>Login</button></a>",
	}
	viewBackend.RenderSuccessfulRegister(w, paramData)
}
