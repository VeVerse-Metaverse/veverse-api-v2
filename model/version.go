package model

type ReleaseVersion struct {
	CodeVersion    string `json:"codeVersion,omitempty"`
	ContentVersion string `json:"contentVersion,omitempty"`
}
