package viewBackend

import "html/template"

type FrontEndParams struct {
	Title              string
	Name               string
	Message            string
	Message2           string
	Message3           string
	Path               string
	UserData           interface{}
	SessionData        map[string]string
	CurrentURL         string
	Page               int
	CSRFToken          template.HTML
	LoggedIn           bool
	IsViewingPublic    bool
	UserOwnsRepository bool
	Error              map[string]string
	RepoList           []string
	RepoMatches        []map[string]string
	RepoDescription    string
	RepoOwners         string
	RepoPublic         bool
}
